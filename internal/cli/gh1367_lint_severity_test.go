package cli

import (
	"strings"
	"testing"

	"github.com/skrptiq/engine/manifest"
)

// GH#1367 — GH#1358's defect one command over. `lint`'s counter had no `default`
// while the loop above it prints every issue and `severityLabel` renders any
// severity's raw string, so an issue carrying a third value was printed, counted
// toward nothing, and `lint` returned `ExitOK`.
//
// Found by sweeping for the shape after fixing it in the scan rather than by
// anyone reporting it, which is the only reason it was looked for at all.

func parseIssue(code string, sev manifest.Severity) manifest.ParseIssue {
	return manifest.ParseIssue{Code: manifest.IssueCode(code), Severity: sev, Message: "m"}
}

// THE PLANT. `"blocker"` is fabricated: this estate's manifest vocabulary is
// error/warn/info. A planted value that could occur naturally would make the test
// a coincidence rather than a control — it could not distinguish "the counter
// caught it" from "the data happened to contain it."
func TestGH1367_UnrecognisedSeverity_IsRefusedAndNamed(t *testing.T) {
	const planted = manifest.Severity("blocker")

	unrecognised, errors, warnings := lintTally([]manifest.ParseIssue{
		parseIssue("some.rule", planted),
		parseIssue("another.rule", planted),
	})

	if errors == 0 {
		t.Fatalf("errors = 0 — two findings carrying %q counted toward nothing, so lint "+
			"would print them and exit %d", planted, ExitOK)
	}
	if warnings != 0 {
		t.Errorf("warnings = %d, want 0 — an unrecognised value must not be filed under a "+
			"known one", warnings)
	}
	if len(unrecognised) != 1 {
		t.Fatalf("got %d messages, want 1 — one per distinct unrecognised severity, not one "+
			"per issue", len(unrecognised))
	}

	msg := unrecognised[0]
	for _, want := range []string{string(planted), "2 finding"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the message does not contain %q: %q", want, msg)
		}
	}
	if !strings.Contains(msg, "some.rule") && !strings.Contains(msg, "another.rule") {
		t.Errorf("the message names no example rule: %q", msg)
	}
	if got := lintExitCode(errors, warnings); got == ExitOK {
		t.Errorf("lintExitCode = %d (ExitOK) with an unrecognised severity present", got)
	}
}

// THE KNOWN-GOOD CONTROL, and the half that decides whether the fix survives: a
// counter treating everything as unrecognised passes the plant perfectly and
// breaks every clean lint.
func TestGH1367_KnownSeverities_KeepTheirExistingCounts(t *testing.T) {
	unrecognised, errors, warnings := lintTally([]manifest.ParseIssue{
		parseIssue("a", manifest.SeverityError),
		parseIssue("b", manifest.SeverityWarn),
		parseIssue("c", manifest.SeverityWarn),
		parseIssue("d", manifest.SeverityInfo),
	})

	if errors != 1 || warnings != 2 {
		t.Errorf("errors=%d warnings=%d, want 1/2", errors, warnings)
	}
	if len(unrecognised) != 0 {
		t.Errorf("got %d unrecognised messages on known severities, want 0", len(unrecognised))
	}
}

// `info` is uncounted ON PURPOSE — that is a decision about a KNOWN value, which
// is what distinguishes it from the gap this issue is about. An info-only lint
// stays clean.
func TestGH1367_InfoOnly_IsStillClean(t *testing.T) {
	unrecognised, errors, warnings := lintTally([]manifest.ParseIssue{
		parseIssue("a", manifest.SeverityInfo),
		parseIssue("b", manifest.SeverityInfo),
	})
	if errors != 0 || warnings != 0 || len(unrecognised) != 0 {
		t.Errorf("e=%d w=%d unrecognised=%d on an info-only lint, want 0/0/0",
			errors, warnings, len(unrecognised))
	}
	if got := lintExitCode(errors, warnings); got != ExitOK {
		t.Errorf("lintExitCode = %d, want ExitOK for info only", got)
	}
}

// The exit mapping, asserted rather than assumed.
//
// NOTE `ExitFailed` IS 1 AND SO IS THE WARNING CODE: lint's exit vocabulary is
// two-valued, unlike the scan's 2/1/0, so an error and a warning are
// indistinguishable from outside and the only observable fail-open was 0 instead
// of 1. Asserted here explicitly so nobody reads a passing test as evidence that
// the two are distinguished.
func TestGH1367_LintExitCodeMapping(t *testing.T) {
	for _, c := range []struct {
		name             string
		errors, warnings int
		want             int
	}{
		{"an error refuses", 1, 0, ExitFailed},
		{"a warning refuses", 0, 1, 1},
		{"nothing is clean", 0, 0, ExitOK},
		{"an error outranks a warning", 1, 9, ExitFailed},
	} {
		if got := lintExitCode(c.errors, c.warnings); got != c.want {
			t.Errorf("%s: lintExitCode(%d,%d) = %d, want %d", c.name, c.errors, c.warnings, got, c.want)
		}
	}
	if ExitFailed != 1 {
		t.Errorf("ExitFailed = %d; this test's premise is that it equals the warning code 1", ExitFailed)
	}
}
