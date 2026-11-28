package scan

import (
	"os"
	"path/filepath"
	"testing"
)

// GH#1357 Phase 2 — the declaration half.
//
// WHAT IS HERE AND WHAT IS NOT. `template: true` exempting the invocation rule is
// implemented and graded below. The `library: true` declaration is NOT, and the
// reason is a boundary rather than a decision: `parse.ReadPackage` leaves
// `Manifest.Raw` nil (probed), so a manifest key cli does not already have a typed
// field for is unreadable from this repo — and adding the field means editing the
// engine, which lives in the app repo. Reported on GH#1357 rather than worked
// around; a hand-rolled read of `skrptiq.yaml` would make cli a second YAML parse
// authority, which `TestNoDirectYAMLv3Import` exists to prevent.

// declareTemplate appends `template: true` to a fixture's manifest. `writePkg`
// writes its own manifest, so the declaration is appended rather than passed in.
func declareTemplate(t *testing.T, dir string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, "skrptiq.yaml"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString("template: true\n"); err != nil {
		t.Fatalf("declare template: %v", err)
	}
}

const uninvokedWorkflow = `---
type: workflow
id: runner
title: "Runner"
description: "runs one of the two skills"
connections:
  - target: used-skill
    type: uses
    position: 0
metadata:
  execution:
    - skill: used-skill
      prompt: used-prompt
      step_type: generation
  stepPrompts:
    used-skill: used-prompt
---

Runs one of them.
`

func invocationFixture(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		"workflows/runner.md": uninvokedWorkflow,
		"skills/used.md": `---
type: skill
id: used-skill
title: "Used Skill"
description: "invoked by the workflow"
---
body
`,
		"prompts/used.md": `---
type: prompt
id: used-prompt
title: "Used Prompt"
description: "invoked by the workflow"
---
body
`,
		"skills/orphan.md": `---
type: skill
id: orphan-skill
title: "Orphan Skill"
description: "shipped and never invoked"
---
body
`,
	}
}

// The control: without the declaration, an uninvoked node is an error. This is the
// behaviour the exemption must not weaken for undeclared packages.
func TestGH1357_UninvokedNode_IsStillAnErrorWithoutTheDeclaration(t *testing.T) {
	r := runScan(t, writePkg(t, invocationFixture(t)))
	if got := codesIn(r, CodeNodeNeverInvoked); len(got) != 1 {
		t.Fatalf("got %v, want one %s for orphan-skill — the exemption must not "+
			"relax an undeclared package", got, CodeNodeNeverInvoked)
	}
}

// A DECLARED template package is exempt: material nobody runs is what a template
// is, so reporting its nodes as uninvoked refuses it for being what it declares.
// `templates-prompts` and `templates-skills` are two of the six live declarers and
// are among the 111 packages this rule reaches once enforcement lands.
func TestGH1357_DeclaredTemplatePackage_IsExemptFromTheInvocationRule(t *testing.T) {
	dir := writePkg(t, invocationFixture(t))
	declareTemplate(t, dir)

	if got := codesIn(runScan(t, dir), CodeNodeNeverInvoked); len(got) != 0 {
		t.Errorf("got %v, want none — a declared template package ships material nobody "+
			"runs, which is the shape it declares", got)
	}
}

// The exemption is scoped to this rule. A template declaration must not switch off
// an unrelated finding, or it becomes a general-purpose silencer.
func TestGH1357_TemplateExemption_DoesNotSilenceOtherRules(t *testing.T) {
	files := invocationFixture(t)
	files["skills/orphan.md"] = `---
type: skill
id: orphan-skill
title: "Orphan Skill"
description: "shipped, never invoked, and references an undeclared input"
---

Write about {{input.undeclared_thing}}.
`
	dir := writePkg(t, files)
	declareTemplate(t, dir)

	r := runScan(t, dir)
	if got := codesIn(r, CodeNodeNeverInvoked); len(got) != 0 {
		t.Errorf("invocation rule: got %v, want none", got)
	}
	// The input rules have their own template exemption (GH#1357 Phase 1), so this
	// asserts the two exemptions are the same decision rather than one leaking into
	// the other: both are keyed on the same declaration, deliberately.
	if got := codesIn(r, CodeNodeInputUndeclared); len(got) != 0 {
		t.Errorf("input rule: got %v — a template package's markers are blanks, "+
			"which Phase 1 already established", got)
	}
}

// GH#1364 — THE DECLARED INTERIM.
//
// `template: true` is now consumed by three scan behaviours (the engine's
// unknown-namespace suppression via GH#524, the Phase 1 input rule, and the
// invocation rule above) and NOTHING grades it. A package can declare it and be
// exempted from all three without any check that it has a template property.
//
// This test exists so that is impossible to FORGET rather than impossible to keep
// (GH#1203). It is SKIPPED, not failing: the interim is open-ended, so a permanent
// red would be noise while a skip is a question someone asks. When GH#1364 lands,
// DELETE this test — it goes green by deletion, and the grading's own tests replace
// it.
func TestGH1364_TemplateDeclarationIsUngraded_SkippedUntilItIsGraded(t *testing.T) {
	t.Skip("GH#1364: the predicate is WRITTEN and tested (checkTemplateDeclaration) but not " +
		"wired into scan.go, so nothing grades `template: true` in practice yet. Wiring " +
		"waits on GH#1388 fixing templates-skills' content and on templates-services being " +
		"adjudicated — both named on the function. Delete this test when the append lands.")
}
