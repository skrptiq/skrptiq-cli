package scan

import (
	"strings"
	"testing"

	"github.com/skrptiq/engine/storage"
)

// GH#1011 — a DIRECTIONAL REGRESSION GUARD, not a gate.
//
// The gate already exists and it is the engine's: `validate.go:1321` refuses an
// unhandled `local.*` at SeverityError, `execution/step.go:236` refuses it again
// at dispatch, and both read the one list through
// `storage.IsHandledLocalStepType`. The scan inherits it by running
// ValidateWorkflowWithDeps. A second clause here would be a parallel authority
// whose only possible contribution is disagreement (the GH#1058 lesson).
//
// So these tests assert the INHERITANCE, not the rule: that an unhandled
// `local.*` still reaches a scan caller as an error, and that every type the
// engine does handle is still accepted. They fail in CLI's suite if a future
// engine change reopens the GH#519 prefix hatch — the change nobody would
// notice from this repo.
//
// EVERY TYPE LIST HERE COMES FROM `storage.HandledLocalStepTypes()`. A literal
// list would be the very parallel authority the guard exists to prevent, and it
// would go stale on the day a handler is added — failing in the direction that
// matters, refusing content the engine runs fine.
//
// Census at the time of writing (232 bundles): nine declared step types,
// `local.transform` the only unhandled one, 7 live instances in 3 packages, all
// already refused at error severity.

// stepTypePkg is a one-step workflow whose step_type the caller chooses.
func stepTypePkg(stepType string) map[string]string {
	return map[string]string{
		"workflows/wf.md": `---
type: workflow
id: wf
title: WF
description: "fixture"
connections:
  - target: only-skill
    type: uses
execution:
  - skill: "only-skill"
    step_type: "` + stepType + `"
    prompt: "only-prompt"
---

## Overview

Fixture.
`,
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
---

Do the thing.
`,
	}
}

// stepTypeIssues returns the engine's two step-type findings for a package.
func stepTypeIssues(t *testing.T, dir string) []ScanIssue {
	t.Helper()
	_, _, result := scanPkg(t, dir)
	var ours []ScanIssue
	for _, i := range result.Issues {
		if strings.Contains(i.Code, "step_type") {
			ours = append(ours, i)
		}
	}
	return ours
}

// FLOOR: a step type the engine has no handler for must never publish. It is
// not merely unsupported — before GH#1010 it was dispatched AS AN LLM STEP,
// established by running it (4.06s of real model call against local.template's
// 0.01s on the same fixture), so a step declared deterministic silently became
// a prompt-driven model call.
func TestGH1011_UnhandledLocalStepType_IsRefusedAndGates(t *testing.T) {
	dir := writePkg(t, stepTypePkg("local.transform"))

	issues := stepTypeIssues(t, dir)
	var found *ScanIssue
	for i := range issues {
		if issues[i].Code == "workflow.unhandled_local_step_type" {
			found = &issues[i]
		}
	}
	if found == nil {
		t.Fatalf("an unhandled local.* step type was not refused — the GH#519 prefix hatch is open again; issues = %+v", issues)
	}
	if found.Severity != storage.SeverityError {
		t.Errorf("severity = %q, want error", found.Severity)
	}
	// The message must name the ALLOWLIST, not the prefix: an author needs the
	// set they must join, not the pattern they happened to match.
	for _, handled := range storage.HandledLocalStepTypes() {
		if !strings.Contains(found.Message, handled) {
			t.Errorf("refusal does not name handled type %q, so it does not tell the author the set to join: %q", handled, found.Message)
		}
	}

	if _, code, _ := scanPkg(t, dir); code != 2 {
		t.Errorf("scan exit = %d, want 2 — an unrunnable step type must gate", code)
	}
}

// FLOOR, AND THE ONE THAT MATTERS MOST: the gate must never refuse a type the
// engine can execute. That is the GH#1058 failure direction — a scanner
// refusing content the engine runs fine — and it is why this table is built
// from the accessor rather than written out.
func TestGH1011_EveryHandledLocalStepType_IsAccepted(t *testing.T) {
	handled := storage.HandledLocalStepTypes()
	if len(handled) == 0 {
		t.Fatal("storage.HandledLocalStepTypes() is empty — the authority this guard reads has gone away")
	}
	for _, st := range handled {
		t.Run(st, func(t *testing.T) {
			dir := writePkg(t, stepTypePkg(st))
			for _, i := range stepTypeIssues(t, dir) {
				t.Errorf("handled step type %q was reported: %s — %s", st, i.Code, i.Message)
			}
		})
	}
}

// FLOOR: the engine does not own every vocabulary. A `step_type` outside its
// lists that is NOT in the `local.` namespace is not provably unrunnable, so it
// warns rather than gates — the missing-vs-invalid split, kept.
func TestGH1011_UnknownNonLocalStepType_WarnsAndDoesNotGate(t *testing.T) {
	dir := writePkg(t, stepTypePkg("transform"))

	issues := stepTypeIssues(t, dir)
	var found *ScanIssue
	for i := range issues {
		if issues[i].Code == "workflow.unknown_step_type" {
			found = &issues[i]
		}
	}
	if found == nil {
		t.Fatalf("a non-canonical step type produced no finding at all; issues = %+v", issues)
	}
	if found.Severity != storage.SeverityWarning {
		t.Errorf("severity = %q, want warning — a type outside the engine's vocabulary is not provably unrunnable", found.Severity)
	}
	if _, code, _ := scanPkg(t, dir); code == 2 {
		t.Error("scan exit = 2 — an unknown non-local step type must not gate")
	}
}

// The differential that makes the first test non-vacuous from inside this repo:
// the same fixture shape, one handled type and one unhandled, must not produce
// the same verdict. If both come back clean the hatch is open; if both are
// refused the guard is refusing working content.
func TestGH1011_HandledAndUnhandled_AreNotTreatedAlike(t *testing.T) {
	handledDir := writePkg(t, stepTypePkg("local.template"))
	unhandledDir := writePkg(t, stepTypePkg("local.transform"))

	if n := len(stepTypeIssues(t, handledDir)); n != 0 {
		t.Errorf("local.template (handled) produced %d step-type findings, want 0", n)
	}
	if n := len(stepTypeIssues(t, unhandledDir)); n == 0 {
		t.Error("local.transform (unhandled) produced no step-type finding, want at least 1")
	}
}
