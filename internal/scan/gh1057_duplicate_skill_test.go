package scan

import (
	"path/filepath"
	"strings"
	"testing"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/storage"
)

// GH#1057 — two execution entries naming one skill cannot be addressed
// separately: both steps carry the skill's title, which is the only thing a
// `from_step` can name. `workflow.node_never_invoked` counts both prompts
// invoked because they are NAMED, so the shape scanned clean: a package could
// ship a stage, document it, wire it, and have nothing able to consume it.
//
// WHAT CHANGED UNDER THIS FILE: until app GH#1092 the two entries collapsed to
// ONE step and the earlier prompt never ran, and the engine probe below
// asserted exactly that. GH#1092 made each entry its own step — both prompts
// run now — and explicitly left this rule standing: "same-skill-twice is now
// EXPRESSIBLE, not supported ... plan-time expressible, publish-time refused."
// The probe was re-asserted to the new behaviour; the refusal was not touched,
// because it never rested on the collapse. See invocation.go's clause comment.
//
// The fixture is the artefact these tests are built around. See
// testdata/gh1057-probe/README.md — four entries, four steps since GH#1092, and
// before this rule it scanned with exit 0.
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

// The headline: the duplicate is refused, it names the skill AND the colliding
// stages, and it fails the publish gate.
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
	// The actionable half: WHICH stages collide. Naming only the skill would
	// leave the author to work that out from the entry order. Since GH#1092 the
	// ambiguity is symmetric — both prompts run, neither is "the shadowed one" —
	// so BOTH must be named, and the message must claim no discard.
	if !strings.Contains(i.Message, `"outline-alpha"`) {
		t.Errorf("message does not name the first colliding stage: %q", i.Message)
	}
	if !strings.Contains(i.Message, `"draft-alpha"`) {
		t.Errorf("message does not name the second colliding stage: %q", i.Message)
	}
	for _, stale := range []string{"never runs", "only the last entry", "resolves a skill to one step"} {
		if strings.Contains(i.Message, stale) {
			t.Errorf("message still asserts the GH#1092 collapse (%q): %q", stale, i.Message)
		}
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
// ADDRESSABILITY rather than naming, this test is where that decision surfaces.
//
// Renamed with the probe below: "shadowed" was the collapse's word, and since
// GH#1092 nothing shadows `outline-alpha` — it runs. What the GH#858 rule still
// cannot see is that nothing can bind to it.
func TestGH1057_Probe_NodeNeverInvokedIsSilentOnTheDuplicatesPrompts(t *testing.T) {
	_, _, result := scanPkg(t, probeDir)
	for _, i := range result.Issues {
		if i.Code == CodeNodeNeverInvoked && i.ReferencedSlug == "outline-alpha" {
			t.Fatalf("node_never_invoked now reports the duplicate's first prompt: %+v\n"+
				"If that is deliberate, the two rules overlap and this fixture's "+
				"README needs updating; it is not a free win.", i)
		}
	}
}

// The engine behaviour the rule exists for, asserted against the engine rather
// than described: four authored entries produce FOUR steps, the skill named
// twice produces two distinct steps carrying their own prompts and their own
// declared outputs, and the prompt-only entry still sorts last.
//
// THIS PROBE USED TO ASSERT THE OPPOSITE, and that is the point of the name
// change. It was `TestGH1057_Probe_EngineCollapsesFourEntriesToThreeSteps`, and
// it went red on app GH#1092 — "an execution entry is a STEP, and the engine
// stops choosing silently" — which fixed the overwrite it documented. A probe
// of a defect, left asserting the defect, is a standing argument for restoring
// the silence; the old name was the loudest part of that argument, so it is
// gone rather than amended.
//
// What the probe is FOR has not changed: the engine's half of GH#1057's ruling.
// GH#1092 made the shape expressible and left the refusal alone, on the reason
// asserted at the bottom of this test — both steps carry one title, which is
// the only thing a `from_step` can name.
func TestGH1057_Probe_FourEntriesProduceFourSteps_SinceGH1092(t *testing.T) {
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
	if len(steps) != 4 {
		t.Fatalf("want 4 steps from 4 authored entries, got %d: %+v", len(steps), steps)
	}

	// Two alpha-skill steps, in authored order, each carrying ITS OWN entry's
	// prompt. Asserted on both bodies rather than on a count, because a count
	// would also pass if the clone duplicated one prompt twice — which is the
	// failure mode a cloning planner actually has.
	var alpha []exec.PlannedStep
	for _, s := range steps {
		if s.NodeID == "alpha-skill" {
			alpha = append(alpha, s)
		}
	}
	if len(alpha) != 2 {
		t.Fatalf("want 2 alpha-skill steps, got %d: %+v", len(alpha), alpha)
	}
	if !strings.Contains(alpha[0].Content, "Outline the report.") {
		t.Errorf("first alpha-skill step does not carry outline-alpha's body: %q", alpha[0].Content)
	}
	if !strings.Contains(alpha[1].Content, "Draft the report.") {
		t.Errorf("second alpha-skill step does not carry draft-alpha's body: %q", alpha[1].Content)
	}
	if alpha[0].Position != 0 || alpha[1].Position != 1 {
		t.Errorf("alpha-skill positions = %d, %d; want 0, 1 — authored order",
			alpha[0].Position, alpha[1].Position)
	}

	// Each step owns what its entry declared. GH#1092's clone "starts empty on
	// everything the entry owns", so the outputs are the cheapest proof that
	// the second step is a real second step and not a view of the first.
	wantOutputs := []string{"outline", "draft"}
	for i, want := range wantOutputs {
		if len(alpha[i].Outputs) != 1 || alpha[i].Outputs[0].Name != want {
			t.Errorf("alpha-skill step %d outputs = %+v, want exactly one named %q",
				i, alpha[i].Outputs, want)
		}
	}

	// AND THE REASON THE SCAN RULE STILL REFUSES THIS, asserted against the
	// engine rather than left in prose: the two steps share one title. That is
	// the only handle `from_step` has (storage/validate.go — "the from_step is
	// the dep skill's TITLE"), so no binding can say which step it means. If
	// step identity ever becomes addressable, this is the assertion that fails
	// and the rule that gets revisited.
	if alpha[0].NodeTitle != alpha[1].NodeTitle {
		t.Errorf("the two alpha-skill steps no longer share a title (%q vs %q) — "+
			"step identity may now be addressable, which is GH#1057's stated "+
			"precondition for retiring this rule; do not just update this test",
			alpha[0].NodeTitle, alpha[1].NodeTitle)
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

// A duplicate spanning a `parallel:` boundary is ambiguous exactly like a flat
// one, so the walk must recurse. The GH#858 rule learned this the expensive way:
// a top-level-only read produced 34 false refusals across 10 working packages.
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
		t.Errorf("message does not name the colliding stage: %q", issues[0].Message)
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

// A duplicate is a duplicate whether or not the entries name prompts — what
// changes is how much the message can say. Both branches of that are asserted
// here, because the prompt list is the only part of the message that varies and
// the entry count is the part an author needs either way.
//
// Renamed from `..._WithNoPrompts_IsRefusedWithFallbackWording`: its fixture
// does name a prompt (on the last entry), so it never reached the fallback, and
// the wording it asserted — "only the last entry runs" — is the GH#1092 collapse.
func TestGH1057_DuplicateMessage_CarriesTheEntryCountWithAndWithoutPrompts(t *testing.T) {
	// Prompts on some entries: every one of them is named, in authored order.
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
	if !strings.Contains(issues[0].Message, `"alpha-prompt"`) {
		t.Errorf("message does not name the one prompt in the group: %q", issues[0].Message)
	}

	// No prompt anywhere in the group: the message drops the stage list rather
	// than emitting an empty one, and still carries the count.
	bare := writePkg(t, invokedPkg(`  - skill: "alpha-skill"
    step_type: "generation"
  - skill: "alpha-skill"
    step_type: "generation"
`, nil))

	bareIssues := duplicateIssues(t, bare)
	if len(bareIssues) != 1 {
		t.Fatalf("want 1 finding on the prompt-less duplicate, got %+v", bareIssues)
	}
	if !strings.Contains(bareIssues[0].Message, "2 execution entries") {
		t.Errorf("message does not carry the entry count: %q", bareIssues[0].Message)
	}
	if strings.Contains(bareIssues[0].Message, "colliding stages run") {
		t.Errorf("message emitted an empty stage list: %q", bareIssues[0].Message)
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
