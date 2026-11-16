package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/storage"
)

// GH#1213 — `outputs:` decides where files land and no validator graded it.
//
// The suite is weighted to the NEGATIVE direction and to the drift guard, not to
// the refusals: refusing is easy, and a gate that refuses everything is
// indistinguishable from this one on the refusal cases alone.

func outputsIssues(t *testing.T, dir string) []ScanIssue {
	t.Helper()
	_, _, result := scanPkg(t, dir)
	var ours []ScanIssue
	for _, i := range result.Issues {
		switch i.Code {
		case CodeOutputsNotAList, CodeOutputsEntryNotAMapping, CodeOutputsUnknownType,
			CodeOutputsUnknownKey, CodeOutputsFilePathNotRelative:
			ours = append(ours, i)
		}
	}
	return ours
}

// outputsPkg builds a scannable package whose workflow declares `outputs:`
// verbatim from the YAML fragment given.
func outputsPkg(t *testing.T, outputsYAML string) string {
	t.Helper()
	return writePkg(t, invokedPkg(`  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
`, map[string]string{
		"workflows/wf.md": `---
type: workflow
id: wf
title: WF
description: "fixture"
connections:
  - target: alpha-skill
    type: uses
metadata:
  trigger: manual
` + outputsYAML + `execution:
  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
---

## Overview

Fixture.
`,
	}))
}

// ── Clause A: structure ───────────────────────────────────────────────────

func TestGH1213_OutputsNotAList_IsRefused(t *testing.T) {
	issues := outputsIssues(t, outputsPkg(t, "  outputs: \"notes/out.md\"\n"))
	if len(issues) != 1 || issues[0].Code != CodeOutputsNotAList {
		t.Fatalf("want one not-a-list finding, got %+v", issues)
	}
	if !strings.Contains(issues[0].Message, "LIST") {
		t.Errorf("message does not say it must be a list: %q", issues[0].Message)
	}
}

func TestGH1213_EntryNotAMapping_IsRefusedWithItsIndex(t *testing.T) {
	issues := outputsIssues(t, outputsPkg(t, "  outputs:\n    - \"notes/out.md\"\n"))
	if len(issues) != 1 || issues[0].Code != CodeOutputsEntryNotAMapping {
		t.Fatalf("want one entry-not-a-mapping finding, got %+v", issues)
	}
	if !strings.Contains(issues[0].Message, "outputs[0]") {
		t.Errorf("message does not name the index: %q", issues[0].Message)
	}
}

func TestGH1213_UnknownType_IsRefusedByName(t *testing.T) {
	issues := outputsIssues(t, outputsPkg(t, "  outputs:\n    - type: ftp\n      filePath: notes/out.md\n"))
	if len(issues) != 1 || issues[0].Code != CodeOutputsUnknownType {
		t.Fatalf("want one unknown-type finding, got %+v", issues)
	}
	for _, want := range []string{"outputs[0]", `"ftp"`, "new_node"} {
		if !strings.Contains(issues[0].Message, want) {
			t.Errorf("message missing %q: %q", want, issues[0].Message)
		}
	}
}

// A missing `type` is reported as `<missing>` rather than as an empty string, so
// the message reads as a sentence.
func TestGH1213_MissingType_SaysMissing(t *testing.T) {
	issues := outputsIssues(t, outputsPkg(t, "  outputs:\n    - filePath: notes/out.md\n"))
	if len(issues) != 1 || issues[0].Code != CodeOutputsUnknownType {
		t.Fatalf("want one unknown-type finding, got %+v", issues)
	}
	if !strings.Contains(issues[0].Message, "<missing>") {
		t.Errorf("message does not render an absent type as <missing>: %q", issues[0].Message)
	}
}

// ── Clause B: the clause only the scan can provide ────────────────────────

// A misspelled key is refused BY NAME with a did-you-mean. The engine cannot
// ever catch this: at plan time an unknown key and an absent one are identical,
// and `{type: file}` with no path is legitimate — so the typo silently becomes an
// ask and the author is told the destination "needs a path".
func TestGH1213_MisspelledFilePathKey_IsRefusedWithDidYouMean(t *testing.T) {
	issues := outputsIssues(t, outputsPkg(t, "  outputs:\n    - type: file\n      file_path: notes/out.md\n"))
	if len(issues) != 1 || issues[0].Code != CodeOutputsUnknownKey {
		t.Fatalf("want one unknown-key finding, got %+v", issues)
	}
	m := issues[0].Message
	for _, want := range []string{`"file_path"`, "Did you mean", `"filePath"`, "nothing reads"} {
		if !strings.Contains(m, want) {
			t.Errorf("message missing %q: %q", want, m)
		}
	}
	// The consequence, not just the correction: as written it behaves as an ask.
	if !strings.Contains(m, "silently") {
		t.Errorf("message does not say what the typo silently does: %q", m)
	}
}

// Every near miss an author would plausibly type resolves to the right key.
func TestGH1213_EveryKeySuggestion_ResolvesToARealKey(t *testing.T) {
	for typo, want := range destinationKeySuggestions {
		if !knownDestinationKeys[want] {
			t.Errorf("suggestion %q -> %q, which is not a real destination key", typo, want)
		}
		if knownDestinationKeys[typo] {
			t.Errorf("%q is suggested as a typo but is itself a real key", typo)
		}
	}
}

// A wrong type and a typo'd key are independent mistakes, and reporting one must
// not hide the other.
func TestGH1213_WrongTypeAndTypo_AreBothReported(t *testing.T) {
	issues := outputsIssues(t, outputsPkg(t, "  outputs:\n    - type: ftp\n      file_path: notes/out.md\n"))
	var sawType, sawKey bool
	for _, i := range issues {
		if i.Code == CodeOutputsUnknownType {
			sawType = true
		}
		if i.Code == CodeOutputsUnknownKey {
			sawKey = true
		}
	}
	if !sawType || !sawKey {
		t.Errorf("want both a type and a key finding, got %+v", issues)
	}
}

// ── Clause C: filePath shape ──────────────────────────────────────────────

func TestGH1213_AbsoluteFilePath_IsRefused(t *testing.T) {
	issues := outputsIssues(t, outputsPkg(t, "  outputs:\n    - type: file\n      filePath: /tmp/escape.md\n"))
	if len(issues) != 1 || issues[0].Code != CodeOutputsFilePathNotRelative {
		t.Fatalf("want one filePath finding, got %+v", issues)
	}
	if !strings.Contains(issues[0].Message, "absolute") {
		t.Errorf("message does not say the path is absolute: %q", issues[0].Message)
	}
}

func TestGH1213_EscapingFilePath_IsRefused(t *testing.T) {
	issues := outputsIssues(t, outputsPkg(t, "  outputs:\n    - type: file\n      filePath: ../../escape.md\n"))
	if len(issues) != 1 || issues[0].Code != CodeOutputsFilePathNotRelative {
		t.Fatalf("want one filePath finding, got %+v", issues)
	}
	if !strings.Contains(issues[0].Message, "..") {
		t.Errorf("message does not name the escaping segment: %q", issues[0].Message)
	}
}

// A `~` path publishes CLEAN today, and that is deliberate rather than an
// oversight: it is neither absolute nor an escape, so it survives both engine
// checks. GH#1244 adds the refusal to `outputs.Resolve` and this clause inherits
// it through the call, with no scan-local rule. **This test is expected to flip
// when GH#1244 lands** — it is pinned so the change is visible as a test change
// rather than as a silent behaviour shift.
func TestGH1213_TildeFilePath_IsCleanUntilGH1244(t *testing.T) {
	if issues := outputsIssues(t, outputsPkg(t, "  outputs:\n    - type: file\n      filePath: ~/escape.md\n")); len(issues) != 0 {
		t.Errorf("a `~` path was refused — if GH#1244 has landed this test should be "+
			"inverted deliberately, not deleted: %+v", issues)
	}
}

// ── The controls, without which refusing everything looks identical ───────

func TestGH1213_WellFormedDestination_IsClean(t *testing.T) {
	if issues := outputsIssues(t, outputsPkg(t, "  outputs:\n    - type: file\n      filePath: notes/out.md\n")); len(issues) != 0 {
		t.Errorf("a well-formed file destination was refused: %+v", issues)
	}
}

// `{type: file}` with NO filePath must stay clean: GH#1193 treats an absent path
// as ask-shaped and refusing it here would break that.
func TestGH1213_FileDestinationWithNoPath_IsClean(t *testing.T) {
	if issues := outputsIssues(t, outputsPkg(t, "  outputs:\n    - type: file\n")); len(issues) != 0 {
		t.Errorf("an ask-shaped destination was refused — GH#1193 says it is valid: %+v", issues)
	}
}

func TestGH1213_ClipboardAndNewNode_AreClean(t *testing.T) {
	yaml := "  outputs:\n    - type: clipboard\n    - type: new_node\n      nodeType: document\n      titleTemplate: \"{{workflow}} output\"\n"
	if issues := outputsIssues(t, outputsPkg(t, yaml)); len(issues) != 0 {
		t.Errorf("valid clipboard/new_node destinations were refused: %+v", issues)
	}
}

// A package with NO `outputs:` at all is untouched — the overwhelming majority of
// the catalogue, and the clause must not invent a finding for an absent key.
func TestGH1213_NoOutputsBlock_IsClean(t *testing.T) {
	if issues := outputsIssues(t, outputsPkg(t, "")); len(issues) != 0 {
		t.Errorf("a package declaring no outputs was reported: %+v", issues)
	}
}

// THE FALSE POSITIVE THIS ISSUE'S WORDING INVITES. `type: file` appears in 28
// published files and not one is a destination — every one is an `inputs:`
// declaration. A rule keyed on that string would refuse 28 working packages.
// These are the real published packages, not synthetic ones.
func TestGH1213_Corpus_InputsTypeFileIsNotADestination(t *testing.T) {
	for _, name := range []string{"take-notes", "book-summary-pipeline"} {
		dir := filepath.Join("testdata", "gh1213-corpus", name)
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("corpus fixture %s missing: %v", name, err)
		}
		t.Run(name, func(t *testing.T) {
			if issues := outputsIssues(t, dir); len(issues) != 0 {
				t.Errorf("published package %q was refused for an `inputs:` declaration of "+
					"type file: %+v", name, issues)
			}
		})
	}
}

// ── THE DRIFT GUARD — mandatory, and clause A's honesty ───────────────────

// Clause A mirrors the planner, and the planner is the authority. This builds a
// real plan from each shape clause A refuses and asserts the PLANNER refuses it
// too, so a divergence fails CI rather than being discovered by someone's
// install.
//
// GH#1243 exports `ValidateOutputs` from the engine; when it lands clause A
// collapses into a call to it and this guard becomes redundant by construction.
// Until then it is what makes a second authority defensible.
func TestGH1213_DriftGuard_PlannerRefusesEverythingClauseARefuses(t *testing.T) {
	shapes := []struct {
		name string
		yaml string
	}{
		{"not-a-list", "  outputs: \"notes/out.md\"\n"},
		{"entry-not-a-mapping", "  outputs:\n    - \"notes/out.md\"\n"},
		{"unknown-type", "  outputs:\n    - type: ftp\n      filePath: notes/out.md\n"},
		{"missing-type", "  outputs:\n    - filePath: notes/out.md\n"},
	}

	for _, s := range shapes {
		t.Run(s.name, func(t *testing.T) {
			dir := outputsPkg(t, s.yaml)

			// Clause A must refuse it.
			if len(outputsIssues(t, dir)) == 0 {
				t.Fatalf("clause A did not refuse %s — the guard has nothing to compare", s.name)
			}

			// And so must the planner, which is the authority.
			hb, err := HydrateBundle(dir)
			if err != nil {
				t.Fatalf("hydrate: %v", err)
			}
			defer hb.Close()
			if len(hb.Workflows) != 1 {
				t.Fatalf("want 1 workflow, got %d", len(hb.Workflows))
			}
			if _, perr := exec.BuildExecutionPlan(hb.DB, hb.Workflows[0].ID, nil); perr == nil {
				t.Errorf("the PLANNER accepted %s while clause A refuses it — the scan has "+
					"drifted from the authority it mirrors. Reconcile against "+
					"execution/planner.go, or land GH#1243 and replace clause A with a call",
					s.name)
			}
		})
	}
}

// The other direction of the same guard: a shape clause A accepts must PLAN. A
// mirror that over-refuses is as wrong as one that under-refuses, and only this
// direction catches it.
func TestGH1213_DriftGuard_PlannerAcceptsWhatClauseAAccepts(t *testing.T) {
	for _, yaml := range []string{
		"  outputs:\n    - type: file\n      filePath: notes/out.md\n",
		"  outputs:\n    - type: file\n",
		"  outputs:\n    - type: clipboard\n",
	} {
		dir := outputsPkg(t, yaml)
		if issues := outputsIssues(t, dir); len(issues) != 0 {
			t.Fatalf("clause A refused a shape this guard expects it to accept: %+v", issues)
		}
		hb, err := HydrateBundle(dir)
		if err != nil {
			t.Fatalf("hydrate: %v", err)
		}
		if _, perr := exec.BuildExecutionPlan(hb.DB, hb.Workflows[0].ID, nil); perr != nil {
			t.Errorf("the planner refused a shape clause A accepts: %v", perr)
		}
		hb.Close()
	}
}

var _ = storage.SeverityError
