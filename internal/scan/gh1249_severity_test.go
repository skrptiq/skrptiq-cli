package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/skrptiq/engine/manifest"
	"github.com/skrptiq/engine/storage"
)

// GH#1249 — Rule 3 emitted `warning` while gating a publish, so the severity
// was decorative. It is now `error`, and label and effect agree.
//
// WHY THAT WAS SAFE, AND WHY IT DOES NOT GENERALISE.
//
// The population was measured before the change and was ZERO: no package in any
// corpus here produces a `workflow.input_declared_unused` finding, with or
// without dependency resolution. So no author's message changed — only the
// label. The three findings that once existed were the false positives closed by
// GH#1131 (`loops[].inputExpression`) and GH#1253 (references inside fences).
//
// The counts were then compared package-by-package against `origin/main`:
// 28 runs (14 packages x both resolution modes), and every `exit`, `errorCount`
// and `warnCount` was byte-identical before and after. That is the control the
// directive asked for, measured rather than asserted, and
// `TestGH1249_Control_...` below is its standing form.
//
// The promotion is Rule 3's ALONE. The scan has three other warning emitters and
// two of them point the OTHER way:
//
//   - Rule 1 runs a deliberate two-level ladder: `warning` when the name is
//     documented nowhere ("probably an example, not a contract"), `error` when
//     the `## Inputs` table shows the author meant it as a contract. That
//     distinction is real semantic work, and promoting it would erase it.
//   - `scan.cross_package_edge` says "will be resolved at runtime" — explicitly
//     benign. Pinned at warning by `TestScan_CrossPackageEdge_WarningNotError`.
//   - `parseToStorageSeverity` maps what a MANIFEST declared; promoting it would
//     override the author.
//
// That all three gate a publish anyway is hub's POLICY, not their labels — and
// it is why `skrpt-builder` and `skrptiq-on-skrptiq` cannot be published today
// for carrying literal `{{input.VAR}}` teaching placeholders, which is GH#1256
// and deliberately not addressed here.

// The headline: a declaration nothing consumes is an error, and the exit code
// says 2 rather than 1.
func TestGH1249_UnconsumedDeclaration_IsErrorAndGatesAtExit2(t *testing.T) {
	dir := writePkg(t, onePromptPkg(
		`inputs:
  orphan:
    label: "Orphan"
    required: true
    type: text
`,
		"| `{{input.orphan}}` | Yes | declared, documented, consumed by nothing | `x` |\n",
		"This prompt consumes no input at all.",
	))

	issues, code, result := scanPkg(t, dir)
	issue := findIssue(issues, CodeInputDeclaredUnused, "orphan")
	if issue == nil {
		t.Fatalf("no %s for orphan; got %+v", CodeInputDeclaredUnused, issues)
	}
	if issue.Severity != storage.SeverityError {
		t.Fatalf("severity = %q, want error", issue.Severity)
	}
	// Both halves matter. Before GH#1249 this finding produced exit 1 with
	// errorCount 0 — and hub's `scanner-conformance.mjs` keys on `engErr > 0`,
	// ignoring warnings entirely, so the finding was invisible to the only
	// reader that discriminates. The counts are what moved it into view.
	if code != 2 {
		t.Errorf("scan exit = %d, want 2", code)
	}
	if result.ErrorCount == 0 {
		t.Errorf("errorCount = 0 — the finding must land in the count "+
			"`scanner-conformance.mjs` reads; warnCount = %d", result.WarnCount)
	}
}

// THE CONTROL the directive named: a package that produces no Rule 3 finding is
// unaffected in BOTH counts. The numbers below are the measured baseline, so a
// later change that promotes anything else fails here rather than silently.
func TestGH1249_Control_NoRule3Finding_IsUnaffectedInBothCounts(t *testing.T) {
	t.Run("clean package", func(t *testing.T) {
		dir := writePkg(t, onePromptPkg(
			`inputs:
  topic:
    label: "Topic"
    required: true
    type: text
`,
			"| `{{input.topic}}` | Yes | the topic | `go` |\n",
			"Write about {{input.topic}}.",
		))
		_, code, result := scanPkg(t, dir)
		if code != 0 || result.ErrorCount != 0 || result.WarnCount != 0 {
			t.Errorf("clean package: exit=%d err=%d warn=%d, want 0/0/0",
				code, result.ErrorCount, result.WarnCount)
		}
	})

	// Real packages, and the ones with most at stake: both are blocked today by
	// Rule 1 warnings (GH#1256). Their counts must be exactly what they were
	// before the promotion — the error count especially, since a leak would
	// move them from "blocked by policy" to "structurally invalid".
	for _, tc := range []struct {
		pkg               string
		wantErr, wantWarn int
		wantExit          int
	}{
		{"skrpt-builder", 0, 1, 1},
		{"skrptiq-on-skrptiq", 0, 2, 1},
	} {
		t.Run(tc.pkg, func(t *testing.T) {
			dir := filepath.Join("testdata", "gh1194-corpus", tc.pkg)
			_, code, result := scanPkg(t, dir)
			if result.ErrorCount != tc.wantErr || result.WarnCount != tc.wantWarn || code != tc.wantExit {
				t.Errorf("exit=%d err=%d warn=%d, want %d/%d/%d",
					code, result.ErrorCount, result.WarnCount,
					tc.wantExit, tc.wantErr, tc.wantWarn)
			}
		})
	}
}

// The promotion must not have leaked. Rule 1's ladder keeps BOTH of its levels,
// and `parseToStorageSeverity` still honours what a manifest declared.
func TestGH1249_PromotionIsScopedToRule3(t *testing.T) {
	// Rule 1, warning half: documented nowhere, so it may be an example.
	dir := writePkg(t, onePromptPkg(
		"",
		"| `{{input.something_else}}` | Yes | unrelated | `x` |\n",
		"Every `{{input.VAR}}` in a prompt body needs a matching inputs entry.",
	))
	if i := findIssue(mustIssues(t, dir), CodeInputUndeclared, "VAR"); i == nil {
		t.Error("Rule 1 stopped firing on an undocumented undeclared input")
	} else if i.Severity != storage.SeverityWarning {
		t.Errorf("Rule 1 undocumented severity = %q, want warning — the ladder's "+
			"lower rung is the distinction Rule 1 exists to draw", i.Severity)
	}

	// Rule 1, error half: the `## Inputs` table is the author's evidence of
	// intent, so the same rule gates. Both rungs, or it is not a ladder.
	dir = writePkg(t, onePromptPkg(
		"",
		"| `{{input.log}}` | Yes | the raw log | `ERROR db timeout` |\n",
		"Summarise this log: {{input.log}}",
	))
	if i := findIssue(mustIssues(t, dir), CodeInputUndeclared, "log"); i == nil {
		t.Error("Rule 1 stopped firing on a documented undeclared input")
	} else if i.Severity != storage.SeverityError {
		t.Errorf("Rule 1 documented severity = %q, want error", i.Severity)
	}

	// A manifest-declared severity is still the manifest's to choose.
	if got := parseToStorageSeverity(manifest.SeverityWarn); got != storage.SeverityWarning {
		t.Errorf("parseToStorageSeverity(warn) = %q, want warning — promoting this "+
			"would override what an author declared", got)
	}
}

// The population guard. This is what makes the promotion a label change rather
// than a new refusal, and it is the assertion that fails first if that stops
// being true for any package committed here.
//
// Proven to reach: planting a `planted_orphan` declaration in
// `creator-profile-builder`'s workflow fails this test by name. ITS LIMIT, found
// the same way: Rule 3 runs per workflow, so a PROMPT-ONLY package (`take-notes`
// carries no `workflows/` directory) cannot produce a finding here no matter
// what it declares. The same plant in its prompt was inert. So this guards the
// 13 workflow-bearing corpus packages, not all 14.
func TestGH1249_CorpusRule3Population_IsZero(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("testdata", "*corpus*", "*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	scanned := 0
	for _, dir := range dirs {
		fi, err := os.Stat(dir)
		if err != nil || !fi.IsDir() {
			continue // the glob also matches each corpus README
		}
		scanned++
		for _, i := range unusedInputIssues(t, dir) {
			t.Errorf("%s produces a Rule 3 finding (%s) — the promotion in GH#1249 "+
				"was justified on a ZERO population; with one, it is a new refusal "+
				"and needs its own decision", filepath.Base(dir), i.Message)
		}
	}
	// Vacuity guard: "no package produces a finding" and "no package was
	// scanned" are the same result otherwise, and this assertion would pass
	// forever if the corpora moved. 14 at the time of writing.
	if scanned < 12 {
		t.Fatalf("only %d corpus packages scanned — the population claim is vacuous "+
			"below the corpora this repo actually carries", scanned)
	}
}

func mustIssues(t *testing.T, dir string) []ScanIssue {
	t.Helper()
	issues, _, _ := scanPkg(t, dir)
	return issues
}
