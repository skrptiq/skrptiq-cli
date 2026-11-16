package scan

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/skrptiq/engine/storage"
)

// GH#1131 — `loops[].inputExpression` is a real consumption mechanism Rule 3
// could not see, so a legitimately-used input was reported unconsumed.
//
// Under hub's `caller-release.yml` policy ("any engine finding blocks — errors
// and warnings alike") that WARNING blocked first-party publication, so this was
// a live gate refusing valid published content — the false-refusal direction.
//
// The suite's centre of gravity is item 2 of the acceptance: a package whose
// input is consumed NOWHERE must still warn. Without it, "fixed" and "Rule 3
// disabled" are indistinguishable.

func unusedInputIssues(t *testing.T, dir string) []ScanIssue {
	t.Helper()
	_, _, result := scanPkg(t, dir)
	var ours []ScanIssue
	for _, i := range result.Issues {
		if i.Code == CodeInputDeclaredUnused {
			ours = append(ours, i)
		}
	}
	return ours
}

// ── The corpus: two live false refusals, and the control ──────────────────

// The two published packages that could not be published. Real, not synthetic.
func TestGH1131_Corpus_LoopConsumedInputsAreNotReportedUnused(t *testing.T) {
	for _, tc := range []struct{ pkg, input string }{
		{"batch-competitor-analysis", "competitors"},
		{"profile-social-pipeline", "platforms"},
	} {
		t.Run(tc.pkg, func(t *testing.T) {
			dir := filepath.Join("testdata", "gh1131-corpus", tc.pkg)
			issues := unusedInputIssues(t, dir)
			for _, i := range issues {
				if i.ReferencedSlug == tc.input {
					t.Errorf("input %q is consumed by inputExpression and was still reported "+
						"unused — under hub's publish policy this blocks the package: %s",
						tc.input, i.Message)
				}
			}
			// And the package must pass the gate outright: a warning is exit 1,
			// which is what blocked it.
			if _, code, _ := scanPkg(t, dir); code != 0 {
				t.Errorf("scan exit = %d, want 0 — a non-zero exit is what blocks the release", code)
			}
		})
	}
}

// THE CONTROL, and it is not synthetic: `translation-pipeline` declares the same
// kind of loop over an input and passed BEFORE this change too, because that
// input also appears in a prompt body. It must still pass — a change that simply
// switched Rule 3 off would look identical on the two packages above and only
// this one would catch it.
func TestGH1131_Corpus_TranslationPipelineStaysClean(t *testing.T) {
	dir := filepath.Join("testdata", "gh1131-corpus", "translation-pipeline")
	if issues := unusedInputIssues(t, dir); len(issues) != 0 {
		t.Errorf("the control package was reported: %+v", issues)
	}

	// NO exit-code assertion here, and the reason is worth writing down rather
	// than leaving as a gap. `scanPkg` runs with `NoResolveDeps: true`, and under
	// that this package reports two `execution_bindings_invalid_from_step`
	// errors: its `from_step` bindings name DEP-PROVIDED step titles ("Tone
	// Adaptation", "Language Polish") which cannot resolve when nothing was
	// fetched. With dep resolution on it scans clean, exit 0 — verified with the
	// built binary.
	//
	// So the exit code here is governed by an unrelated dep artefact, and
	// asserting 0 under this harness would be asserting against a different
	// instrument from the one the measurement used. The control's job is to prove
	// Rule 3 stays silent on an input the rule can legitimately see, and that is
	// what is asserted above. The other two corpus packages DO assert exit 0,
	// because for them the harness and the measurement agree.
}

// ── Item 2's other half: Rule 3 must STILL FIRE ───────────────────────────

// An input consumed nowhere at all is still reported. This is the test that
// makes the two above mean something.
func TestGH1131_InputConsumedNowhere_StillWarns(t *testing.T) {
	dir := writePkg(t, invokedPkg(`  - skill: "alpha-skill"
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
inputs:
  orphan:
    type: text
    required: true
    description: "declared and consumed by nothing"
metadata:
  trigger: manual
execution:
  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
---

## Overview

Fixture.
`,
	}))

	issues := unusedInputIssues(t, dir)
	if len(issues) != 1 {
		t.Fatalf("want exactly 1 unused-input finding for an input nothing consumes, got %+v", issues)
	}
	if issues[0].ReferencedSlug != "orphan" {
		t.Errorf("referencedSlug = %q, want orphan", issues[0].ReferencedSlug)
	}
	if issues[0].Severity != storage.SeverityWarning {
		t.Errorf("severity = %q — this change deliberately does NOT alter the severity; "+
			"that is tracked separately so it cannot hide inside a false-refusal fix",
			issues[0].Severity)
	}
}

// ── The mechanism, directly ───────────────────────────────────────────────

// Both spellings the engine accepts are seen. `storage/resolve.go:395-397` reads
// `input_expression` OR `inputExpression`, so keying on the one this issue names
// would have left the other broken.
func TestGH1131_BothInputExpressionSpellings_AreSeen(t *testing.T) {
	for _, key := range []string{"inputExpression", "input_expression"} {
		t.Run(key, func(t *testing.T) {
			dir := loopPkg(t, `  - id: "each"
    mode: "for_each"
    `+key+`: "{{input.items}}"
    steps:
      - "alpha-skill"
`)
			if issues := unusedInputIssues(t, dir); len(issues) != 0 {
				t.Errorf("an input consumed via %s was reported unused: %+v", key, issues)
			}
		})
	}
}

// A loop BODY NODE's own `{{input.X}}` is seen — the second half of the fix.
// A loop's `steps:` name nodes whose bodies consume inputs like any other
// step's, and before this they were unreachable unless the node also appeared in
// `execution`.
func TestGH1131_InputConsumedOnlyInALoopBodyNode_IsSeen(t *testing.T) {
	dir := writePkg(t, map[string]string{
		"workflows/wf.md": `---
type: workflow
id: wf
title: WF
description: "fixture"
connections:
  - target: looped-skill
    type: uses
inputs:
  body_only:
    type: text
    required: true
    description: "consumed only inside a loop body node"
metadata:
  trigger: manual
loops:
  - id: "each"
    mode: "for_each"
    inputExpression: "{{loop.seed}}"
    steps:
      - "looped-skill"
execution:
  - skill: "looped-skill"
    step_type: "generation"
    prompt: "looped-prompt"
---

## Overview

Fixture.
`,
		"skills/looped-skill.md": `---
type: skill
id: looped-skill
title: Looped Skill
description: "s"
---

Do it.
`,
		"prompts/looped-prompt.md": `---
type: prompt
id: looped-prompt
title: Looped Prompt
description: "p"
connections:
  - target: looped-skill
    type: derived_from
---

## Prompt

Work on {{input.body_only}}.
`,
	})

	if issues := unusedInputIssues(t, dir); len(issues) != 0 {
		t.Errorf("an input consumed in a loop body node was reported unused: %+v", issues)
	}
}

// Item 4 — the opposite axis must not be blinded: an input referenced by a loop
// but never DECLARED still trips Rule 1.
func TestGH1131_LoopReferencesAnUndeclaredInput_StillTripsRule1(t *testing.T) {
	dir := loopPkg(t, `  - id: "each"
    mode: "for_each"
    inputExpression: "{{input.never_declared}}"
    steps:
      - "alpha-skill"
`)
	_, _, result := scanPkg(t, dir)
	var sawRule1 bool
	for _, i := range result.Issues {
		if i.Code == CodeInputUndeclared && i.ReferencedSlug == "never_declared" {
			sawRule1 = true
		}
	}
	if !sawRule1 {
		t.Errorf("a loop referencing an undeclared input did not trip Rule 1 — making loops "+
			"visible to Rule 3 must not blind the opposite rule: %+v", result.Issues)
	}
}

// The member-slug reader takes exactly the keys the engine reads, and tolerates
// a node-less member (GH#831) rather than inventing a node for it.
func TestGH1131_LoopMemberSlugs_ReadsStepsAndVerifierOnly(t *testing.T) {
	got := loopMemberSlugs(map[string]any{
		"id":              "each",
		"mode":            "until_pass",
		"steps":           []any{"body-one", "body-two", "", 42},
		"verifier":        "check-it",
		"inputExpression": "{{input.x}}",
		"label":           "not a slug",
	})
	want := []string{"body-one", "body-two", "check-it"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("slug %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// loopPkg builds a package whose workflow carries the given `loops:` block and
// declares an `items` input, consumed only through that block.
func loopPkg(t *testing.T, loopsYAML string) string {
	t.Helper()
	return writePkg(t, invokedPkg("", map[string]string{
		"workflows/wf.md": `---
type: workflow
id: wf
title: WF
description: "fixture"
connections:
  - target: alpha-skill
    type: uses
inputs:
  items:
    type: text
    required: true
    description: "consumed only by the loop"
metadata:
  trigger: manual
loops:
` + loopsYAML + `execution:
  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
---

## Overview

Fixture.
`,
	}))
}

var _ = strings.TrimSpace
