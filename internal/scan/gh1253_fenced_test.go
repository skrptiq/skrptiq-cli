package scan

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/skrptiq/engine/storage"
)

// GH#1253 — Rule 3 warned on an input whose only reference sat inside a fenced
// code block, because `stripCode` removes fenced blocks before detection.
//
// Rule 1 and Rule 3 ask OPPOSITE questions of the same text: "is this reference
// declared?" versus "is this declaration consumed?". The strip is right for the
// first and wrong for the second, and they shared one set.
//
// The suite's load-bearing members are the two that prove this is a fix rather
// than a disabling: a genuinely unconsumed input must still warn, and Rule 1 must
// not start seeing fenced references.

// ── The corpus: the two live false refusals ───────────────────────────────

func TestGH1253_Corpus_FencedReferencesCountAsConsumption(t *testing.T) {
	// `exitZero` is per-package, and the asymmetry is a HARNESS property rather
	// than a difference between the packages.
	//
	// `scanPkg` runs with `NoResolveDeps: true`. Under that,
	// `documentation-pipeline` reports two `execution_bindings_invalid_from_step`
	// ERRORS, because its `from_step` bindings name a DEP-PROVIDED step title
	// ("Code Analysis") which cannot resolve when nothing was fetched. With dep
	// resolution on it scans clean, exit 0 — verified with the built binary.
	//
	// So asserting exit 0 for it here would be asserting against a different
	// instrument from the one the measurement used. The subject of this clause is
	// the unused-input finding, which is asserted for BOTH packages; the exit code
	// is asserted only where the harness and the measurement agree. (The same
	// artefact bit GH#1131's control for the same reason.)
	for _, tc := range []struct {
		pkg      string
		inputs   []string
		exitZero bool
	}{
		{"documentation-pipeline", []string{"source_code"}, false},
		{"creator-profile-builder", []string{"content_type", "creator_name"}, true},
	} {
		t.Run(tc.pkg, func(t *testing.T) {
			dir := filepath.Join("testdata", "gh1253-corpus", tc.pkg)
			issues := unusedInputIssues(t, dir)
			for _, i := range issues {
				for _, in := range tc.inputs {
					if i.ReferencedSlug == in {
						t.Errorf("input %q is referenced inside a fenced block and was still "+
							"reported unused — under hub's publish policy this blocks the "+
							"package: %s", in, i.Message)
					}
				}
			}
			if tc.exitZero {
				if _, code, _ := scanPkg(t, dir); code != 0 {
					t.Errorf("scan exit = %d, want 0 — a non-zero exit is what blocked the release", code)
				}
			}
		})
	}
}

// ── The control for the OTHER direction ───────────────────────────────────

// `weekly-report` writes load-bearing references inside INLINE backticks — "if
// `{{input.week_start}}` is provided". The `fencedBlock` comment records that an
// earlier draft stripped inline spans too and falsely refused its two correctly
// declared inputs. A fix that widened the strip in the wrong place shows up here.
func TestGH1253_Corpus_WeeklyReportStaysClean(t *testing.T) {
	dir := filepath.Join("testdata", "gh1253-corpus", "weekly-report")
	if issues := unusedInputIssues(t, dir); len(issues) != 0 {
		t.Errorf("the inline-backtick control was reported: %+v", issues)
	}
}

// ── Rule 3 must STILL FIRE — synthetic, and deliberately so ───────────────

// A declared input consumed NOWHERE is still reported.
//
// SYNTHETIC ON PURPOSE, and the measurement is the justification: the published
// catalogue contains no package with a genuinely unconsumed input, because the
// only three such warnings were the false positives this change fixes. There is
// no corpus fixture for this case to use. Without this test, "fixed" and "Rule 3
// disabled" are indistinguishable.
func TestGH1253_InputConsumedNowhere_StillWarns(t *testing.T) {
	dir := writePkg(t, invokedPkg(`  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
`, map[string]string{
		"prompts/alpha-prompt.md": `---
type: prompt
id: alpha-prompt
title: Alpha Prompt
description: "p"
connections:
  - target: alpha-skill
    type: derived_from
inputs:
  orphan:
    type: text
    required: true
    description: "declared here and consumed by nothing, in or out of a fence"
---

## Prompt

Do the work. Nothing references the declared input, anywhere.
`,
	}))

	issues := unusedInputIssues(t, dir)
	if len(issues) != 1 {
		t.Fatalf("want exactly 1 finding for an input consumed nowhere, got %+v", issues)
	}
	if issues[0].ReferencedSlug != "orphan" {
		t.Errorf("referencedSlug = %q, want orphan", issues[0].ReferencedSlug)
	}
	// GH#1249 resolved the question this assertion used to hold open: it became
	// `error`. The population at the time was zero, so no author's message
	// changed — only the label, which had been describing something other than
	// the consequence.
	if issues[0].Severity != storage.SeverityError {
		t.Errorf("severity = %q, want error (GH#1249)", issues[0].Severity)
	}
}

// ── The mechanism, and the leak that must not happen ──────────────────────

// The headline, minimally: the SAME reference, inside a fence, is consumption.
func TestGH1253_ReferenceInsideAFence_IsConsumption(t *testing.T) {
	fenced := promptWithBody(t, `inputs:
  code_in:
    type: text
    required: true
    description: "referenced only inside a fence"
`, "Analyze this:\n\n```\n{{input.code_in}}\n```\n")

	if issues := unusedInputIssues(t, fenced); len(issues) != 0 {
		t.Errorf("a reference inside a fenced block was not counted as consumption: %+v", issues)
	}
}

// The same declaration with the reference in prose — the before/after pair in one
// place, so the fence is shown to be the only variable.
func TestGH1253_SameReferenceInProse_IsAlsoConsumption(t *testing.T) {
	prose := promptWithBody(t, `inputs:
  code_in:
    type: text
    required: true
    description: "referenced in prose"
`, "Analyze this: {{input.code_in}}\n")

	if issues := unusedInputIssues(t, prose); len(issues) != 0 {
		t.Errorf("a prose reference was not counted as consumption: %+v", issues)
	}
}

// ITEM 5 — THE LEAK THAT MUST NOT HAPPEN. Rule 1 asks whether a REFERENCE is
// declared, and a `{{input.X}}` inside a fence may be a teaching example. The
// widening is Rule 3's only: a fenced reference that is NOT declared must still
// not trip Rule 1, or every package carrying an example would be refused.
func TestGH1253_FencedReferenceAlone_DoesNotTripRule1(t *testing.T) {
	dir := promptWithBody(t, "", "Here is how you would write it:\n\n```\n{{input.never_declared_anywhere}}\n```\n")

	_, _, result := scanPkg(t, dir)
	for _, i := range result.Issues {
		if i.Code == CodeInputUndeclared && i.ReferencedSlug == "never_declared_anywhere" {
			t.Errorf("a fenced reference tripped Rule 1 — the Rule 3 widening has leaked into "+
				"the rule that must stay lenient, and every package with an example would "+
				"now be refused: %s", i.Message)
		}
	}
}

// promptWithBody builds a package whose invoked prompt carries the given `inputs:`
// fragment in its frontmatter and the given body.
func promptWithBody(t *testing.T, inputsYAML, body string) string {
	t.Helper()
	return writePkg(t, invokedPkg(`  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
`, map[string]string{
		"prompts/alpha-prompt.md": `---
type: prompt
id: alpha-prompt
title: Alpha Prompt
description: "p"
connections:
  - target: alpha-skill
    type: derived_from
` + inputsYAML + `---

## Prompt

` + body,
	}))
}

var _ = strings.TrimSpace
