package scan

import (
	"bytes"
	"strings"
	"testing"

	"github.com/skrptiq/engine/storage"
)

// GH#1358 — the severity counter had no `default`, so an issue carrying a fourth
// value was printed by `OutputTable`, counted toward no verdict, and the scan
// exited 0. **A finding on stdout with a success status is indistinguishable from
// a clean scan** to anything that checks the status, and hub's publish path checks
// the status.
//
// THE PLANTED VALUE IS THE POINT. `"critical"` is not a severity this estate uses;
// it is fabricated here precisely so the test fails against the old code. Run
// `TestGH1358_UnrecognisedSeverity_IsRefusedAndNamed` against the parent of this
// commit and it reports exit 0 with the finding present.

func issue(code string, sev storage.ValidationSeverity) ScanIssue {
	return ScanIssue{ValidationIssue: storage.ValidationIssue{Code: code, Severity: sev}}
}

// DIRECTION 1 — the fail-open is closed, and the severity is NAMED rather than
// silently promoted.
func TestGH1358_UnrecognisedSeverity_IsRefusedAndNamed(t *testing.T) {
	const planted = storage.ValidationSeverity("critical")

	extra, errs, warns, infos := tallySeverities([]ScanIssue{
		issue("some.rule", planted),
		issue("another.rule", planted),
	})

	if errs == 0 {
		t.Fatalf("errors = 0 — two findings carrying %q counted toward nothing, so the scan "+
			"would print them and exit 0", planted)
	}
	if warns != 0 || infos != 0 {
		t.Errorf("warnings=%d infos=%d, want 0/0 — an unrecognised value must not be "+
			"silently filed under a known one", warns, infos)
	}
	if len(extra) != 1 {
		t.Fatalf("got %d synthetic findings, want 1 — one per distinct unrecognised "+
			"severity, not one per issue", len(extra))
	}

	got := extra[0]
	if got.Code != CodeUnrecognisedSeverity {
		t.Errorf("code = %q, want %q", got.Code, CodeUnrecognisedSeverity)
	}
	if got.Severity != storage.SeverityError {
		t.Errorf("severity = %q, want error — the verdict must come from the ordinary "+
			"path, not a special case", got.Severity)
	}
	if !strings.Contains(got.Message, string(planted)) {
		t.Errorf("message does not name the unrecognised severity: %q", got.Message)
	}
	if !strings.Contains(got.Message, "2 finding") {
		t.Errorf("message does not say how many findings carried it: %q", got.Message)
	}
	if !strings.Contains(got.Message, "some.rule") && !strings.Contains(got.Message, "another.rule") {
		t.Errorf("message names no example rule: %q", got.Message)
	}
}

// DIRECTION 2 — the known-good control, and the half that decides whether the fix
// survives. A tally that reports everything as unrecognised passes direction 1
// perfectly and breaks every clean scan.
func TestGH1358_KnownSeverities_KeepTheirExistingCounts(t *testing.T) {
	extra, errs, warns, infos := tallySeverities([]ScanIssue{
		issue("a", storage.SeverityError),
		issue("b", storage.SeverityWarning),
		issue("c", storage.SeverityWarning),
		issue("d", storage.SeverityInfo),
	})

	if errs != 1 || warns != 2 || infos != 1 {
		t.Errorf("errors=%d warnings=%d infos=%d, want 1/2/1", errs, warns, infos)
	}
	if len(extra) != 0 {
		t.Errorf("got %d synthetic findings on a tally of known severities, want 0", len(extra))
	}
}

// An empty scan stays clean. The `default` branch must not fire on nothing.
func TestGH1358_NoIssues_IsStillClean(t *testing.T) {
	extra, errs, warns, infos := tallySeverities(nil)
	if len(extra) != 0 || errs != 0 || warns != 0 || infos != 0 {
		t.Errorf("got extra=%d e=%d w=%d i=%d on an empty scan, want all zero",
			len(extra), errs, warns, infos)
	}
}

// The exit codes themselves, asserted rather than assumed — the mapping hub's
// publish path depends on.
func TestGH1358_ExitCodeMapping(t *testing.T) {
	for _, c := range []struct {
		name   string
		result ScanResult
		want   int
	}{
		{"an error refuses", ScanResult{ErrorCount: 1}, 2},
		{"a warning refuses", ScanResult{WarnCount: 1}, 1},
		{"info alone is clean", ScanResult{InfoCount: 3}, 0},
		{"nothing is clean", ScanResult{}, 0},
		{"an error outranks a warning", ScanResult{ErrorCount: 1, WarnCount: 9}, 2},
	} {
		if got := exitCodeFor(c.result); got != c.want {
			t.Errorf("%s: exitCodeFor = %d, want %d", c.name, got, c.want)
		}
	}
}

// `--json` and the table share one tally, so they cannot disagree. This asserts
// the property rather than the implementation: both printers are handed the SAME
// ScanResult, so there is no second map to drift (GH#1352's one-map rule).
func TestGH1358_JSONAndTableReadTheSameCounts(t *testing.T) {
	result := ScanResult{
		Path:       "/x",
		Issues:     []ScanIssue{issue("a", storage.SeverityError), issue("b", storage.SeverityWarning)},
		ErrorCount: 1,
		WarnCount:  1,
	}

	var js, tbl bytes.Buffer
	OutputJSON(result, &js)
	OutputTable(result, &tbl)

	if !strings.Contains(js.String(), `"errorCount": 1`) || !strings.Contains(js.String(), `"warnCount": 1`) {
		t.Errorf("JSON does not carry the tally's counts: %s", js.String())
	}
	if !strings.Contains(tbl.String(), "1 error(s)") || !strings.Contains(tbl.String(), "1 warning(s)") {
		t.Errorf("table does not carry the tally's counts: %s", tbl.String())
	}
}
