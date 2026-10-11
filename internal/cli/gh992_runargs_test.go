package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"
)

// GH#992 — `skrptiq run "Gated WF" --yes` silently dropped --yes, because Go's
// flag package stops parsing at the first non-flag argument. Exit 0 is what
// made that invisible, so every assertion here is on the PARSED OPTIONS: a test
// that only checked the exit code would have passed against the bug.

func parseOK(t *testing.T, args ...string) runOptions {
	t.Helper()
	opts, _, err := parseRunArgs(args)
	if err != nil {
		t.Fatalf("parseRunArgs(%q): unexpected error: %v", args, err)
	}
	return opts
}

// The headline regression: the three argument forms the issue names must parse
// identically, flags honoured wherever they sit.
func TestGH992_FlagOrderIsIrrelevant(t *testing.T) {
	want := runOptions{workflowName: "Gated WF", autoYes: true}

	forms := [][]string{
		{"--yes", "Gated WF"},
		{"Gated WF", "--yes"},
	}
	for _, args := range forms {
		got := parseOK(t, args...)
		if got.workflowName != want.workflowName || got.autoYes != want.autoYes {
			t.Errorf("parseRunArgs(%q) = {name:%q yes:%v}, want {name:%q yes:%v}",
				args, got.workflowName, got.autoYes, want.workflowName, want.autoYes)
		}
	}

	// And with a value-taking flag interleaved after the workflow name.
	got := parseOK(t, "Gated WF", "--input", "a=b", "--yes")
	if got.workflowName != "Gated WF" || !got.autoYes {
		t.Errorf("interleaved form: name=%q yes=%v", got.workflowName, got.autoYes)
	}
	if len(got.inputs) != 1 || got.inputs[0] != "a=b" {
		t.Errorf("interleaved form: inputs=%v, want [a=b]", got.inputs)
	}
}

// Every flag, placed AFTER the positional — the exact position that used to
// discard it. One case per flag so a future parser change cannot quietly drop
// one of them.
func TestGH992_EveryFlagHonouredAfterThePositional(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		check func(runOptions) error
	}{
		{"yes", []string{"WF", "--yes"}, func(o runOptions) error {
			if !o.autoYes {
				return errf("autoYes=false")
			}
			return nil
		}},
		{"strict", []string{"WF", "--strict"}, func(o runOptions) error {
			if !o.strict {
				return errf("strict=false")
			}
			return nil
		}},
		{"json", []string{"WF", "--json"}, func(o runOptions) error {
			if !o.jsonOut {
				return errf("jsonOut=false")
			}
			return nil
		}},
		{"output", []string{"WF", "--output", "/tmp/o.txt"}, func(o runOptions) error {
			if o.outputPath != "/tmp/o.txt" {
				return errf("outputPath=%q", o.outputPath)
			}
			return nil
		}},
		{"output=", []string{"WF", "--output=/tmp/o.txt"}, func(o runOptions) error {
			if o.outputPath != "/tmp/o.txt" {
				return errf("outputPath=%q", o.outputPath)
			}
			return nil
		}},
		{"gate-timeout", []string{"WF", "--gate-timeout", "30"}, func(o runOptions) error {
			if o.gateTimeout != 30 {
				return errf("gateTimeout=%d", o.gateTimeout)
			}
			return nil
		}},
		{"gate-timeout=", []string{"WF", "--gate-timeout=30"}, func(o runOptions) error {
			if o.gateTimeout != 30 {
				return errf("gateTimeout=%d", o.gateTimeout)
			}
			return nil
		}},
		{"input repeated", []string{"WF", "--input", "a=b", "--input", "c=d"}, func(o runOptions) error {
			if len(o.inputs) != 2 || o.inputs[0] != "a=b" || o.inputs[1] != "c=d" {
				return errf("inputs=%v", o.inputs)
			}
			return nil
		}},
		{"single dash form", []string{"WF", "-yes"}, func(o runOptions) error {
			if !o.autoYes {
				return errf("autoYes=false")
			}
			return nil
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := parseOK(t, tc.args...)
			if opts.workflowName != "WF" {
				t.Errorf("workflowName = %q, want WF", opts.workflowName)
			}
			if err := tc.check(opts); err != nil {
				t.Errorf("parseRunArgs(%q): %v", tc.args, err)
			}
		})
	}
}

// Accept-then-ignore is the bug class. Anything shaped like a flag that is not
// a known flag must fail loudly and NAME the flag — including after the
// positional, which is where the old parser swallowed it as a workflow word.
func TestGH992_UnknownFlagIsAHardError(t *testing.T) {
	cases := []struct {
		args     []string
		offender string
	}{
		{[]string{"--yess", "WF"}, "--yess"},
		{[]string{"WF", "--yess"}, "--yess"},
		{[]string{"WF", "-x"}, "-x"},
		{[]string{"WF", "--gate-timout=30"}, "--gate-timout"}, // plausible typo of a real flag
		{[]string{"--json", "WF", "--dry-run"}, "--dry-run"},
	}
	for _, tc := range cases {
		opts, _, err := parseRunArgs(tc.args)
		if err == nil {
			t.Errorf("parseRunArgs(%q) = %+v, want an error", tc.args, opts)
			continue
		}
		if !strings.Contains(err.Error(), "unknown flag") {
			t.Errorf("parseRunArgs(%q) error = %q, want it to say 'unknown flag'", tc.args, err)
		}
		// The message has to name the offending argument, or the user cannot
		// tell which of several flags was rejected.
		if !strings.Contains(err.Error(), tc.offender) {
			t.Errorf("parseRunArgs(%q) error = %q, want it to name %q", tc.args, err, tc.offender)
		}
	}
}

// A value-taking flag with nothing after it must not silently swallow the end
// of the line.
func TestGH992_ValueFlagWithoutValueIsAnError(t *testing.T) {
	for _, args := range [][]string{
		{"WF", "--input"},
		{"WF", "--output"},
		{"WF", "--gate-timeout"},
	} {
		if _, _, err := parseRunArgs(args); err == nil {
			t.Errorf("parseRunArgs(%q): want an error, got none", args)
		} else if !strings.Contains(err.Error(), args[len(args)-1]) {
			t.Errorf("parseRunArgs(%q) error = %q, want it to name the flag", args, err)
		}
	}
}

// A bad value for a typed flag still has to fail — the pre-pass must not
// accidentally make `--gate-timeout abc` acceptable.
func TestGH992_BadFlagValueStillFails(t *testing.T) {
	if _, _, err := parseRunArgs([]string{"WF", "--gate-timeout", "abc"}); err == nil {
		t.Fatal("--gate-timeout abc: want a parse error, got none")
	}
}

// Deliberate departure from the issue's third acceptance criterion, which asked
// for ">1 positional is an error, not last-one-wins". The parser never did
// last-one-wins — it JOINS positionals with a space, which is how an unquoted
// multi-word title resolves. Every positional is used, so nothing is accepted
// and then discarded, and erroring here would break `skrptiq run Weekly Report`.
func TestGH992_MultiplePositionalsJoinRatherThanDrop(t *testing.T) {
	opts := parseOK(t, "Weekly", "Report", "--yes")
	if opts.workflowName != "Weekly Report" {
		t.Errorf("workflowName = %q, want %q", opts.workflowName, "Weekly Report")
	}
	if !opts.autoYes {
		t.Error("autoYes = false — a flag after several positionals was dropped")
	}

	// A stray extra word therefore fails at lookup with the whole string in the
	// message, rather than being silently ignored.
	opts = parseOK(t, "Gated WF", "stray")
	if opts.workflowName != "Gated WF stray" {
		t.Errorf("workflowName = %q, want the stray word kept so lookup fails loudly", opts.workflowName)
	}
}

// `--` ends flag parsing, so a workflow title that begins with a dash is still
// reachable.
func TestGH992_DoubleDashTerminatesFlags(t *testing.T) {
	opts := parseOK(t, "--yes", "--", "--odd-title")
	if opts.workflowName != "--odd-title" {
		t.Errorf("workflowName = %q, want --odd-title", opts.workflowName)
	}
	if !opts.autoYes {
		t.Error("autoYes = false, want the flag before -- to be honoured")
	}
}

// No arguments at all leaves an empty name for Run to reject with usage.
func TestGH992_NoPositionalLeavesNameEmpty(t *testing.T) {
	opts := parseOK(t, "--yes")
	if opts.workflowName != "" {
		t.Errorf("workflowName = %q, want empty", opts.workflowName)
	}
}

// --help stays the flag package's own concern, so `run --help` keeps printing
// usage rather than being rejected as an unknown flag.
func TestGH992_HelpIsNotAnUnknownFlag(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"WF", "--help"}} {
		_, _, err := parseRunArgs(args)
		if err == nil {
			t.Errorf("parseRunArgs(%q): want flag.ErrHelp, got none", args)
			continue
		}
		if !errors.Is(err, flag.ErrHelp) {
			t.Errorf("parseRunArgs(%q) error = %v, want flag.ErrHelp", args, err)
		}
	}
}

// errf keeps the table above reading as assertions rather than t.Errorf calls.
func errf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
