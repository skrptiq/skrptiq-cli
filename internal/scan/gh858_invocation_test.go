package scan

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/skrptiq/engine/storage"
)

// GH#858 — a prompt or skill the package ships that no workflow invokes.
//
// JUSTIFIED ON THE 19, NOT ON A1. A1's own defect (a step written `- prompt: "X"`
// with no `skill:`, so nothing could `from_step`-bind to it) is ALREADY refused by
// the scan: the pre-fix package exits 2 under the shipped build with
// `workflow.execution_context_missing_binding`. Its orphaned prompts were a
// symptom of that, not an independent gap.
//
// The gap this rule closes is the 19 orphaned nodes across 13 of the 88 published
// packages that pass the scan TODAY. If this rule and the binding checks caught
// the same thing, that number would be zero. It is not zero, so the populations
// differ — demonstrated on live content rather than argued.

func invocationIssues(t *testing.T, dir string) []ScanIssue {
	t.Helper()
	_, _, result := scanPkg(t, dir)
	var ours []ScanIssue
	for _, i := range result.Issues {
		if i.Code == CodeNodeNeverInvoked {
			ours = append(ours, i)
		}
	}
	return ours
}

// A package whose workflow names every node it ships, in a shape the walker has
// to recurse into.
func invokedPkg(execution string, extraFiles map[string]string) map[string]string {
	files := map[string]string{
		"workflows/wf.md": `---
type: workflow
id: wf
title: WF
description: "fixture"
connections:
  - target: alpha-skill
    type: uses
execution:
` + execution + `---

## Overview

Fixture.
`,
		"skills/alpha-skill.md": `---
type: skill
id: alpha-skill
title: Alpha Skill
description: "s"
---

Do alpha.
`,
		"prompts/alpha-prompt.md": `---
type: prompt
id: alpha-prompt
title: Alpha Prompt
description: "p"
---

Alpha.
`,
	}
	for k, v := range extraFiles {
		files[k] = v
	}
	return files
}

// ── Refusal ───────────────────────────────────────────────────────────────

// The headline: a shipped prompt no execution entry names is refused, naming
// the node, at a severity that fails the publish gate.
func TestGH858_ShippedButNeverInvoked_IsRefused(t *testing.T) {
	dir := writePkg(t, invokedPkg(`  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
`, map[string]string{
		"prompts/orphan-prompt.md": `---
type: prompt
id: orphan-prompt
title: Orphan Prompt
description: "documented as a stage, never invoked"
---

Produce the thing the docs promise.
`,
	}))

	issues := invocationIssues(t, dir)
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 %s, got %d: %+v", CodeNodeNeverInvoked, len(issues), issues)
	}
	issue := issues[0]
	if issue.Severity != storage.SeverityError {
		t.Errorf("severity = %q, want error — this must fail the publish gate", issue.Severity)
	}
	if issue.ReferencedSlug != "orphan-prompt" {
		t.Errorf("referencedSlug = %q, want orphan-prompt", issue.ReferencedSlug)
	}
	if !strings.Contains(issue.Message, `"orphan-prompt"`) {
		t.Errorf("message does not name the node: %q", issue.Message)
	}
	// Both honest remedies, because which one is right is the author's call.
	for _, want := range []string{"add an execution entry", "remove the node"} {
		if !strings.Contains(issue.Message, want) {
			t.Errorf("message does not offer %q: %q", want, issue.Message)
		}
	}
}

// ── Acceptance ────────────────────────────────────────────────────────────

// A package whose every prompt and skill is invoked is accepted, AND the
// execution wiring survives the scan verbatim. Asserted by reading the step's
// own entry back out of the scanned package rather than from the absence of an
// issue — on GH#975 an absence-only assertion would have been satisfied by the
// scan silently losing the declaration it was supposed to preserve.
func TestGH858_AllNodesInvoked_IsAcceptedAndWiringSurvives(t *testing.T) {
	dir := writePkg(t, invokedPkg(`  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
    bindings:
      subject:
        from_step: "Alpha Skill"
        field: output
`, nil))

	issues, _, result := scanPkg(t, dir)
	for _, i := range issues {
		if i.Code == CodeNodeNeverInvoked {
			t.Errorf("a fully-invoked package was refused: %+v", i)
		}
	}

	var sawBinding bool
	for _, n := range result.Package.Nodes {
		if n.Type != "workflow" {
			continue
		}
		execList, ok := n.Metadata["execution"].([]any)
		if !ok {
			t.Fatal("the scan did not preserve the execution block at all")
		}
		for _, e := range execList {
			m, _ := e.(map[string]any)
			b, ok := m["bindings"].(map[string]any)
			if !ok {
				continue
			}
			subj, ok := b["subject"].(map[string]any)
			if !ok {
				continue
			}
			sawBinding = true
			if subj["from_step"] != "Alpha Skill" {
				t.Errorf("from_step survived as %v, want %q", subj["from_step"], "Alpha Skill")
			}
			if subj["field"] != "output" {
				t.Errorf("field survived as %v, want %q", subj["field"], "output")
			}
		}
	}
	if !sawBinding {
		t.Error("the binding did not survive the scan — nothing to assert on")
	}
}

// ── The regression this rule's own census caught ──────────────────────────

// A step inside an `execution:` `- parallel:` group is invoked. The first draft
// read only each entry's top-level `skill:`/`prompt:` and reported every node in
// a parallel group as never invoked.
func TestGH858_StepsInsideAParallelGroup_AreInvoked(t *testing.T) {
	dir := writePkg(t, invokedPkg(`  - parallel:
    - skill: "alpha-skill"
      step_type: "generation"
      prompt: "alpha-prompt"
    - skill: "beta-skill"
      step_type: "generation"
      prompt: "beta-prompt"
`, map[string]string{
		"skills/beta-skill.md": `---
type: skill
id: beta-skill
title: Beta Skill
description: "s"
---

Do beta.
`,
		"prompts/beta-prompt.md": `---
type: prompt
id: beta-prompt
title: Beta Prompt
description: "p"
---

Beta.
`,
	}))

	if issues := invocationIssues(t, dir); len(issues) != 0 {
		t.Errorf("steps nested in a parallel group were reported as never invoked: %+v", issues)
	}
}

// A node invoked only from a loop body is invoked. No published package relies
// on this today — every loop-body step is also an execution entry — so this is
// the assertion that keeps it true rather than accidental.
func TestGH858_NodeInvokedOnlyFromALoopBody_IsInvoked(t *testing.T) {
	files := invokedPkg(`  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
`, map[string]string{
		"skills/looped-skill.md": `---
type: skill
id: looped-skill
title: Looped Skill
description: "s"
---

Do it per item.
`,
	})
	files["workflows/wf.md"] = strings.Replace(files["workflows/wf.md"],
		"execution:\n", `loops:
  - id: "per-item"
    mode: "for_each"
    inputExpression: "{{input.items}}"
    steps:
      - "looped-skill"
    maxIterations: 20
execution:
`, 1)

	dir := writePkg(t, files)
	if issues := invocationIssues(t, dir); len(issues) != 0 {
		t.Errorf("a loop-body step was reported as never invoked: %+v", issues)
	}
}

// ── Scoping ───────────────────────────────────────────────────────────────

// Only prompt and skill nodes exist in order to be executed. Sources,
// documents and assets are reference material and have no execution entry by
// design; reporting them would refuse most of the catalogue.
func TestGH858_NonExecutableNodeTypes_AreNeverReported(t *testing.T) {
	dir := writePkg(t, invokedPkg(`  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
`, map[string]string{
		"sources/framework.md": `---
type: source
id: framework
title: Framework
description: "reference"
---

Reference material.
`,
		"documents/playbook.md": `---
type: document
id: playbook
title: Playbook
description: "reference"
---

A playbook.
`,
		"assets/template.md": `---
type: asset
id: template
title: Template
description: "reference"
---

A template.
`,
	}))

	if issues := invocationIssues(t, dir); len(issues) != 0 {
		t.Errorf("non-executable node types were reported: %+v", issues)
	}
}

// A package with NO workflow of its own ships its nodes to be IMPORTED, so
// "uninvoked" is its normal state. This is the answer to `_shared/`: all 135
// shared packages in the published catalogue carry a manifest and zero
// workflows, so the rule scopes them out structurally, with no knowledge of the
// `_shared/` path convention.
func TestGH858_PackageWithNoWorkflow_IsNeverRefused(t *testing.T) {
	dir := writePkg(t, map[string]string{
		"prompts/library-prompt.md": `---
type: prompt
id: library-prompt
title: Library Prompt
description: "shipped for consumers to invoke"
---

A shared prompt.
`,
	})

	if issues := invocationIssues(t, dir); len(issues) != 0 {
		t.Errorf("a workflow-less library package was refused: %+v", issues)
	}
}

// Invocation is unioned across every workflow in the package: a node invoked by
// one workflow is invoked, even if the others never name it.
func TestGH858_MultiWorkflowPackage_UnionsInvocations(t *testing.T) {
	files := invokedPkg(`  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
`, map[string]string{
		"prompts/second-prompt.md": `---
type: prompt
id: second-prompt
title: Second Prompt
description: "invoked by the second workflow only"
---

Second.
`,
		"workflows/wf-two.md": `---
type: workflow
id: wf-two
title: WF Two
description: "the other workflow"
connections:
  - target: alpha-skill
    type: uses
execution:
  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "second-prompt"
---

## Overview

Second workflow.
`,
	})

	dir := writePkg(t, files)
	if issues := invocationIssues(t, dir); len(issues) != 0 {
		t.Errorf("a node invoked by a sibling workflow was reported as never invoked: %+v", issues)
	}
}

// ── Corpus regressions, on verbatim copies of published content ───────────

// The working package the first draft falsely refused. See
// testdata/gh858-corpus/README.md.
func TestGH858_Corpus_ParallelGroupPackageScansClean(t *testing.T) {
	issues := invocationIssues(t, filepath.Join("testdata", "gh858-corpus", "morning-briefing"))
	if len(issues) != 0 {
		t.Errorf("the published morning-briefing package was refused: %+v", issues)
	}
}

// A genuine orphan in published content, frozen as a specimen: the workflow body
// says to invoke `stakeholder-update-email` and the execution block never does.
// This is the live evidence the rule's population is non-empty and distinct from
// what the binding checks already catch.
func TestGH858_Corpus_GenuineOrphanInPublishedContentIsRefused(t *testing.T) {
	issues := invocationIssues(t, filepath.Join("testdata", "gh858-corpus", "status-report-flow"))
	var found bool
	for _, i := range issues {
		if i.ReferencedSlug == "stakeholder-update-email" {
			found = true
			if i.Severity != storage.SeverityError {
				t.Errorf("severity = %q, want error", i.Severity)
			}
		}
	}
	if !found {
		t.Errorf("stakeholder-update-email was not refused; issues = %+v", issues)
	}
}
