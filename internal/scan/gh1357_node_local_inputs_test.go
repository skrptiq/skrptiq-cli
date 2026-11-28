package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/skrptiq/engine/parse"
)

// GH#1357 Phase 1 — the three input rules were gated on the PACKAGE containing a
// workflow node, so a package shipping the same prompt with no local runner was
// graded on nothing. Measured on the live catalogue: 145 of 234 packages have no
// workflow node, 134 of the 135 in `_shared/`.
//
// Both GH#1215 directions are graded here, and the second is the one a naive fix
// fails: removing the gate would report every reference in every library bundle.
//
// `writePkg` is GH#975's helper, reused deliberately: it writes the manifest an
// author would write, so these fixtures go through the real parser rather than a
// hand-built parse.Package.

func codesIn(r ScanResult, code string) []string {
	var out []string
	for _, i := range r.Issues {
		if i.Code == code {
			out = append(out, i.ReferencedSlug)
		}
	}
	return out
}

// DIRECTION 1 — the bypass is closed. No workflow, undeclared references, reported.
func TestGH1357_WorkflowLessPackage_ReportsUndeclaredInputs(t *testing.T) {
	dir := writePkg(t, map[string]string{
		"prompts/lonely.md": `---
type: prompt
id: lonely
title: "Lonely"
description: "ships with no local runner"
---

Write about {{input.topic}} and also {{input.never_declared}}.
`,
	})

	r := runScan(t, dir)
	got := codesIn(r, CodeNodeInputUndeclared)
	if len(got) != 2 {
		t.Fatalf("got %d node.input_undeclared findings %v, want 2 (topic, never_declared).\n"+
			"A package with no workflow node must still have its node content graded — "+
			"before GH#1357 this scanned clean at exit 0.", len(got), got)
	}
	if r.WarnCount == 0 && r.ErrorCount == 0 {
		t.Error("the findings must count toward the verdict, not just print")
	}
}

// DIRECTION 2 — the false-refusal direction, and the one that matters. A library
// bundle whose inputs ARE declared must stay clean.
func TestGH1357_WorkflowLessPackage_DeclaredInputsStayClean(t *testing.T) {
	dir := writePkg(t, map[string]string{
		"prompts/declared.md": `---
type: prompt
id: declared
title: "Declared"
description: "a library prompt that declares its contract"
inputs:
  topic:
    label: "Topic"
    description: "what to write about"
    required: true
    type: text
---

Write about {{input.topic}}.
`,
	})

	if got := codesIn(runScan(t, dir), CodeNodeInputUndeclared); len(got) != 0 {
		t.Errorf("got %v, want none — a declared input must not be reported just because "+
			"no local workflow runs the node", got)
	}
}

// A declaration ANYWHERE in the package satisfies a reference anywhere in it.
// `valid-package` established this: `spaced-ref-prompt` reads `{{input.topic}}`
// while its sibling `example-prompt` declares it, and a per-node rule reported a
// correct package as broken.
func TestGH1357_ADeclarationOnASiblingNodeSatisfiesTheReference(t *testing.T) {
	dir := writePkg(t, map[string]string{
		"prompts/declarer.md": `---
type: prompt
id: declarer
title: "Declarer"
description: "declares the input"
inputs:
  topic:
    required: true
---

Nothing here.
`,
		"prompts/user.md": `---
type: prompt
id: user
title: "User"
description: "references what the sibling declared"
---

Write about {{input.topic}}.
`,
	})

	if got := codesIn(runScan(t, dir), CodeNodeInputUndeclared); len(got) != 0 {
		t.Errorf("got %v, want none — inputs are resolved for the run, not for one node", got)
	}
}

// A bare `{{MARKER}}` is not a claim on the input contract. `exec.DetectInputVariables`
// returns bare identifiers too, which is right for the runtime and too broad for a
// publish gate — `testdata/template-package` is the live proof, and it reported all
// five of its authoring markers before the reference filter was narrowed.
func TestGH1357_BareMarkersAreNotInputReferences(t *testing.T) {
	dir := writePkg(t, map[string]string{
		"prompts/template.md": `---
type: prompt
id: template
title: "Template"
description: "fill-in-the-blank markers, not inputs"
---

You are a {{ROLE}} working on {{TASK}}. Use the style of {{PERSONA_NAME}}.
`,
	})

	if got := codesIn(runScan(t, dir), CodeNodeInputUndeclared); len(got) != 0 {
		t.Errorf("got %v, want none — a node-local finding requires the reference to name "+
			"its namespace, because with no workflow there is no reach to disambiguate it", got)
	}
}

// Reference material cannot state an input contract, so it cannot break one either
// (`nodeCanStateInputContract`, GH#1287). A `{{input.X}}` in a document body is
// documentation ABOUT skrpts, not a reference BY one.
func TestGH1357_ReferenceMaterialIsNotGraded(t *testing.T) {
	dir := writePkg(t, map[string]string{
		"documents/notes.md": `---
type: document
id: notes
title: "Notes"
description: "reference material"
---

Elsewhere you would write {{input.whatever}} to read an input.
`,
	})

	if got := codesIn(runScan(t, dir), CodeNodeInputUndeclared); len(got) != 0 {
		t.Errorf("got %v, want none — nothing runs a document", got)
	}
}

// The two suppressions, graded directly because neither is reachable from a
// fixture without a live dependency resolution.
func TestGH1357_SuppressionsAreHonoured(t *testing.T) {
	pkg := &parse.Package{Nodes: []parse.NodeFile{{
		ID:      "p",
		Type:    "prompt",
		Content: "Write about {{input.topic}}.",
	}}}

	if got := checkNodeLocalInputs(pkg, "", nil, nil); len(got) != 1 {
		t.Fatalf("baseline: got %d findings, want 1", len(got))
	}

	// Already reported by the reach-based pass — this rule is additive, so it must
	// not emit a second finding for the same input name.
	if got := checkNodeLocalInputs(pkg, "", map[string]bool{"topic": true}, nil); len(got) != 0 {
		t.Errorf("reported-suppression: got %d findings, want 0 — a workflow-bearing package "+
			"must gain no duplicate", len(got))
	}

	// Excused for straddling a dependency edge: the declaration may live in the half
	// whose frontmatter is not served (GH#1266), so re-reporting it here would
	// recreate the GH#1131 / GH#1253 false positives from another direction.
	if got := checkNodeLocalInputs(pkg, "", nil, map[string]bool{"p": true}); len(got) != 0 {
		t.Errorf("dep-straddle suppression: got %d findings, want 0", len(got))
	}
}

// A declared template package is exempt — the author has already said the markers
// are blanks to fill in (`manifest.template: true`, GH#374; GH#524 suppresses the
// engine's unknown-namespace warnings on the same basis). Measured on the live
// catalogue, two of the six `templates-*` packages produced 14 of the 15 warnings
// this rule emitted before the exemption.
//
// This is the only place in Phase 1 where the rule keys on a DECLARATION rather
// than on content, and it is the shape the invocation half of GH#1357 still needs.
func TestGH1357_DeclaredTemplatePackageIsExempt(t *testing.T) {
	body := `---
type: prompt
id: blanks
title: "Blanks"
description: "a fill-in-the-blank template"
---

Write about {{input.specification}} in a {{input.tone}} tone.
`
	plain := writePkg(t, map[string]string{"prompts/blanks.md": body})
	if got := codesIn(runScan(t, plain), CodeNodeInputUndeclared); len(got) != 2 {
		t.Fatalf("control: got %v, want 2 findings without the template flag", got)
	}

	// `writePkg` writes its own manifest, so append the declaration to it.
	tmpl := writePkg(t, map[string]string{"prompts/blanks.md": body})
	f, err := os.OpenFile(filepath.Join(tmpl, "skrptiq.yaml"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	if _, err := f.WriteString("template: true\n"); err != nil {
		t.Fatalf("declare template: %v", err)
	}
	f.Close()

	if got := codesIn(runScan(t, tmpl), CodeNodeInputUndeclared); len(got) != 0 {
		t.Errorf("got %v, want none — a declared template package has already said its "+
			"markers are blanks", got)
	}
}
