package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/skrptiq/engine/parse"
)

// GH#1364 — `template: true` exempts three scan behaviours and nothing graded it,
// while `library: true` is graded in two directions. This is the predicate.
//
// WHAT IT MEASURES IS A PROXY, and the tests say so too: `{{UPPERCASE}}` is INERT
// to the engine (nothing resolves it, because the node is meant to be edited by a
// human before it runs) while `{{input.X}}` is LIVE (the identical construct with
// an obligation the engine enforces). The real question — *would the engine be
// expected to resolve this, or was it put there to be replaced* — is about
// authorial intent, and the form of the reference is the only evidence in the file.

func templatePkg(t *testing.T, content string, declare bool) parse.Package {
	t.Helper()
	files := map[string]string{"prompts/p.md": "---\ntype: prompt\nid: p\ntitle: P\ndescription: d\n---\n\n" + content + "\n"}
	dir := writePkg(t, files)
	if declare {
		f, err := os.OpenFile(filepath.Join(dir, "skrptiq.yaml"), os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatalf("open manifest: %v", err)
		}
		if _, err := f.WriteString("template: true\n"); err != nil {
			t.Fatalf("declare: %v", err)
		}
		f.Close()
	}
	pkg, _, err := parse.ReadPackage(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return pkg
}

// Direction 1 — a declaration with nothing to fill in is refused and named.
func TestGH1364_TemplateWithNoFillableNode_IsRefused(t *testing.T) {
	pkg := templatePkg(t, "A finished prompt about {{input.topic}} with no placeholders.", true)

	issues := checkTemplateDeclaration(&pkg, "")
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1 — a template with nothing to fill in must be refused", len(issues))
	}
	if issues[0].Code != CodeTemplateNoFillableNode {
		t.Errorf("code = %q, want %q", issues[0].Code, CodeTemplateNoFillableNode)
	}
	// `{{input.X}}` must NOT satisfy the predicate: it is the LIVE construct, and
	// accepting it would make every package with an input reference a template.
	if len(issues) == 0 {
		t.Error("`{{input.topic}}` satisfied the predicate — it is live, not fillable")
	}
}

// Direction 2 — THE KNOWN-GOOD CONTROL. A check refusing every declaration passes
// the known-bad case perfectly, so this is the half that decides it.
func TestGH1364_TemplateWithAFillableNode_IsAccepted(t *testing.T) {
	for _, marker := range []string{"{{ROLE}}", "{{ TASK }}", "{{PERSONA_NAME}}", "{{EXAMPLE_INPUT_1}}"} {
		pkg := templatePkg(t, "You are a "+marker+" doing work.", true)
		if issues := checkTemplateDeclaration(&pkg, ""); len(issues) != 0 {
			t.Errorf("%s was refused: %+v", marker, issues)
		}
	}
}

// An UNDECLARED package is not graded at all — the check must not reach a package
// that made no claim.
func TestGH1364_UndeclaredPackage_IsNotGraded(t *testing.T) {
	pkg := templatePkg(t, "A finished prompt with no placeholders at all.", false)
	if issues := checkTemplateDeclaration(&pkg, ""); len(issues) != 0 {
		t.Errorf("a package declaring nothing was graded: %+v", issues)
	}
}

// A lowercase or namespaced reference is not a fill-in-the-blank marker. These are
// the near-misses that would make the predicate accept anything.
func TestGH1364_NonMarkersDoNotSatisfyThePredicate(t *testing.T) {
	for _, nonMarker := range []string{
		"{{lowercase}}", "{{input.topic}}", "{{steps.First.output}}",
		"{{loop.item}}", "{{context.thing}}", "{{Mixed_Case}}",
	} {
		pkg := templatePkg(t, "Text with "+nonMarker+" in it.", true)
		if issues := checkTemplateDeclaration(&pkg, ""); len(issues) == 0 {
			t.Errorf("%s satisfied the predicate — only an inert {{UPPERCASE}} marker should",
				nonMarker)
		}
	}
}

// THE CORPUS CONTROL, and the reason this check is not wired into `scan.go` yet.
//
// Measured against the six live declarers through the real parser rather than
// against fixtures: four pass, and `templates-services` and `templates-skills` are
// refused. `templates-skills`' flag is wrong (GH#1388 fixes its content first, then
// the flag) and `templates-services` has not been adjudicated at all.
//
// This test RECORDS that state rather than asserting the ideal one, so that when
// either package changes the test fails and someone decides deliberately whether
// the check can be wired. Skipped where there is no hub checkout.
func TestGH1364_SixLiveDeclarers_RecordedState(t *testing.T) {
	base := "/Users/bencrocker/Developer/skrptiq-hub/examples"
	if _, err := os.Stat(base); err != nil {
		t.Skip("no hub checkout on this machine")
	}

	want := map[string]bool{ // true = the predicate accepts it today
		"templates-assets":    true,
		"templates-documents": true,
		"templates-prompts":   true,
		"templates-sources":   true,
		"templates-services":  false, // 27 service nodes, zero {{…}} — unadjudicated
		"templates-skills":    false, // flag is wrong; GH#1388 fixes content first
	}

	for name, accept := range want {
		dir := filepath.Join(base, name)
		pkg, _, err := parse.ReadPackage(dir)
		if err != nil {
			t.Errorf("%s: parse: %v", name, err)
			continue
		}
		if !pkg.Manifest.Template {
			t.Errorf("%s no longer declares `template: true` — update this table deliberately", name)
			continue
		}
		got := len(checkTemplateDeclaration(&pkg, dir)) == 0
		if got != accept {
			t.Errorf("%s: predicate accepts=%v, recorded %v. If the package changed, decide "+
				"deliberately whether the check can now be wired into scan.go — do not just "+
				"update this table", name, got, accept)
		}
	}
}
