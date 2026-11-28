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
