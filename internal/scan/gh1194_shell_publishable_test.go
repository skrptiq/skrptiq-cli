package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skrptiq/engine/storage"
)

// GH#1194 — K-046 rules raw shell non-publishable and nothing enforced it.
//
// The rule's whole risk is over-reach, so the tests are weighted to the NEGATIVE
// direction: a control with no shell, a control with a different local.* type,
// the local-only path, and the two REAL published packages whose prose lists
// `local.shell` as part of the step-type vocabulary. GH#858's 34 false refusals
// were found by the corpus, not by invented cases.

// shellIssues returns only this rule's findings for a package.
func shellIssues(t *testing.T, dir string) []ScanIssue {
	t.Helper()
	_, _, result := scanPkg(t, dir)
	var ours []ScanIssue
	for _, i := range result.Issues {
		if i.Code == CodeShellStepNotPublishable {
			ours = append(ours, i)
		}
	}
	return ours
}

// writePublishablePkg writes the package and THEN stamps a manifest.id onto it.
//
// The two-step is load-bearing, and it cost a round of red tests to learn:
// `writePkg` assigns `files["skrptiq.yaml"]` UNCONDITIONALLY, so a manifest
// handed to it in the files map is silently discarded. Every fixture built that
// way is local-only — which this rule is deliberately silent about, so the
// refusal tests passed vacuously in the wrong direction (0 findings, reported as
// "not refused"). The manifest has to be written after the helper has had its
// say. `writePkg` itself is untouched: thirteen other call sites depend on it.
func writePublishablePkg(t *testing.T, execution string, extra map[string]string) string {
	t.Helper()
	dir := writePkg(t, invokedPkg(execution, extra))
	manifest := `name: shell-probe
slug: shell-probe
display_name: Shell Probe
description: "publish-bound probe for GH#1194"
author: test
category: productivity
version: 1.0.0
id: "7f1c2a4e-0000-4000-8000-00000000c0de"
`
	if err := os.WriteFile(filepath.Join(dir, "skrptiq.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("stamp manifest.id: %v", err)
	}
	return dir
}

// assertPublishBound fails if the fixture is not actually publish-bound. Without
// it, every refusal test in this file could pass for the wrong reason: a silent
// rule and an absent manifest.id are indistinguishable from the findings alone.
func assertPublishBound(t *testing.T, dir string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "skrptiq.yaml"))
	if err != nil {
		t.Fatalf("read fixture manifest: %v", err)
	}
	if !strings.Contains(string(raw), "id:") {
		t.Fatalf("fixture manifest carries no id — the rule is silent by design for "+
			"local-only packages, so this test would pass vacuously:\n%s", raw)
	}
}

// ── The refusal ───────────────────────────────────────────────────────────

// The headline: a publish-bound package with a raw shell step is refused, by
// name, at a severity that fails the gate — and the message says the content was
// permitted local-only, so the author meets the transition and not a bare "no".
func TestGH1194_PublishBoundShellStep_IsRefused(t *testing.T) {
	dir := writePublishablePkg(t, `  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
  - id: "build-check"
    step_type: "local.shell"
    context:
      run: "echo hello"
`, nil)
	assertPublishBound(t, dir)

	issues := shellIssues(t, dir)
	if len(issues) != 1 {
		t.Fatalf("want exactly 1 shell finding, got %d: %+v", len(issues), issues)
	}
	i := issues[0]
	if i.Severity != storage.SeverityError {
		t.Errorf("severity = %q, want error — a raw shell step must fail the gate", i.Severity)
	}
	if i.ReferencedSlug != "build-check" {
		t.Errorf("referencedSlug = %q, want build-check — the refusal must name the entry", i.ReferencedSlug)
	}
	if !strings.Contains(i.Message, "build-check") {
		t.Errorf("message does not name the entry: %q", i.Message)
	}
	if i.Field != "execution" {
		t.Errorf("field = %q, want execution", i.Field)
	}

	// The orchestrator's requirement, not a nicety: the error must explain that
	// this content was valid local-only and that declaring publish intent is
	// what changed. Silence before the transition is only defensible if the
	// transition itself is explained.
	for _, want := range []string{"local-only", "publish intent"} {
		if !strings.Contains(i.Message, want) {
			t.Errorf("message does not explain the transition (missing %q): %q", want, i.Message)
		}
	}
	// And it must say WHY, not merely that a rule exists.
	if !strings.Contains(i.Message, "sh -c") {
		t.Errorf("message does not give the reason (no `sh -c`): %q", i.Message)
	}

	_, code, _ := scanPkg(t, dir)
	if code != 2 {
		t.Errorf("scan exit = %d, want 2 — this must gate", code)
	}
}

// A shell step nested inside a `parallel:` group is refused exactly like a flat
// one. The walk must recurse — and nesting is the first thing an author would
// try if the flat form were refused.
func TestGH1194_ShellInsideAParallelGroup_IsRefused(t *testing.T) {
	dir := writePublishablePkg(t, `  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
  - parallel:
    - id: "nested-shell"
      step_type: "local.shell"
      context:
        run: "echo nested"
`, nil)
	assertPublishBound(t, dir)

	issues := shellIssues(t, dir)
	if len(issues) != 1 {
		t.Fatalf("a shell step inside a parallel group was not refused; got %+v", issues)
	}
	if issues[0].ReferencedSlug != "nested-shell" {
		t.Errorf("referencedSlug = %q, want nested-shell", issues[0].ReferencedSlug)
	}
}

// ── The negative direction, which is where this rule can do harm ──────────

// THE LOCAL-ONLY PATH, which R7 depends on: the same shell step, in a package
// with no manifest.id and no trust block, is NOT reported at all. Silence, not
// info — an info here would be the soft warning users learn to ignore, emitted
// most often to the author for whom it is least relevant.
func TestGH1194_LocalOnlyShellStep_IsSilent(t *testing.T) {
	execution := `  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
  - id: "build-check"
    step_type: "local.shell"
    context:
      run: "echo hello"
`
	// invokedPkg's manifest carries no id — that IS the local-only shape.
	dir := writePkg(t, invokedPkg(execution, nil))

	if issues := shellIssues(t, dir); len(issues) != 0 {
		t.Fatalf("a local-only package was reported — R7 permits author scripts locally: %+v", issues)
	}

	// And the whole scan must still pass: silence is worthless if some other
	// clause gates the same package.
	if _, code, _ := scanPkg(t, dir); code != 0 {
		t.Errorf("local-only scan exit = %d, want 0", code)
	}

	// The control that makes the silence meaningful rather than a dead rule:
	// the SAME execution block, published, is refused.
	pub := writePublishablePkg(t, execution, nil)
	assertPublishBound(t, pub)
	if issues := shellIssues(t, pub); len(issues) != 1 {
		t.Fatalf("the identical package WITH a manifest.id was not refused — the "+
			"discriminator is not doing anything: %+v", issues)
	}
}

// A publish-bound package whose steps are ordinary is clean. Without this, a
// rule that refused everything would be indistinguishable from this one.
func TestGH1194_PublishBoundWithoutShell_IsClean(t *testing.T) {
	dir := writePublishablePkg(t, `  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
`, nil)

	if issues := shellIssues(t, dir); len(issues) != 0 {
		t.Errorf("a publish-bound package with no shell step was reported: %+v", issues)
	}
}

// `context.run` is not the trigger. A `run` key on a non-shell step dispatches
// nothing — the engine's validator requires `run` FOR a shell step, not the
// reverse — so keying on `run` would refuse a step that never reaches a shell.
func TestGH1194_RunKeyWithoutShellStepType_IsNotReported(t *testing.T) {
	dir := writePublishablePkg(t, `  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "alpha-prompt"
  - id: "word-count-1"
    step_type: "local.builtin"
    context:
      run: "this is not a shell step"
      operation: "word_count"
`, nil)

	if issues := shellIssues(t, dir); len(issues) != 0 {
		t.Errorf("a local.builtin entry carrying a `run` key was refused as shell: %+v", issues)
	}
}

// ── The corpus floor: the two REAL packages the naive rule would refuse ────

// `skrpt-builder` and `skrptiq-on-skrptiq` both contain the string
// `local.shell` — in PROSE listing the valid step-type vocabulary. Neither
// executes a shell step, and `skrptiq-on-skrptiq` is the crossing demo's own
// home (R8). A text-level rule would refuse the two packages whose purpose is
// to document the step types.
//
// These are the real published packages, not synthetic ones: GH#858's false
// refusals were found by the corpus and nothing else would have caught them.
func TestGH1194_Corpus_PackagesDocumentingShellAreNotReported(t *testing.T) {
	for _, name := range []string{"skrpt-builder", "skrptiq-on-skrptiq"} {
		dir := filepath.Join("testdata", "gh1194-corpus", name)
		t.Run(name, func(t *testing.T) {
			if issues := shellIssues(t, dir); len(issues) != 0 {
				t.Errorf("published package %q was refused for documenting the step "+
					"types in prose: %+v", name, issues)
			}
		})
	}
}

// The walker, directly: authored order preserved, nesting followed, non-shell
// step types ignored, and an unlabelled entry still named rather than dropped.
func TestGH1194_ShellEntriesIn_FindsNestedAndLabelsEntries(t *testing.T) {
	execution := []any{
		map[string]any{"skill": "one", "step_type": "generation"},
		map[string]any{"id": "flat-shell", "step_type": "local.shell"},
		map[string]any{"parallel": []any{
			map[string]any{"id": "nested-shell", "step_type": "local.shell"},
			map[string]any{"id": "builtin", "step_type": "local.builtin"},
		}},
		map[string]any{"skill": "skill-keyed-shell", "step_type": "local.shell"},
		map[string]any{"step_type": "local.shell"},
	}

	got := shellEntriesIn(execution)
	want := []string{"flat-shell", "nested-shell", "skill-keyed-shell", "(unnamed local.shell entry)"}
	if len(got) != len(want) {
		t.Fatalf("got %d shell entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].label != want[i] {
			t.Errorf("entry %d label = %q, want %q", i, got[i].label, want[i])
		}
	}
}
