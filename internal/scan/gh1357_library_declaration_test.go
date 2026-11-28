package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GH#1357 Phase 2 — the `library: true` declaration and its grading.
//
// THE CONSTRAINT THIS IS BUILT AROUND: a declaration-based exemption becomes an
// allowlist the moment it can be set without being graded. `manifest.template:
// true` is the cautionary case — parsed, consumed by three scan behaviours, graded
// nowhere (GH#1364). So this declaration is refused when it is not true.
//
// The known-good control is the deciding fixture: **a check that refuses every
// declaration passes every known-bad case perfectly** and makes the exemption
// useless.

func declare(t *testing.T, dir, line string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, "skrptiq.yaml"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		t.Fatalf("declare: %v", err)
	}
}

const libPrompt = `---
type: prompt
id: shared-prompt
title: "Shared Prompt"
description: "supplied to other packages"
inputs:
  topic:
    required: true
---

Write about {{input.topic}}.
`

const libSkill = `---
type: skill
id: shared-skill
title: "Shared Skill"
description: "supplied to other packages"
---
body
`

// THE KNOWN-GOOD CONTROL. A genuine library — executable nodes, no workflow
// running them — must pass, or the exemption is worthless.
func TestGH1357_GenuineLibrary_IsAccepted(t *testing.T) {
	dir := writePkg(t, map[string]string{
		"prompts/shared.md": libPrompt,
		"skills/shared.md":  libSkill,
	})
	declare(t, dir, "library: true")

	r := runScan(t, dir)
	for _, code := range []string{CodeLibraryShipsNoNodes, CodeLibraryRunsItsOwnNodes, CodeNodeNeverInvoked} {
		if got := codesIn(r, code); len(got) != 0 {
			t.Errorf("%s fired on a genuine library: %v", code, got)
		}
	}
}

// DISAGREEMENT 1 — it declares the shape and runs its own nodes. Refused, and the
// refusal names the node.
func TestGH1357_LibraryThatRunsItsOwnNodes_IsRefused(t *testing.T) {
	dir := writePkg(t, map[string]string{
		"prompts/shared.md": libPrompt,
		"skills/shared.md":  libSkill,
		"workflows/w.md": `---
type: workflow
id: w
title: "W"
description: "runs the package's own skill"
connections:
  - target: shared-skill
    type: uses
    position: 0
metadata:
  execution:
    - skill: shared-skill
      prompt: shared-prompt
      step_type: generation
  stepPrompts:
    shared-skill: shared-prompt
---

Runs it.
`,
	})
	declare(t, dir, "library: true")

	r := runScan(t, dir)
	got := codesIn(r, CodeLibraryRunsItsOwnNodes)
	if len(got) == 0 {
		t.Fatal("a package declaring `library: true` while running its own nodes was accepted — " +
			"the declaration must be refused when the package does not have the shape")
	}
	// EVERY self-invoked node is named, not just the first: the workflow's entry
	// invokes both `skill: shared-skill` and `prompt: shared-prompt`, so both are
	// nodes this "library" runs itself. Reporting one and hiding the other would
	// understate how far the declaration is from true.
	named := map[string]bool{}
	for _, slug := range got {
		named[slug] = true
	}
	for _, want := range []string{"shared-skill", "shared-prompt"} {
		if !named[want] {
			t.Errorf("the refusal does not name %q; it named %v", want, got)
		}
	}
	for _, i := range r.Issues {
		if i.Code == CodeLibraryRunsItsOwnNodes && !strings.Contains(i.Message, i.ReferencedSlug) {
			t.Errorf("a refusal message does not name its own node: %q", i.Message)
		}
	}
	if r.ErrorCount == 0 {
		t.Error("a refused declaration must count toward the verdict")
	}
}

// DISAGREEMENT 2 — the vacuous case. It declares a library of nodes it does not
// ship, so the declaration exempts a rule for a package with nothing to exempt.
func TestGH1357_VacuousLibraryDeclaration_IsRefused(t *testing.T) {
	dir := writePkg(t, map[string]string{
		"documents/notes.md": `---
type: document
id: notes
title: "Notes"
description: "reference material, not executable"
---
body
`,
	})
	declare(t, dir, "library: true")

	r := runScan(t, dir)
	if got := codesIn(r, CodeLibraryShipsNoNodes); len(got) != 1 {
		t.Fatalf("got %d %s findings, want 1 — a library of nothing cannot be true",
			len(got), CodeLibraryShipsNoNodes)
	}
	if r.ErrorCount == 0 {
		t.Error("a refused declaration must count toward the verdict")
	}
}

// An UNDECLARED package is untouched by any of this. The grading must not reach a
// package that made no claim.
func TestGH1357_UndeclaredPackage_IsNotGradedAsALibrary(t *testing.T) {
	dir := writePkg(t, map[string]string{
		"documents/notes.md": `---
type: document
id: notes
title: "Notes"
description: "reference material"
---
body
`,
	})

	r := runScan(t, dir)
	for _, code := range []string{CodeLibraryShipsNoNodes, CodeLibraryRunsItsOwnNodes} {
		if got := codesIn(r, code); len(got) != 0 {
			t.Errorf("%s fired on a package that declared nothing: %v", code, got)
		}
	}
}

// The declaration exempts the invocation rule — which is its purpose — and the
// exemption is reachable today only because `checkNodeInvocation`'s early return
// still covers workflow-less packages. Asserted directly so the wiring is graded
// before enforcement lands (migration order 1: hub annotates, then cli enforces).
func TestGH1357_LibraryDeclaration_ExemptsTheInvocationRule(t *testing.T) {
	files := map[string]string{
		"prompts/shared.md": libPrompt,
		"skills/orphan.md": `---
type: skill
id: orphan-skill
title: "Orphan"
description: "shipped and never invoked"
---
body
`,
		"workflows/w.md": `---
type: workflow
id: w
title: "W"
description: "runs nothing from this package"
metadata:
  execution: []
---

Runs nothing local.
`,
	}

	// Without the declaration the orphan is an error.
	plain := writePkg(t, files)
	if got := codesIn(runScan(t, plain), CodeNodeNeverInvoked); len(got) == 0 {
		t.Fatal("control: an uninvoked node in an undeclared package must still be reported")
	}

	// With it, the rule is skipped — and the declaration is still graded, so it
	// must not itself produce a finding for this shape.
	lib := writePkg(t, files)
	declare(t, lib, "library: true")
	r := runScan(t, lib)
	if got := codesIn(r, CodeNodeNeverInvoked); len(got) != 0 {
		t.Errorf("the invocation rule fired on a declared library: %v", got)
	}
	for _, code := range []string{CodeLibraryShipsNoNodes, CodeLibraryRunsItsOwnNodes} {
		if got := codesIn(r, code); len(got) != 0 {
			t.Errorf("%s fired on a library whose workflow runs nothing local: %v", code, got)
		}
	}
}
