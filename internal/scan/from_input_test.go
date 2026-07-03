package scan

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// GH#755 / K-034 D1: the CLI must validate `from_input` execution bindings
// (added to the engine by GH#750) at the scan boundary — this is what the Hub
// publish gate runs via the checked-in `.skrptiq-cli`. These tests mirror the
// engine's from_input cases (validate_test.go) through `skrptiq scan`, so a
// future engine bump can never silently regress the path the gate depends on.
//
// The trap they lock: an input consumed ONLY via a binding never appears as
// {{input.X}}, so naive usage-detection would false-reject a clean consumer.
// Declared-input validation (the `inputs:` oracle) is what makes it pass.

// fromInputSkillMeta declares `topic` as an input on the consumer skill — the
// authoritative oracle collectDeclaredInputs unions over reachable nodes.
const fromInputSkill = `---
type: skill
id: consumer-skill
title: "Consumer Step"
description: "declares an input"
connections:
  - target: consumer-prompt
    type: derived_from
metadata:
  inputs:
    topic:
      description: "the topic"
---
Consumer skill.
`

// The prompt references the binding key, NOT {{input.topic}} — the K-034 trap.
const fromInputPrompt = `---
type: prompt
id: consumer-prompt
title: "Consumer Prompt"
description: "uses the binding, not an input expression"
---
Write about {{step.context.subject}}.
`

// writeFromInputPackage builds a self-contained skrpt package in a temp dir
// whose single workflow binds `subject` via the supplied source lines (indented
// to sit under `bindings.subject:`), mirroring the engine's seedFromInputFixture
// parametrisation. Returns the package root.
func writeFromInputPackage(t *testing.T, subjectSource string) string {
	t.Helper()
	root := t.TempDir()
	mustWrite := func(rel, body string) {
		t.Helper()
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("skrptiq.yaml", "name: from-input-fixture\nversion: \"1.0.0\"\ndescription: from_input boundary test\nauthor: test\n")
	mustWrite("skills/consumer-skill.md", fromInputSkill)
	mustWrite("prompts/consumer-prompt.md", fromInputPrompt)
	mustWrite("workflows/wf.md", `---
type: workflow
id: wf
title: "WF"
description: "from_input binding under test"
connections:
  - target: consumer-skill
    type: uses
    position: 0
metadata:
  execution:
    - skill: consumer-skill
      prompt: consumer-prompt
      step_type: generation
      bindings:
        subject:
`+subjectSource+`
  stepPrompts:
    consumer-skill: consumer-prompt
---
Workflow body.
`)
	return root
}

// scanDir runs a strict scan over an absolute path and returns the parsed result.
func scanDir(t *testing.T, dir string) ScanResult {
	t.Helper()
	var buf bytes.Buffer
	RunTo(dir, true, &buf)
	var result ScanResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON output not parseable: %v\nOutput: %s", err, buf.String())
	}
	return result
}

// A from_input binding against a DECLARED input validates clean — no binding
// codes, and (crucially) the publish gate accepts it. Only the local-only
// manifest_id_missing info is expected.
func TestScan_FromInput_Clean(t *testing.T) {
	result := scanDir(t, writeFromInputPackage(t, "          from_input: topic"))

	if result.ErrorCount != 0 || result.WarnCount != 0 {
		t.Errorf("clean from_input package: expected 0 errors + 0 warnings, got %d + %d",
			result.ErrorCount, result.WarnCount)
		for _, i := range result.Issues {
			t.Logf("  %s: %s", i.Code, i.Message)
		}
	}
	for _, code := range []string{
		"workflow.execution_bindings_source_conflict",
		"workflow.execution_bindings_missing_source",
		"workflow.execution_bindings_unknown_input",
		"workflow.execution_bindings_field_not_permitted",
	} {
		if hasIssueCode(result, code) {
			t.Errorf("clean from_input binding should not trip %s", code)
		}
	}

	var buf bytes.Buffer
	if code := RunTo(writeFromInputPackage(t, "          from_input: topic"), true, &buf); code != 0 {
		t.Errorf("clean from_input package: expected exit 0, got %d", code)
	}
}

// The four #750 reject modes, each locked to its own code (the missing-vs-invalid
// split — never one lumped "bad binding").
func TestScan_FromInput_Rejects(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:    "both_sources_set",
			source:  "          from_step: Whatever\n          from_input: topic",
			wantErr: "workflow.execution_bindings_source_conflict",
		},
		{
			name:    "neither_source_set",
			source:  "          {}",
			wantErr: "workflow.execution_bindings_missing_source",
		},
		{
			name:    "unknown_input",
			source:  "          from_input: titel", // typo → not a declared input
			wantErr: "workflow.execution_bindings_unknown_input",
		},
		{
			name:    "field_with_from_input",
			source:  "          from_input: topic\n          field: output",
			wantErr: "workflow.execution_bindings_field_not_permitted",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeFromInputPackage(t, tc.source)
			result := scanDir(t, dir)
			if !hasIssueCode(result, tc.wantErr) {
				t.Errorf("expected %s, got issues:", tc.wantErr)
				for _, i := range result.Issues {
					t.Logf("  %s: %s", i.Code, i.Message)
				}
			}
			// A binding error must fail the scan (exit 2) so the gate rejects it.
			var buf bytes.Buffer
			if code := RunTo(dir, true, &buf); code != 2 {
				t.Errorf("expected exit 2 (error) for %s, got %d", tc.name, code)
			}
		})
	}
}
