package scan

import (
	"path/filepath"
	"strings"
	"testing"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/storage"
)

// GH#1057 — two execution entries naming one skill produce ONE step, and the
// earlier entry's prompt is discarded. `workflow.node_never_invoked` counts that
// prompt invoked because it is NAMED, so the shape scanned clean: a package
// could ship a stage, document it, wire it, and never run it.
//
// The fixture is the artefact these tests are built around. See
// testdata/gh1057-probe/README.md — four entries, three steps, and before this
// rule it scanned with exit 0.
const probeDir = "testdata/gh1057-probe/duplicate-skill-probe"

// duplicateIssues returns only this rule's findings for a package.
func duplicateIssues(t *testing.T, dir string) []ScanIssue {
	t.Helper()
	_, _, result := scanPkg(t, dir)
	var ours []ScanIssue
	for _, i := range result.Issues {
		if i.Code == CodeDuplicateSkillEntry {
			ours = append(ours, i)
		}
	}
	return ours
}

// ── The probe ─────────────────────────────────────────────────────────────

// The headline: the duplicate is refused, it names the skill AND the prompt
// that never runs, and it fails the publish gate.
func TestGH1057_Probe_DuplicateSkillEntryIsRefused(t *testing.T) {
	issues := duplicateIssues(t, probeDir)
	if len(issues) != 1 {
		t.Fatalf("want exactly 1 duplicate finding, got %d: %+v", len(issues), issues)
	}
	i := issues[0]
	if i.ReferencedSlug != "alpha-skill" {
		t.Errorf("referencedSlug = %q, want alpha-skill", i.ReferencedSlug)
	}
	if i.Severity != storage.SeverityError {
		t.Errorf("severity = %q, want error — a duplicate is never correct", i.Severity)
	}
	// The actionable half: WHICH prompt the engine discards. Naming only the
	// skill would leave the author to work that out from the entry order.
	if !strings.Contains(i.Message, `"outline-alpha"`) {
		t.Errorf("message does not name the shadowed prompt: %q", i.Message)
	}
	if i.Field != "execution" {
		t.Errorf("field = %q, want execution", i.Field)
	}

	_, code, _ := scanPkg(t, probeDir)
	if code != 2 {
		t.Errorf("scan exit = %d, want 2 — this must gate", code)
	}
}

// The blind spot, kept as a standing assertion rather than a sentence in a
// commit message: the GH#858 rule sees `outline-alpha` as invoked, because it is
// named in the execution block. That is why GH#1057 is a second clause and not a
// repair of the first — and if someone later makes node_never_invoked test
// reachability, this test is where that decision surfaces.
func TestGH1057_Probe_NodeNeverInvokedIsSilentOnTheShadowedPrompt(t *testing.T) {
	_, _, result := scanPkg(t, probeDir)
	for _, i := range result.Issues {
		if i.Code == CodeNodeNeverInvoked && i.ReferencedSlug == "outline-alpha" {
			t.Fatalf("node_never_invoked now reports the shadowed prompt: %+v\n"+
				"If that is deliberate, the two rules overlap and this fixture's "+
				"README needs updating; it is not a free win.", i)
		}
	}
}

// The engine behaviour the rule exists for, asserted against the engine rather
// than described: four entries, three steps, the shadowed prompt's content
// absent from the plan, and the prompt-only entry sorted last.
func TestGH1057_Probe_EngineCollapsesFourEntriesToThreeSteps(t *testing.T) {
	hb, err := HydrateBundle(probeDir)
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	defer hb.Close()
	if len(hb.Workflows) != 1 {
		t.Fatalf("want 1 workflow, got %d", len(hb.Workflows))
	}

	plan, err := exec.BuildExecutionPlan(hb.DB, hb.Workflows[0].ID, nil)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	var steps []exec.PlannedStep
	for _, g := range plan.PositionGroups {
		steps = append(steps, g.Steps...)
	}
	if len(steps) != 4-1 {
		t.Fatalf("want 3 steps from 4 authored entries, got %d: %+v", len(steps), steps)
	}

	// The collapse keeps the LAST entry's prompt: alpha-skill drafts, and the
	// outline prompt is nowhere in the plan at all.
	var alpha *exec.PlannedStep
	for i := range steps {
		if steps[i].NodeID == "alpha-skill" {
			alpha = &steps[i]
		}
	}
	if alpha == nil {
		t.Fatalf("no alpha-skill step in the plan: %+v", steps)
	}
	if !strings.Contains(alpha.Content, "Draft the report.") {
		t.Errorf("alpha-skill step does not carry draft-alpha's body: %q", alpha.Content)
	}
	for _, s := range steps {
		if strings.Contains(s.Content, "Outline the report.") {
			t.Errorf("the shadowed prompt's body reached the plan on step %q — the "+
				"collapse this rule refuses did not happen, so either the engine "+
				"changed or the fixture did", s.NodeTitle)
		}
	}

	// The prompt-only entry was authored SECOND and plans LAST: only
	// `skill:`/`id:`/`action:` entries are assigned a position, so it keeps the
	// default 999. An authoring-surface gap owned elsewhere (Hub emits no
	// connection `position:`); pinned here so a fix is visible as a fixture
	// change rather than as a silent reorder.
	last := steps[len(steps)-1]
	if last.NodeID != "loner-prompt" {
		t.Errorf("last step = %q, want loner-prompt", last.NodeID)
	}
	if last.Position != 999 {
		t.Errorf("loner-prompt position = %d, want 999 (the unassigned default)", last.Position)
	}
}

// ── Depth, and the false-positive class that burned the GH#858 rule ───────

// A duplicate spanning a `parallel:` boundary collapses exactly like a flat one,
// so the walk must recurse. The GH#858 rule learned this the expensive way: a
// top-level-only read produced 34 false refusals across 10 working packages.
func TestGH1057_DuplicateAcrossAParallelGroup_IsRefused(t *testing.T) {
	dir := writePkg(t, invokedPkg(`  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
  - parallel:
    - skill: "alpha-skill"
      step_type: "generation"
      prompt: "second-prompt"
`, map[string]string{
		"prompts/second-prompt.md": `---
type: prompt
id: second-prompt
title: Second Prompt
description: "p"
---

Second.
`,
	}))

	issues := duplicateIssues(t, dir)
	if len(issues) != 1 {
		t.Fatalf("a duplicate inside a parallel group was not refused; got %+v", issues)
	}
	if !strings.Contains(issues[0].Message, `"alpha-prompt"`) {
		t.Errorf("message does not name the shadowed prompt: %q", issues[0].Message)
	}
}

// The regression guard: distinct skills in one parallel group are the normal
// shape of the published catalogue, and must never be reported. `morning-briefing`
// is the verbatim published package the GH#858 first draft falsely refused.
func TestGH1057_Corpus_DistinctSkillsInAParallelGroupAreNotReported(t *testing.T) {
	issues := duplicateIssues(t, filepath.Join("testdata", "gh858-corpus", "morning-briefing"))
	if len(issues) != 0 {
		t.Errorf("the published morning-briefing package was refused: %+v", issues)
	}
}

// One entry per skill is the overwhelming majority of the corpus: measured over
// the 97 published packages plus 135 shared elements, this rule has exactly one
// live instance (`report-writing-flow`). Everything else must stay silent.
func TestGH1057_SingleEntryPerSkill_IsAccepted(t *testing.T) {
	dir := writePkg(t, invokedPkg(`  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
`, nil))

	issues := duplicateIssues(t, dir)
	if len(issues) != 0 {
		t.Errorf("a single-entry skill was reported: %+v", issues)
	}
}

// A duplicate with no prompt on either entry is still a duplicate — the engine
// still discards one step — so the rule fires, with the fallback wording.
func TestGH1057_DuplicateWithNoPrompts_IsRefusedWithFallbackWording(t *testing.T) {
	dir := writePkg(t, invokedPkg(`  - skill: "alpha-skill"
    step_type: "generation"
  - skill: "alpha-skill"
    step_type: "generation"
  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
`, nil))

	issues := duplicateIssues(t, dir)
	if len(issues) != 1 {
		t.Fatalf("want 1 finding, got %+v", issues)
	}
	if !strings.Contains(issues[0].Message, "3 execution entries") {
		t.Errorf("message does not carry the entry count: %q", issues[0].Message)
	}
	if !strings.Contains(issues[0].Message, "only the last entry runs") {
		t.Errorf("want the no-prompt fallback wording, got: %q", issues[0].Message)
	}
}

// ── The walker, directly ──────────────────────────────────────────────────

// Authored order is what decides WHICH prompt is reported as shadowed, so the
// walk must preserve it — including across nesting, where map iteration would
// otherwise randomise the report between runs.
func TestGH1057_SkillEntriesIn_PreservesAuthoredOrderAndPairsPrompts(t *testing.T) {
	execution := []any{
		map[string]any{"skill": "one", "prompt": "p-one"},
		map[string]any{"parallel": []any{
			map[string]any{"skill": "two", "prompt": "p-two"},
			map[string]any{"skill": "one", "prompt": "p-one-again"},
		}},
		map[string]any{"id": "builtin-step", "step_type": "local.builtin"},
		map[string]any{"skill": "three"},
		map[string]any{"skill": 42},
		map[string]any{"skill": ""},
	}

	got := skillEntriesIn(execution)
	want := []skillEntry{
		{skill: "one", prompt: "p-one"},
		{skill: "two", prompt: "p-two"},
		{skill: "one", prompt: "p-one-again"},
		{skill: "three"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
