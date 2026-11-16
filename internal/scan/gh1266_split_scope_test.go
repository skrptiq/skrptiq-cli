package scan

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/skrptiq/engine/storage"
)

// GH#1266 — Rule 1 was switched off PACKAGE-WIDE by one straddling execution
// entry, so its error rung could not fire for 42 of 88 workflow-bearing
// catalogue packages (48%).
//
// HOW IT WAS FOUND, because the method is the transferable part: it surfaced
// while running a POSITIVE CONTROL that was checking something else. hub had
// asked why a documented `{{input.VAR}}` did not promote to error; the control
// for that question was "delete a real declaration from a real package and
// confirm the ladder fires". It returned exit 0 and total silence — not a
// missed promotion, a rule that never ran. Reading the code shows a `continue`
// with a defensible rationale; only the control showed the blast radius.
//
// The suppression is CORRECT and still exists. A dependency's node content is
// served with its frontmatter stripped — measured on the wire from the
// resolver's own cache: 203 dep nodes, 87 carrying content, 0 carrying an
// `inputs:` block. So a declaration on the dep half is genuinely unreadable and
// reporting its reference would be a false refusal. What was wrong was that one
// straddling entry excused every OTHER node in the package too.

// ACCEPTANCE 1 — the proof case, on the real package it was found in.
func TestGH1266_SplitEntryDoesNotExcuseAWhollyLocalNode(t *testing.T) {
	dir := filepath.Join("testdata", "gh1266-corpus", "weekly-report-split")
	issues, code, _ := scanPkg(t, dir)

	i := findIssue(issues, CodeInputUndeclared, "week_start")
	if i == nil {
		t.Fatalf("`week_start` is documented in the workflow's Inputs table, referenced in "+
			"prompts/fetch-week-data.md and declared nowhere — Rule 1 must report it. "+
			"Before GH#1266 this package scanned clean because ONE other entry straddled "+
			"a dep edge. got: %+v", issues)
	}
	// The ladder's top rung: documented in the table, so it gates.
	if i.Severity != storage.SeverityError {
		t.Errorf("severity = %q, want error — the workflow's Inputs table documents it", i.Severity)
	}
	if code == 0 {
		t.Error("scan exit 0 — the acceptance for this issue is that this package is no longer silent")
	}
}

// ACCEPTANCE 4 — the residue, which must survive. The node that IS paired with
// the unreadable half stays excused. A fix that drove this to zero would have
// deleted the false-refusal guard rather than narrowing it.
func TestGH1266_NodePairedWithTheUnreadableHalfIsStillExcused(t *testing.T) {
	dir := filepath.Join("testdata", "gh1266-corpus", "weekly-report-split")
	_, _, result := scanPkg(t, dir)

	if len(result.Rule1SuppressedNodes) == 0 {
		t.Fatalf("no node reported as suppressed, but this package has a straddling entry "+
			"(skill `synthesise-weekly-report` is a dependency) — the guard must still "+
			"apply to its local half. got: %+v", result.Rule1SuppressedNodes)
	}
	var found bool
	for _, n := range result.Rule1SuppressedNodes {
		if n == "synthesise-report" {
			found = true
		}
	}
	if !found {
		t.Errorf("`synthesise-report` is the local half of the straddling entry and must be "+
			"excused; suppressed set = %v", result.Rule1SuppressedNodes)
	}
}

// ACCEPTANCE 3 — the template-row package must not regress. `skrpt-builder`'s
// `{{input.VAR}}` is a teaching placeholder; its only Inputs-table row lives in
// an asset, which the promotion does not read, so it stays a WARNING. GH#1266
// must not turn the package that documents the format into a refusal — that is
// GH#1256's recurring shape and it is explicitly not this change's business.
func TestGH1266_TemplatePlaceholderStaysAWarning(t *testing.T) {
	dir := filepath.Join("testdata", "gh1194-corpus", "skrpt-builder")
	issues, _, _ := scanPkg(t, dir)

	i := findIssue(issues, CodeInputUndeclared, "VAR")
	if i == nil {
		t.Fatalf("expected `VAR` to still be reported at all; got %+v", issues)
	}
	if i.Severity != storage.SeverityError {
		return // warning, as required
	}
	t.Error("`VAR` was promoted to error — skrpt-builder would be refused for documenting " +
		"the input-table format, which is the GH#1256 shape this change must not create")
}

// The complement of the rewritten GH#975 guard, stated as its own case: a
// straddling entry excuses ITS OWN local nodes and nobody else's.
func TestGH1266_SuppressionIsScopedToTheStraddlingEntry(t *testing.T) {
	files := onePromptPkg("", "| `{{input.log}}` | Yes | the log | `x` |\n", "Summarise {{input.log}}")
	// `only-prompt` (which makes the reference) sits in a wholly-local entry.
	// A SECOND entry straddles the dep edge via a different prompt.
	files["workflows/fixture-wf.md"] = strings.Replace(
		files["workflows/fixture-wf.md"],
		`    prompt: "only-prompt"`,
		`    prompt: "only-prompt"
  - skill: "only-skill"
    step_type: "generation"
    prompt: "prompt-from-a-dependency"`, 1)

	dir := writePkg(t, files)
	issues, _, result := scanPkg(t, dir)

	if findIssue(issues, CodeInputUndeclared, "log") == nil {
		t.Errorf("`log` is referenced by `only-prompt`, whose entry is wholly local — its "+
			"declaration could only sit on `only-prompt` or the workflow, both readable. "+
			"Another entry's dep edge must not excuse it. got %+v", issues)
	}
	// And the guard is still recorded for the node it does apply to.
	if len(result.Rule1SuppressedNodes) == 0 {
		t.Error("the straddling entry's local node should still be listed as suppressed")
	}
}

// The scanner-side count exists so the population is a figure the instrument
// produces rather than one re-derived by replicating the rule outside it — the
// 48% measurement was computed externally and could drift from the rule.
// A package with no straddling entry must report nothing, or the figure is noise.
func TestGH1266_CountIsEmptyForAPackageWithNoDepEdge(t *testing.T) {
	dir := writePkg(t, onePromptPkg(
		`inputs:
  topic:
    label: "Topic"
    required: true
    type: text
`,
		"| `{{input.topic}}` | Yes | the topic | `go` |\n",
		"Write about {{input.topic}}.",
	))
	_, code, result := scanPkg(t, dir)
	if len(result.Rule1SuppressedNodes) != 0 {
		t.Errorf("clean package reported suppressed nodes %v — the count must be empty "+
			"when no execution entry straddles a dependency edge", result.Rule1SuppressedNodes)
	}
	if code != 0 {
		t.Errorf("clean package exit = %d, want 0", code)
	}
}
