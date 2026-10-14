package scan

import (
	"strings"
	"testing"

	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/engine/storage"
)

// GH#1057 second wave — a step that reads its own output, and a reference no
// step ever satisfies. Census over the published corpus before shipping (232
// scans, dep resolution on): 3 self-references and 1 unresolvable reference,
// zero false positives. The named-but-absent instances are in the rule comments.

// stepRefIssues returns both clauses' findings for a package directory.
func stepRefIssues(t *testing.T, dir string) []ScanIssue {
	t.Helper()
	_, _, result := scanPkg(t, dir)
	var ours []ScanIssue
	for _, i := range result.Issues {
		if i.Code == CodeStepReadsOwnOutput || i.Code == CodeStepRefNeverResolves {
			ours = append(ours, i)
		}
	}
	return ours
}

// checkRefs runs the clauses directly, so dependency state can be injected —
// the suppression cases cannot be reached through a scan of local files alone.
func checkRefs(t *testing.T, dir string, deps stepRefDeps) []ScanIssue {
	t.Helper()
	pkg, _, err := parse.ReadPackage(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return checkStepReferences(&pkg, dir, deps)
}

// twoStepPkg is a two-entry workflow whose SECOND step runs promptBody. The
// first step is a legitimate upstream producer, so a reference to it is the
// correct shape and a reference to the second step's own skill is the defect.
func twoStepPkg(promptBody string) map[string]string {
	return map[string]string{
		"workflows/wf.md": `---
type: workflow
id: wf
title: WF
description: "fixture"
connections:
  - target: upstream-skill
    type: uses
  - target: downstream-skill
    type: uses
execution:
  - skill: "upstream-skill"
    step_type: "generation"
    prompt: "upstream-prompt"
  - skill: "downstream-skill"
    step_type: "generation"
    prompt: "downstream-prompt"
---

## Overview

Fixture.
`,
		"skills/upstream-skill.md": `---
type: skill
id: upstream-skill
title: Upstream Skill
description: "s"
---

Produce.
`,
		"skills/downstream-skill.md": `---
type: skill
id: downstream-skill
title: Downstream Skill
description: "s"
---

Consume.
`,
		"prompts/upstream-prompt.md": `---
type: prompt
id: upstream-prompt
title: Upstream Prompt
description: "p"
---

Produce the material.
`,
		"prompts/downstream-prompt.md": `---
type: prompt
id: downstream-prompt
title: Downstream Prompt
description: "p"
---

` + promptBody + `
`,
	}
}

// ── Clause 1: a step reading its own output ───────────────────────────────

// The live shape: the corpus references a step by its skill's TITLE.
// `data-analysis-pipeline` runs findings-synthesiser on qualitative-coding and
// the prompt reads {{steps.Qualitative Coding.output}}.
func TestGH1057_StepReadingItsOwnTitle_IsRefused(t *testing.T) {
	dir := writePkg(t, twoStepPkg("Themes: {{steps.Downstream Skill.output}}"))

	issues := stepRefIssues(t, dir)
	if len(issues) != 1 {
		t.Fatalf("want 1 finding, got %+v", issues)
	}
	i := issues[0]
	if i.Code != CodeStepReadsOwnOutput {
		t.Errorf("code = %q, want %q", i.Code, CodeStepReadsOwnOutput)
	}
	if i.Severity != storage.SeverityError {
		t.Errorf("severity = %q, want error", i.Severity)
	}
	if !strings.Contains(i.Message, "downstream-prompt") || !strings.Contains(i.Message, "downstream-skill") {
		t.Errorf("message names neither the prompt nor the skill: %q", i.Message)
	}
	if i.File != "prompts/downstream-prompt.md" {
		t.Errorf("file = %q, want the prompt that carries the reference", i.File)
	}
}

// The same defect written as the skill SLUG rather than its title.
func TestGH1057_StepReadingItsOwnSlug_IsRefused(t *testing.T) {
	dir := writePkg(t, twoStepPkg("Themes: {{steps.downstream-skill.output}}"))

	issues := stepRefIssues(t, dir)
	if len(issues) != 1 || issues[0].Code != CodeStepReadsOwnOutput {
		t.Fatalf("want 1 self-reference finding, got %+v", issues)
	}
}

// A prompt-only entry is still a step (the planner's step types are
// {skill, prompt}), referenced by its own title — so it can read its own output.
func TestGH1057_PromptOnlyStepReadingItsOwnOutput_IsRefused(t *testing.T) {
	files := twoStepPkg("Nothing to see.")
	files["workflows/wf.md"] = `---
type: workflow
id: wf
title: WF
description: "fixture"
connections:
  - target: upstream-skill
    type: uses
  - target: downstream-prompt
    type: uses
execution:
  - skill: "upstream-skill"
    step_type: "generation"
    prompt: "upstream-prompt"
  - prompt: "downstream-prompt"
    step_type: "generation"
---

## Overview

Fixture.
`
	files["prompts/downstream-prompt.md"] = `---
type: prompt
id: downstream-prompt
title: Downstream Prompt
description: "p"
---

Summary so far: {{steps.Downstream Prompt.output}}
`
	dir := writePkg(t, files)

	issues := stepRefIssues(t, dir)
	if len(issues) != 1 || issues[0].Code != CodeStepReadsOwnOutput {
		t.Fatalf("a prompt-only step reading its own output was not refused: %+v", issues)
	}
}

// ACCEPTANCE, and the one that matters most: reading the UPSTREAM step is the
// normal shape of every pipeline in the catalogue.
func TestGH1057_StepReadingAnUpstreamStep_IsAccepted(t *testing.T) {
	dir := writePkg(t, twoStepPkg("Based on {{steps.Upstream Skill.output}}, continue."))

	if issues := stepRefIssues(t, dir); len(issues) != 0 {
		t.Errorf("a legitimate upstream reference was reported: %+v", issues)
	}
}

// `previous` is positional lookback and always resolvable — never reported.
func TestGH1057_PositionalPreviousRef_IsNeverReported(t *testing.T) {
	dir := writePkg(t, twoStepPkg("Based on {{steps.previous.output}}, continue."))

	if issues := stepRefIssues(t, dir); len(issues) != 0 {
		t.Errorf("{{steps.previous.output}} was reported: %+v", issues)
	}
}

// A gate must not refuse the content that documents the gate: `skrpt-builder`
// and `skrptiq-on-skrptiq` write step refs as literal examples, inside code.
// This mirrors the engine's own GH#522 behaviour for the same detector.
func TestGH1057_RefInsideMarkdownCode_IsNotReported(t *testing.T) {
	dir := writePkg(t, twoStepPkg("Write `{{steps.Downstream Skill.output}}` to read a step, e.g.\n\n```\n{{steps.Nonexistent Thing.output}}\n```\n"))

	if issues := stepRefIssues(t, dir); len(issues) != 0 {
		t.Errorf("a reference inside markdown code was reported: %+v", issues)
	}
}

// ── Clause 2: a reference no step satisfies ───────────────────────────────

// Nothing of that name exists anywhere: the "publish it or remove it" fix.
func TestGH1057_RefToSomethingThatDoesNotExist_IsRefused(t *testing.T) {
	dir := writePkg(t, twoStepPkg("From {{steps.Practice Question Generator.output}}, continue."))

	issues := stepRefIssues(t, dir)
	if len(issues) != 1 {
		t.Fatalf("want 1 finding, got %+v", issues)
	}
	i := issues[0]
	if i.Code != CodeStepRefNeverResolves || i.Severity != storage.SeverityError {
		t.Fatalf("code/severity = %q/%q, want %q/error", i.Code, i.Severity, CodeStepRefNeverResolves)
	}
	if !strings.Contains(i.Message, "no node of that name ships") {
		t.Errorf("want the does-not-exist wording, got: %q", i.Message)
	}
}

// The live instance's actual shape: the node EXISTS and no execution entry runs
// it, so the fix is to wire the stage, not to publish it. `revision-strategy-flow`
// resolves `practice-question-generator` as a dependency and never wires it.
func TestGH1057_RefToAShippedButUnwiredNode_IsRefusedWithTheWiringFix(t *testing.T) {
	files := twoStepPkg("From {{steps.Orphan Skill.output}}, continue.")
	files["skills/orphan-skill.md"] = `---
type: skill
id: orphan-skill
title: Orphan Skill
description: "ships, and no execution entry runs it"
---

Never invoked.
`
	dir := writePkg(t, files)

	issues := stepRefIssues(t, dir)
	var ours *ScanIssue
	for i := range issues {
		if issues[i].Code == CodeStepRefNeverResolves {
			ours = &issues[i]
		}
	}
	if ours == nil {
		t.Fatalf("the unwired reference was not refused: %+v", issues)
	}
	if !strings.Contains(ours.Message, "no execution entry runs a step of that name") {
		t.Errorf("want the wiring wording, got: %q", ours.Message)
	}
}

// ACCEPTANCE, and the false positive this clause would otherwise produce on
// working content: a dep-provided step is wired by SLUG and referenced by its
// TITLE, which can look nothing like the slug. `research-proposal-flow` wires
// `skill: gap-analysis` and its prompt reads
// {{steps.Research Gap Identification.output}} — the dep node's title.
func TestGH1057_RefToADepProvidedStepByItsTitle_IsAccepted(t *testing.T) {
	files := twoStepPkg("From {{steps.Research Gap Identification.output}}, continue.")
	files["workflows/wf.md"] = strings.Replace(
		files["workflows/wf.md"],
		`  - skill: "upstream-skill"`,
		`  - skill: "gap-analysis"`, 1)
	dir := writePkg(t, files)

	deps := stepRefDeps{
		Names:   map[string]bool{"gap-analysis": true, "research-gap-identification": true},
		TitleOf: map[string]string{"gap-analysis": "Research Gap Identification"},
	}
	if issues := checkRefs(t, dir, deps); len(issues) != 0 {
		t.Errorf("a dep-provided step referenced by title was reported: %+v", issues)
	}
}

// Under --no-resolve-deps nothing was fetched, so no non-local name can be
// judged at all. Same suppression Rule 1 carries on GH#858/GH#975, same reason.
func TestGH1057_PermissiveMode_SuppressesTheUnresolvableClause(t *testing.T) {
	dir := writePkg(t, twoStepPkg("From {{steps.Could Be A Dep.output}}, continue."))

	if issues := checkRefs(t, dir, stepRefDeps{Permissive: true}); len(issues) != 0 {
		t.Errorf("permissive mode still reported: %+v", issues)
	}
}

// A package with no workflow ships its nodes to be IMPORTED, and a shared
// prompt legitimately references steps its CONSUMER provides (GH#782 reuse).
// All 135 `_shared/` elements are in this class; judging them would refuse the
// reuse model itself.
func TestGH1057_PackageWithNoWorkflow_IsNeverJudged(t *testing.T) {
	dir := writePkg(t, map[string]string{
		"prompts/shared-prompt.md": `---
type: prompt
id: shared-prompt
title: Shared Prompt
description: "p"
---

From {{steps.Whatever The Consumer Calls It.output}}, continue.
`,
	})

	if issues := stepRefIssues(t, dir); len(issues) != 0 {
		t.Errorf("a workflow-less package was judged: %+v", issues)
	}
}

// A builtin step is named by its `id:`, not by a skill — `big-log-extractor-
// summariser` reads {{steps.extract-errors.output}}. Missing `id:`/`label:`
// would false-fire this clause on every builtin pipeline in the catalogue.
func TestGH1057_RefToABuiltinStepID_IsAccepted(t *testing.T) {
	files := twoStepPkg("From {{steps.extract-errors.output}}, continue.")
	files["workflows/wf.md"] = strings.Replace(
		files["workflows/wf.md"],
		`execution:
`,
		`execution:
  - id: "extract-errors"
    step_type: "local.builtin"
    context:
      operation: "regex-extract"
`, 1)
	dir := writePkg(t, files)

	if issues := stepRefIssues(t, dir); len(issues) != 0 {
		t.Errorf("a reference to a builtin step id was reported: %+v", issues)
	}
}

// ── The detectors, directly ───────────────────────────────────────────────

func TestGH1057_StepRefsIn_SkipsCodeAndPrevious(t *testing.T) {
	body := "Read {{steps.Real Step.output}} and {{steps.previous.output}}.\n" +
		"Example: `{{steps.Inline Example.output}}`\n" +
		"```\n{{steps.Fenced Example.output}}\n```\n" +
		"Again {{steps.Real Step.output}}.\n"

	got := stepRefsIn(body)
	if len(got) != 1 || got[0] != "Real Step" {
		t.Errorf("stepRefsIn = %v, want [Real Step]", got)
	}
}

func TestGH1057_RefKey_MatchesTheResolversSlugification(t *testing.T) {
	cases := map[string]string{
		"Qualitative Coding":   "qualitative-coding",
		"qualitative-coding":   "qualitative-coding",
		"Note-Taking":          "note-taking",
		"Self-Assessment":      "self-assessment",
		"Research Gap ID (v2)": "research-gap-id-v2",
	}
	for in, want := range cases {
		if got := refKey(in); got != want {
			t.Errorf("refKey(%q) = %q, want %q", in, got, want)
		}
	}
}
