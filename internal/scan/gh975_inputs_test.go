package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skrptiq/engine/storage"
)

// GH#975 — the input-contract rules. Two assertions per rule, with the refusal
// one FIRST and the acceptance one second, because the acceptance assertion is
// the one that finds things: it caught a false positive in the first draft of
// this rule (see fencedBlock in inputs.go) before it shipped.

// writePkg writes a package to a temp dir. Each file is the literal frontmatter
// an author would write, so the fixtures exercise the real parser.
func writePkg(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	files["skrptiq.yaml"] = `name: fixture
slug: fixture
display_name: Fixture
description: "GH#975 input-contract fixture"
author: test
category: productivity
version: 1.0.0
`
	for rel, body := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return dir
}

// scanPkg runs the real scan and returns its GH#975 issues plus the exit code.
func scanPkg(t *testing.T, dir string) ([]ScanIssue, int, ScanResult) {
	t.Helper()
	var buf strings.Builder
	code := RunWithOptions(dir, true, Options{NoResolveDeps: true}, &buf)

	var result ScanResult
	if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
		t.Fatalf("scan --json output did not parse: %v\n%s", err, buf.String())
	}
	var ours []ScanIssue
	for _, i := range result.Issues {
		if strings.HasPrefix(i.Code, "workflow.input_") {
			ours = append(ours, i)
		}
	}
	return ours, code, result
}

func findIssue(issues []ScanIssue, code, slug string) *ScanIssue {
	for i := range issues {
		if issues[i].Code == code && issues[i].ReferencedSlug == slug {
			return &issues[i]
		}
	}
	return nil
}

// A one-step workflow whose prompt references `name`, with the prompt's
// frontmatter and the workflow's Inputs table supplied by the caller.
func onePromptPkg(promptFrontmatter, wfInputsTable, ref string) map[string]string {
	return map[string]string{
		"workflows/fixture-wf.md": `---
type: workflow
id: fixture-wf
title: Fixture WF
description: "one step"
connections:
  - target: only-skill
    type: uses
execution:
  - skill: "only-skill"
    step_type: "generation"
    prompt: "only-prompt"
---

## Overview

Fixture.

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
` + wfInputsTable,
		"skills/only-skill.md": `---
type: skill
id: only-skill
title: Only Skill
description: "s"
---

Do the thing.
`,
		"prompts/only-prompt.md": `---
type: prompt
id: only-prompt
title: Only Prompt
description: "p"
` + promptFrontmatter + `---

## Purpose

` + ref + `
`,
	}
}

// ── Rule 1 ────────────────────────────────────────────────────────────────

// REFUSAL: documented in the Inputs table, declared nowhere. This is the shape
// the four genuine corpus violations have (ioc-defanger's {{input.text}},
// csv-to-narrative-report's {{input.csv}}, big-log-extractor-summariser's
// {{input.log}}, word-count-gated-writer's {{input.draft}}).
func TestGH975_Scan_UndeclaredDocumentedInput_IsRefused(t *testing.T) {
	dir := writePkg(t, onePromptPkg(
		"", // no inputs: block at all
		"| `{{input.log}}` | Yes | The raw log text | `ERROR db timeout` |\n",
		"Summarise this log: {{input.log}}",
	))

	issues, code, _ := scanPkg(t, dir)
	issue := findIssue(issues, CodeInputUndeclared, "log")
	if issue == nil {
		t.Fatalf("no %s for log; got %+v", CodeInputUndeclared, issues)
	}
	if issue.Severity != storage.SeverityError {
		t.Errorf("severity = %q, want error — a documented-but-undeclared input must gate", issue.Severity)
	}
	if !strings.Contains(issue.Message, `"log"`) {
		t.Errorf("message does not name the input: %q", issue.Message)
	}
	if !strings.Contains(issue.Message, "only-prompt") {
		t.Errorf("message does not name the referencing node: %q", issue.Message)
	}
	if code == 0 {
		t.Error("scan exit code 0 — an error-severity issue must fail the gate")
	}
}

// ACCEPTANCE: declared optional, and THE DECLARATION SURVIVES THE SCAN VERBATIM.
// Asserted by reading `required` back out of the scanned package, not by the
// absence of an issue — an absence would also be satisfied by the scan losing
// the declaration entirely.
func TestGH975_Scan_DeclaredOptionalInput_IsAcceptedAndSurvives(t *testing.T) {
	dir := writePkg(t, onePromptPkg(
		`inputs:
  week_start:
    label: "Week Start Date"
    description: "Start of the reporting week"
    required: false
    type: text
`,
		"| `{{input.week_start}}` | No | Reporting week start | `2026-04-20` |\n",
		"Range: if `{{input.week_start}}` is provided, use it as Monday.",
	))

	issues, code, result := scanPkg(t, dir)
	if len(issues) != 0 {
		t.Errorf("a correctly-declared optional input produced issues: %+v", issues)
	}
	if code != 0 {
		t.Errorf("scan exit = %d, want 0", code)
	}

	var found bool
	for _, n := range result.Package.Nodes {
		raw, ok := n.Metadata["inputs"].(map[string]any)
		if !ok {
			continue
		}
		decl, ok := raw["week_start"].(map[string]any)
		if !ok {
			continue
		}
		found = true
		req, present := decl["required"]
		if !present {
			t.Error("the scan dropped the `required` key from the declaration")
		}
		if req != false {
			t.Errorf("required survived as %v (%T), want false", req, req)
		}
		if decl["label"] != "Week Start Date" {
			t.Errorf("label survived as %v, want %q", decl["label"], "Week Start Date")
		}
	}
	if !found {
		t.Error("no node in the scanned package carries the inputs declaration at all")
	}
}

// An undeclared input that is documented NOWHERE is a warning, not a gate: it
// may be a literal example in prose about authoring, and a gate must not refuse
// the content that documents the gate.
func TestGH975_Scan_UndeclaredUndocumentedInput_WarnsNotGates(t *testing.T) {
	dir := writePkg(t, onePromptPkg(
		"",
		"| `{{input.something_else}}` | Yes | unrelated | `x` |\n",
		"Every `{{input.VAR}}` in a prompt body needs a matching inputs entry.",
	))

	issues, code, _ := scanPkg(t, dir)
	issue := findIssue(issues, CodeInputUndeclared, "VAR")
	if issue == nil {
		t.Fatalf("no %s for VAR; got %+v", CodeInputUndeclared, issues)
	}
	if issue.Severity != storage.SeverityWarning {
		t.Errorf("severity = %q, want warning — an undocumented name may be an example", issue.Severity)
	}
	// 0 pass / 1 warnings / 2 errors: the publish gate is 2, so a warning may
	// move the exit code off 0 without gating.
	if code == 2 {
		t.Errorf("scan exit = 2 — a warning must not gate")
	}
}

// `{{input.*}}` is not a legal identifier and must not be reported at all.
func TestGH975_Scan_IllegalInputName_IsNotReported(t *testing.T) {
	dir := writePkg(t, onePromptPkg("", "", "Inventory every `{{input.*}}` reference."))

	issues, _, _ := scanPkg(t, dir)
	if i := findIssue(issues, CodeInputUndeclared, "*"); i != nil {
		t.Errorf("reported `{{input.*}}` as an input: %+v", i)
	}
}

// ── Rule 2 ────────────────────────────────────────────────────────────────
//
// SYNTHETIC FIXTURES BY NECESSITY: no live instance exists corpus-wide as of
// 2026-10-12 (198 of 198 node files with an `inputs:` block use an explicit
// `required:`). See CodeInputOptionalityUndeclared for why the rule ships anyway.

func TestGH975_Scan_OptionalityOnlyInProse_IsRefused(t *testing.T) {
	dir := writePkg(t, onePromptPkg(
		`inputs:
  week_start:
    label: "Week Start Date"
    description: "Start of the reporting week. Defaults to last Monday."
    type: text
`,
		"| `{{input.week_start}}` | No | Reporting week start | `2026-04-20` |\n",
		"Range: `{{input.week_start}}`",
	))

	issues, code, _ := scanPkg(t, dir)
	issue := findIssue(issues, CodeInputOptionalityUndeclared, "week_start")
	if issue == nil {
		t.Fatalf("no %s for week_start; got %+v", CodeInputOptionalityUndeclared, issues)
	}
	if issue.Severity != storage.SeverityError {
		t.Errorf("severity = %q, want error", issue.Severity)
	}
	if !strings.Contains(issue.Message, `"week_start"`) {
		t.Errorf("message does not name the input: %q", issue.Message)
	}
	if code == 0 {
		t.Error("scan exit code 0 — this must gate")
	}
}

func TestGH975_Scan_OptionalityDeclared_IsAccepted(t *testing.T) {
	dir := writePkg(t, onePromptPkg(
		`inputs:
  week_start:
    label: "Week Start Date"
    description: "Start of the reporting week. Defaults to last Monday."
    required: false
    type: text
`,
		"| `{{input.week_start}}` | No | Reporting week start | `2026-04-20` |\n",
		"Range: `{{input.week_start}}`",
	))

	issues, code, _ := scanPkg(t, dir)
	if i := findIssue(issues, CodeInputOptionalityUndeclared, "week_start"); i != nil {
		t.Errorf("a declared `required: false` still tripped Rule 2: %+v", i)
	}
	if code != 0 {
		t.Errorf("scan exit = %d, want 0; issues = %+v", code, issues)
	}
}

// ── Rule 3 ────────────────────────────────────────────────────────────────

func TestGH975_Scan_DeclaredUnusedInput_WarnsAndDoesNotGate(t *testing.T) {
	dir := writePkg(t, onePromptPkg(
		`inputs:
  never_used:
    label: "Never Used"
    required: false
    type: text
`,
		"",
		"This prompt references no input at all.",
	))

	issues, code, _ := scanPkg(t, dir)
	issue := findIssue(issues, CodeInputDeclaredUnused, "never_used")
	if issue == nil {
		t.Fatalf("no %s for never_used; got %+v", CodeInputDeclaredUnused, issues)
	}
	if issue.Severity != storage.SeverityWarning {
		t.Errorf("severity = %q, want warning", issue.Severity)
	}
	if code == 2 {
		t.Errorf("scan exit = 2 — an unused declaration must not gate")
	}
}

// ACCEPTANCE: an input consumed only through the execution block's binding is
// used. The reuse model (GH#782) maps a shared element's input slots from the
// consuming workflow, so the reference lives in the workflow's execution entry
// rather than in any prompt body.
func TestGH975_Scan_InputBoundInExecutionBlock_IsNotUnused(t *testing.T) {
	files := onePromptPkg(
		`inputs:
  payload:
    label: "Payload"
    required: true
    type: text
`,
		"| `{{input.payload}}` | Yes | the payload | `x` |\n",
		"Operate on the supplied payload.",
	)
	// The workflow binds the slot; the prompt body never names it.
	files["workflows/fixture-wf.md"] = strings.Replace(
		files["workflows/fixture-wf.md"],
		`    prompt: "only-prompt"`,
		`    prompt: "only-prompt"
    context:
      payload: "{{input.payload}}"`, 1)

	dir := writePkg(t, files)
	issues, code, _ := scanPkg(t, dir)
	if i := findIssue(issues, CodeInputDeclaredUnused, "payload"); i != nil {
		t.Errorf("an input bound in the execution block was reported unused: %+v", i)
	}
	if code != 0 {
		t.Errorf("scan exit = %d, want 0; issues = %+v", code, issues)
	}
}

// ── Dependency suppression ────────────────────────────────────────────────

// A step whose skill is local and whose prompt is dependency-provided (or the
// reverse) splits the pair across a boundary this scan cannot read:
// storage.DepNodeSummary carries Content but no Metadata, so a dep node's
// `inputs:` declaration is invisible even when resolution succeeds. Rule 1 must
// stay silent there rather than fail offline authoring.
func TestGH975_Scan_SplitAcrossDependency_SuppressesRule1(t *testing.T) {
	files := onePromptPkg("", "| `{{input.log}}` | Yes | the log | `x` |\n", "Summarise {{input.log}}")
	// Add a second step whose prompt is not defined in this package.
	files["workflows/fixture-wf.md"] = strings.Replace(
		files["workflows/fixture-wf.md"],
		`    prompt: "only-prompt"`,
		`    prompt: "only-prompt"
  - skill: "only-skill"
    step_type: "generation"
    prompt: "prompt-from-a-dependency"`, 1)

	dir := writePkg(t, files)
	issues, _, _ := scanPkg(t, dir)
	if i := findIssue(issues, CodeInputUndeclared, "log"); i != nil {
		t.Errorf("Rule 1 fired across an unreadable dependency boundary: %+v", i)
	}
}

// ── Corpus regressions — the most valuable assertions here ────────────────

// weekly-report is the package in GH#975's title and it is CORRECT: both inputs
// declare `required: false`. If a future rule change rejects it, the rule is
// wrong, not the content. This is the assertion that caught the first draft's
// inline-backtick escape discarding genuine references.
func TestGH975_Corpus_WeeklyReportScansClean(t *testing.T) {
	issues, code, _ := scanPkg(t, filepath.Join("testdata", "gh975-corpus", "weekly-report"))
	if len(issues) != 0 {
		t.Errorf("the published weekly-report package produced input issues: %+v", issues)
	}
	if code != 0 {
		t.Errorf("scan exit = %d, want 0", code)
	}
}

// The real prose that documents the rule being enforced must never be an error.
// `{{input.VAR}}` and `{{input.field_name}}` appear in published checklists such
// as "every `{{input.VAR}}` variable in prompt bodies has a matching `inputs`
// entry" — a gate that refuses those refuses its own documentation.
//
// Asserted on the INPUT-rule severities only, deliberately not on the exit code:
// this fixture is a REDUCTION of two published packages (three node files out of
// their full graphs), so its copied `connections:` point at nodes that are not
// here and the scan rightly reports those. Pinning the exit code would pin the
// reduction rather than the rule. The verbatim copies are the point; see
// testdata/gh975-corpus/README.md.
func TestGH975_Corpus_MetaAuthoringProseIsNotAnError(t *testing.T) {
	issues, _, _ := scanPkg(t, filepath.Join("testdata", "gh975-corpus", "meta-authoring"))
	if len(issues) == 0 {
		t.Fatal("no input issues at all — the fixture no longer exercises the escape")
	}
	for _, i := range issues {
		if i.Severity == storage.SeverityError {
			t.Errorf("teaching prose produced an ERROR: %s %s (%s)", i.Code, i.ReferencedSlug, i.Message)
		}
	}
}
