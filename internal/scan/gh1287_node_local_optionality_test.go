package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skrptiq/engine/parse"
)

// GH#1287 — Rule 2 was assembled from `reach`, so a package with no workflow had
// its input OPTIONALITY ungraded. Rule 2 never needed reach: `required:`, the
// declaration's own prose and a node-local `## Inputs` table are the whole of its
// subject. The gate's answer therefore depended on whether a `workflows/`
// directory existed — a fact unrelated to the question being asked.
//
// THE PLANTED SHAPE IS A DECLARATION WITH NO `required:` KEY whose optionality is
// stated in prose. It cannot occur in the live catalogue: 432 distinct declared
// input names across all 234 packages carry an explicit `required:`, which is why
// this fixture is synthetic and why the corpus assertion at the bottom exists.

const optionalityNode = `---
type: prompt
id: p
title: "P"
description: "the same prompt in both packages"
inputs:
  tone:
    description: "optional; defaults to neutral when the caller omits it"
---

Write in {{input.tone}}.
`

const optionalityRunner = `---
type: workflow
id: w
title: "W"
description: "runs the prompt"
connections:
  - target: p
    type: uses
    position: 0
metadata:
  execution:
    - prompt: p
      step_type: generation
---

Runs it.
`

func optionalityCodes(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	for _, i := range runScan(t, dir).Issues {
		out = append(out, i.Code)
	}
	return out
}

func hasCode(codes []string, want string) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

// DIRECTION 1 — the gap is closed. A workflow-less package earns the finding.
//
// The fixture's premise is asserted before its conclusion: if the prompt stopped
// parsing, or stopped declaring `tone`, or gained a `required:` key, the rule
// would have nothing to find and this test would otherwise pass in silence.
func TestGH1287_WorkflowLessPackage_GradesInputOptionality(t *testing.T) {
	dir := writePkg(t, map[string]string{"prompts/p.md": optionalityNode})

	pkg, _, err := parse.ReadPackage(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, n := range pkg.Nodes {
		if n.Type == "workflow" {
			t.Fatalf("the fixture ships a workflow — it is not the workflow-less case")
		}
	}
	declared, _ := nodeLocalContract(&pkg)
	d, ok := declared["tone"]
	if !ok {
		t.Fatalf("the fixture declares no %q input — the premise failed, so a finding "+
			"of zero would say nothing about the rule", "tone")
	}
	if d.hasRequired {
		t.Fatalf("the fixture's %q declares required: — there is nothing for Rule 2 to find", "tone")
	}

	codes := optionalityCodes(t, dir)
	if !hasCode(codes, CodeNodeInputOptionalityUndeclared) {
		t.Fatalf("got %v, want a %s — before GH#1287 this package scanned with its "+
			"optionality ungraded", codes, CodeNodeInputOptionalityUndeclared)
	}
}

// DIRECTION 1b — the asymmetry itself, which is the actual defect. The SAME node
// content, with and without a local runner, must earn the same verdict about its
// optionality. The workflow-bearing half is asserted first: if the reach-based
// rule stopped firing, the comparison would be vacuous.
func TestGH1287_TheSameNodeContentIsGradedWithAndWithoutARunner(t *testing.T) {
	withWf := writePkg(t, map[string]string{
		"prompts/p.md":   optionalityNode,
		"workflows/w.md": optionalityRunner,
	})
	alone := writePkg(t, map[string]string{"prompts/p.md": optionalityNode})

	wfCodes := optionalityCodes(t, withWf)
	if !hasCode(wfCodes, CodeInputOptionalityUndeclared) {
		t.Fatalf("the workflow-bearing package did not earn %s — the premise of the "+
			"asymmetry is gone, got %v", CodeInputOptionalityUndeclared, wfCodes)
	}

	aloneCodes := optionalityCodes(t, alone)
	if !hasCode(aloneCodes, CodeNodeInputOptionalityUndeclared) {
		t.Errorf("byte-identical node content is graded with a runner (%v) and not "+
			"without one (%v) — that asymmetry is the defect", wfCodes, aloneCodes)
	}
}

// NO DUPLICATE. A workflow-bearing package must earn exactly one finding for one
// declaration, not the reach-based one plus the node-local one. The suppression is
// by input name, the granularity the reach pass itself uses.
func TestGH1287_AWorkflowBearingPackageGainsNoDuplicate(t *testing.T) {
	dir := writePkg(t, map[string]string{
		"prompts/p.md":   optionalityNode,
		"workflows/w.md": optionalityRunner,
	})

	reach, local := 0, 0
	for _, i := range runScan(t, dir).Issues {
		switch i.Code {
		case CodeInputOptionalityUndeclared:
			reach++
		case CodeNodeInputOptionalityUndeclared:
			local++
		}
	}
	if reach != 1 {
		t.Fatalf("the reach-based rule emitted %d findings, want 1 — the premise failed", reach)
	}
	if local != 0 {
		t.Errorf("the node-local rule added %d finding(s) for a declaration the reach-based "+
			"rule already reported — one authoring mistake, one finding", local)
	}
}

// DIRECTION 2 — the known-good control, and the half that decides whether the rule
// survives contact with the catalogue. A declaration that states its optionality
// properly must stay clean, with and without a runner.
func TestGH1287_ADeclaredRequiredMarkerStaysClean(t *testing.T) {
	const good = `---
type: prompt
id: p
title: "P"
description: "declares optionality the way the catalogue does"
inputs:
  tone:
    required: false
    description: "optional; defaults to neutral when the caller omits it"
---

Write in {{input.tone}}.
`
	for name, files := range map[string]map[string]string{
		"workflow-less": {"prompts/p.md": good},
		"with a runner": {"prompts/p.md": good, "workflows/w.md": optionalityRunner},
	} {
		codes := optionalityCodes(t, writePkg(t, files))
		if hasCode(codes, CodeNodeInputOptionalityUndeclared) || hasCode(codes, CodeInputOptionalityUndeclared) {
			t.Errorf("%s: got %v — an explicit required: marker is exactly what the rule "+
				"asks for and must never be refused", name, codes)
		}
	}
}

// THE TWO TRIGGERS, separately. Prose is one; a node-local `## Inputs` table row is
// the other, and the table branch is the one a mis-written fixture silently
// disables — `inputTableRow` matches `| {{input.tone}} | No |`, never `| tone | No |`.
// So the table is asserted to have PARSED before the finding is concluded from it.
func TestGH1287_BothTriggersFire(t *testing.T) {
	const tableOnly = `---
type: prompt
id: p
title: "P"
description: "optionality documented in a table, not declared"
inputs:
  tone:
    description: "the voice to write in"
---

## Inputs

| Name | Required | Description |
|------|----------|-------------|
| {{input.tone}} | No | the voice |

Write in {{input.tone}}.
`
	dir := writePkg(t, map[string]string{"prompts/p.md": tableOnly})
	pkg, _, err := parse.ReadPackage(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, tableRequired := nodeLocalContract(&pkg)
	if _, ok := tableRequired["tone"]; !ok {
		t.Fatalf("the fixture's Inputs table parsed no row for %q — the table branch is "+
			"not being exercised and a pass here would mean nothing", "tone")
	}
	// The two triggers must be ISOLATED here, or a pass would not say which one
	// fired. This fixture's description states no default, so only the table can
	// be the cause.
	if proseDefault.MatchString("the voice to write in") {
		t.Fatalf("the fixture's description matches proseDefault — the prose trigger is " +
			"also live, so this case no longer isolates the table branch")
	}

	if codes := optionalityCodes(t, dir); !hasCode(codes, CodeNodeInputOptionalityUndeclared) {
		t.Errorf("table branch: got %v, want %s", codes, CodeNodeInputOptionalityUndeclared)
	}

	// Neither trigger: declared, no required:, and optionality stated NOWHERE. The
	// rule must stay silent — it reports a contradiction between two statements the
	// author made, not the absence of one.
	const neither = `---
type: prompt
id: p
title: "P"
description: "no table, no prose default"
inputs:
  tone:
    description: "the voice to write in"
---

Write in {{input.tone}}.
`
	if codes := optionalityCodes(t, writePkg(t, map[string]string{"prompts/p.md": neither})); hasCode(codes, CodeNodeInputOptionalityUndeclared) {
		t.Errorf("got %v — with optionality stated nowhere there is no contradiction to "+
			"report, and the reach-based rule is silent on the same shape", codes)
	}
}

// THE TEMPLATE DECISION, asserted because it is a decision. `checkNodeLocalInputs`
// exempts `template: true`; this rule deliberately does not, because
// `checkWorkflowInputs` does not either — so inheriting the exemption would close
// one asymmetry by opening another. An explicit `inputs:` block is a contract the
// author wrote, not a blank to fill in.
func TestGH1287_ATemplatePackageIsStillGradedOnOptionality(t *testing.T) {
	dir := writePkg(t, map[string]string{"prompts/p.md": optionalityNode})
	f, err := os.OpenFile(filepath.Join(dir, "skrptiq.yaml"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	if _, err := f.WriteString("template: true\n"); err != nil {
		t.Fatalf("declare template: %v", err)
	}
	f.Close()

	pkg, _, err := parse.ReadPackage(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !pkg.Manifest.Template {
		t.Fatalf("the declaration did not take — the exemption is not being exercised")
	}

	codes := optionalityCodes(t, dir)
	if !hasCode(codes, CodeNodeInputOptionalityUndeclared) {
		t.Errorf("got %v, want %s — the template exemption covers bare {{MARKER}} blanks, "+
			"not an explicit inputs: block, and the reach-based Rule 2 grades template "+
			"packages today", codes, CodeNodeInputOptionalityUndeclared)
	}
	// And the exemption it must NOT disturb: Rule 1 node-local stays exempt.
	if hasCode(codes, CodeNodeInputUndeclared) {
		t.Errorf("got %v — the template exemption on the node-local Rule 1 must be "+
			"untouched by this rule", codes)
	}
}

// Reference material cannot state an input contract, so it cannot breach one
// (`nodeCanStateInputContract`, GH#1287). An `inputs:` block in a document is
// documentation about skrpts, not a declaration by one.
func TestGH1287_ReferenceMaterialIsNotGraded(t *testing.T) {
	dir := writePkg(t, map[string]string{
		"documents/notes.md": `---
type: document
id: notes
title: "Notes"
description: "reference material"
inputs:
  tone:
    description: "optional; defaults to neutral"
---

Elsewhere you would declare an input like that.
`,
	})
	if codes := optionalityCodes(t, dir); hasCode(codes, CodeNodeInputOptionalityUndeclared) {
		t.Errorf("got %v, want no %s — nothing runs a document", codes, CodeNodeInputOptionalityUndeclared)
	}
}

// THE CORPUS ASSERTION, with a floor. The rule must refuse nothing live: 432
// distinct declared input names across the catalogue all carry `required:`. The
// floor is here because a walk that finds nothing passes in silence, and the
// declaration count is asserted too — a corpus with no declarations would satisfy
// this rule's predicate vacuously.
func TestGH1287_NoLivePackageIsRefused(t *testing.T) {
	base := "/Users/bencrocker/Developer/skrptiq-hub/examples"
	if _, err := os.Stat(base); err != nil {
		t.Skip("no hub checkout on this machine")
	}

	var dirs []string
	for _, root := range []string{base, filepath.Join(base, "_shared")} {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("read %s: %v", root, err)
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "_shared" {
				continue
			}
			d := filepath.Join(root, e.Name())
			if _, err := os.Stat(filepath.Join(d, "skrptiq.yaml")); err == nil {
				dirs = append(dirs, d)
			}
		}
	}
	if len(dirs) < 200 {
		t.Fatalf("found only %d live packages — the corpus walk is wrong and this "+
			"assertion would pass vacuously", len(dirs))
	}

	declaredNames, refused := 0, []string{}
	for _, d := range dirs {
		pkg, _, err := parse.ReadPackage(d)
		if err != nil {
			continue // parse failures are other rules' subject
		}
		declared, _ := nodeLocalContract(&pkg)
		declaredNames += len(declared)
		if len(checkNodeLocalOptionality(&pkg, d, nil)) > 0 {
			refused = append(refused, filepath.Base(d))
		}
	}
	if declaredNames < 300 {
		t.Fatalf("the corpus walk found only %d declared input names — the rule's subject "+
			"is barely present and a zero would say nothing", declaredNames)
	}
	if len(refused) != 0 {
		t.Errorf("the rule refuses %d live packages %v — it must refuse none. If a "+
			"package genuinely stopped declaring optionality, fix the package; if the "+
			"predicate widened, fix the rule", len(refused), strings.Join(refused, " "))
	}
	t.Logf("corpus: %d live packages, %d declared input names, %d refused",
		len(dirs), declaredNames, len(refused))
}
