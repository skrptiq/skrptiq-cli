package scan

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/skrptiq/engine/storage"
)

// GH#1287 — Rule 1's promotion read the Inputs table from the WORKFLOW BODY alone, so a reference
// documented in a skill or prompt stayed a `warning`. Since GH#1256 a warning passes the publish
// gate, which turned a cosmetic gap into a latent hole.
//
// The whole catalogue population was measured before widening, and it separates by node TYPE with
// no overlap: 8 table-bearing files outside `workflows/`, 7 of them `skill` nodes with real input
// names, and exactly ONE template row — `skrpt-builder/assets/node-templates-reference.md`,
// `| {{input.VAR}} | Yes | DESCRIPTION | EXAMPLE |`.
//
// THE DISCRIMINATOR IS STRUCTURAL, NOT A CONTENT HEURISTIC. Matching the literal `DESCRIPTION` and
// `EXAMPLE` cells was the obvious move and the wrong one: it asks the scanner to judge prose, it
// breaks the day a real description reads "DESCRIPTION", and it misses a template using different
// placeholders. An asset is not an executing node — never named as `skill:` or `prompt:` in an
// execution entry, declares no contract, runs no step — so a table in it cannot state this skrpt's
// input contract. That is a property of the authoring model, not of today's catalogue.
//
// Both directions are asserted, per the GH#1215 amendment: A WIDENING THAT PROMOTED EVERYTHING
// WOULD PASS THE FIRST TEST PERFECTLY.

// skillTablePkg is `onePromptPkg` with the Inputs table moved OUT of the workflow and into the
// skill, which is where all seven genuine non-workflow tables actually live.
func skillTablePkg(t *testing.T, tableRow, ref string) string {
	t.Helper()
	files := onePromptPkg("", "", ref)
	files["skills/only-skill.md"] = `---
type: skill
id: only-skill
title: Only Skill
description: "s"
---

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
` + tableRow + `
Do the thing.
`
	return writePkg(t, files)
}

// ROW 1 — PROMOTES. A reference documented in a SKILL and declared nowhere is Anti-Pattern 5 just
// as much as one documented in the workflow.
func TestGH1287_SkillTableDocumentsIt_Promotes(t *testing.T) {
	dir := skillTablePkg(t, "| `{{input.log}}` | Yes | the raw log | `ERROR db timeout` |\n",
		"Summarise this log: {{input.log}}")

	issues, code, _ := scanPkg(t, dir)
	i := findIssue(issues, CodeInputUndeclared, "log")
	if i == nil {
		t.Fatalf("no %s for log; got %+v", CodeInputUndeclared, issues)
	}
	if i.Severity != storage.SeverityError {
		t.Errorf("severity = %q, want error — the skill's Inputs table documents it, which is the "+
			"author's evidence of intent wherever the table sits", i.Severity)
	}
	if code != 2 {
		t.Errorf("scan exit = %d, want 2", code)
	}
}

// Prompts are in scope too. No prompt in the catalogue carries a table today, but prompts are where
// 195 of 377 nodes declare `inputs:` — excluding them would leave the same hole one type over.
func TestGH1287_PromptTableDocumentsIt_Promotes(t *testing.T) {
	files := onePromptPkg("", "", "Summarise this log: {{input.log}}")
	files["prompts/only-prompt.md"] = strings.Replace(
		files["prompts/only-prompt.md"],
		"## Purpose",
		"## Inputs\n\n| Name | Required | Description | Example |\n|---|---|---|---|\n"+
			"| `{{input.log}}` | Yes | the raw log | `x` |\n\n## Purpose", 1)

	dir := writePkg(t, files)
	issues, _, _ := scanPkg(t, dir)
	i := findIssue(issues, CodeInputUndeclared, "log")
	if i == nil {
		t.Fatalf("no finding for log; got %+v", issues)
	}
	if i.Severity != storage.SeverityError {
		t.Errorf("severity = %q, want error", i.Severity)
	}
}

// ROW 2 — THE ONE THAT PROVES THE DISCRIMINATOR, and it took a correction to actually prove it.
//
// The first version of this test put the asset in the package but NOT in the execution block, and
// it passed with the type filter removed — because `reach` is built from execution entries, so an
// unreferenced asset's table is never read whatever its type. The test was asserting the scoping,
// not the exclusion, and would have reported a working discriminator that did nothing.
//
// So the asset here is deliberately NAMED as a step. `bySlug` is keyed on every node id regardless
// of type, so `skill: templates` resolves the ASSET into `reach` — which is exactly the case the
// type filter exists for, and the only way to exercise it.
func TestGH1287_AssetTableDoesNotPromote(t *testing.T) {
	files := onePromptPkg("", "", "Every `{{input.VAR}}` in a prompt body needs an inputs entry.")
	// Name the asset as a step so it enters `reach`.
	files["workflows/fixture-wf.md"] = strings.Replace(
		files["workflows/fixture-wf.md"],
		`    prompt: "only-prompt"`,
		`    prompt: "only-prompt"
  - skill: "templates"
    step_type: "generation"
    prompt: "only-prompt"`, 1)
	files["assets/templates.md"] = `---
type: asset
id: templates
title: Node Templates
description: "teaches the authoring format"
---

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| ` + "`{{input.VAR}}`" + ` | Yes | DESCRIPTION | EXAMPLE |

inputs:
  INPUT_NAME:
    label: "LABEL"
`
	dir := writePkg(t, files)
	issues, _, _ := scanPkg(t, dir)
	i := findIssue(issues, CodeInputUndeclared, "VAR")
	if i == nil {
		t.Fatalf("expected `VAR` still reported at warning level; got %+v", issues)
	}
	if i.Severity != storage.SeverityWarning {
		t.Errorf("severity = %q, want warning — promoting on an asset's template row refuses the "+
			"package that documents the table format, which is the GH#1194/1213/1249 shape for a "+
			"fourth time", i.Severity)
	}
}

// ROW 2, on the real package. `skrpt-builder` carries BOTH halves: a genuine table in
// `skills/requirements-analysis.md` and the template row in `assets/`. One package, both sides of
// the discriminator.
func TestGH1287_Corpus_SkrptBuilderStaysAWarning(t *testing.T) {
	dir := filepath.Join("testdata", "gh1194-corpus", "skrpt-builder")
	issues, _, _ := scanPkg(t, dir)
	i := findIssue(issues, CodeInputUndeclared, "VAR")
	if i == nil {
		t.Fatalf("expected `VAR` to still be reported; got %+v", issues)
	}
	if i.Severity != storage.SeverityWarning {
		t.Fatalf("severity = %q, want warning — skrpt-builder must not be refused for documenting "+
			"the input-table format", i.Severity)
	}
}

// ROW 3 — UNCHANGED. A name documented in no table anywhere stays a warning: neither rung applies,
// and widening the read scope must not invent evidence that does not exist.
func TestGH1287_UndocumentedAnywhereStillWarns(t *testing.T) {
	dir := writePkg(t, onePromptPkg("", "",
		"Every `{{input.VAR}}` in a prompt body needs a matching inputs entry."))
	issues, _, _ := scanPkg(t, dir)
	i := findIssue(issues, CodeInputUndeclared, "VAR")
	if i == nil {
		t.Fatalf("no finding for VAR; got %+v", issues)
	}
	if i.Severity != storage.SeverityWarning {
		t.Errorf("severity = %q, want warning", i.Severity)
	}
}

// Every non-executing type is excluded, not just `asset`. A new node type must opt in by name
// rather than silently gain authority over a publish gate.
func TestGH1287_OnlyExecutingTypesCanStateTheContract(t *testing.T) {
	for _, tc := range []struct {
		nodeType string
		want     bool
	}{
		{"workflow", true}, {"skill", true}, {"prompt", true},
		{"asset", false}, {"document", false}, {"source", false}, {"service", false},
		{"", false}, {"somethingNew", false},
	} {
		if got := nodeCanStateInputContract(tc.nodeType); got != tc.want {
			t.Errorf("nodeCanStateInputContract(%q) = %v, want %v", tc.nodeType, got, tc.want)
		}
	}
}

// ROW 4 — NO VERDICT MOVES. The corpora are the standing form of the catalogue-wide delta, which
// was measured at 0 added / 0 removed / 0 exit changes across 99 packages before committing.
func TestGH1287_CorpusVerdictsUnchanged(t *testing.T) {
	for _, tc := range []struct {
		dir               string
		wantErr, wantWarn int
	}{
		{filepath.Join("testdata", "gh1194-corpus", "skrpt-builder"), 0, 1},
		{filepath.Join("testdata", "gh1194-corpus", "skrptiq-on-skrptiq"), 0, 2},
	} {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			_, _, result := scanPkg(t, tc.dir)
			if result.ErrorCount != tc.wantErr || result.WarnCount != tc.wantWarn {
				t.Errorf("err=%d warn=%d, want %d/%d — widening the table read scope must not move "+
					"a verdict on a package that was already graded",
					result.ErrorCount, result.WarnCount, tc.wantErr, tc.wantWarn)
			}
		})
	}
}

// A table in a node the workflow never runs is not evidence about this workflow's contract — the
// read is scoped to `reach`, matching how `declared` is built.
func TestGH1287_TableInAnUnreachedNodeIsNotEvidence(t *testing.T) {
	files := onePromptPkg("", "", "Summarise this log: {{input.log}}")
	files["skills/orphan-skill.md"] = `---
type: skill
id: orphan-skill
title: Orphan Skill
description: "never named in the execution block"
---

## Inputs

| Name | Required | Description | Example |
|---|---|---|---|
| ` + "`{{input.log}}`" + ` | Yes | the log | sample |
`
	dir := writePkg(t, files)
	issues, _, _ := scanPkg(t, dir)
	i := findIssue(issues, CodeInputUndeclared, "log")
	if i == nil {
		t.Fatalf("no finding for log; got %+v", issues)
	}
	if i.Severity != storage.SeverityWarning {
		t.Errorf("severity = %q, want warning — an unreached node's table is not evidence about "+
			"this workflow", i.Severity)
	}
}
