package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// GH#992 — Go's `flag` package stops parsing at the first non-flag argument, so
// everything after the workflow name was taken as a positional and joined into
// the name. `skrptiq run "Gated WF" --yes` therefore looked up a workflow
// titled `Gated WF --yes`, which no exact-match lookup could find:
//
//	Workflow not found: Gated WF --yes      (exit 2, nothing started)
//
// The flag was swallowed into the workflow name, so the command failed with a
// confusing "not found" instead of doing what was asked. It did not run
// unattended and it did not exit 0 — the cost was a misleading diagnostic, and
// a misspelled flag (`--dry-run`, `--gate-timout=30`) was reported as a missing
// workflow rather than as a bad flag.
//
// The fix is a pre-pass that separates flags from positionals before the flag
// set sees them, so a flag is honored wherever it appears — and anything that
// looks like a flag but is not one is a hard error naming it, never a silently
// swallowed positional.

// runFlagSpec is every flag `run` accepts, mapped to whether it consumes a
// following value. Booleans do not: Go's flag package only accepts `-json=true`,
// never `-json true`, so consuming the next token for a bool would eat the
// workflow name.
var runFlagSpec = map[string]bool{
	"input":        true,
	"output":       true,
	"json":         false,
	"yes":          false,
	"strict":       false,
	"gate-timeout": true,
}

// runOptions is the parsed form of `skrptiq run`'s arguments. Tests assert on
// this rather than on an exit code, because the exit code cannot tell the two
// cases apart: a swallowed flag and a genuinely missing workflow both exited 2
// with a "not found" message, so an exit-code assertion passes whether or not
// the flag was ever seen. The parsed options are the only evidence that it was.
type runOptions struct {
	workflowName string
	inputs       []string
	outputPath   string
	jsonOut      bool
	autoYes      bool
	strict       bool
	gateTimeout  int
}

// splitRunArgs separates flag arguments from positional ones, permitting flags
// interspersed with the workflow name in any order. A bare `--` ends flag
// parsing, so a workflow name that genuinely begins with a dash stays reachable.
func splitRunArgs(args []string) (flagArgs, positionals []string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			return flagArgs, positionals, nil
		}

		// "-" is a conventional stdin placeholder, not a flag.
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}

		name := strings.TrimLeft(arg, "-")

		// Help belongs to the flag package; hand it straight through.
		if name == "h" || name == "help" {
			flagArgs = append(flagArgs, arg)
			continue
		}

		bare, inlineValue := name, false
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			bare, inlineValue = name[:eq], true
		}

		takesValue, known := runFlagSpec[bare]
		if !known {
			return nil, nil, fmt.Errorf("unknown flag: %s", arg)
		}

		flagArgs = append(flagArgs, arg)
		if takesValue && !inlineValue {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("flag needs an argument: %s", arg)
			}
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	return flagArgs, positionals, nil
}

// parseRunArgs parses `run`'s arguments into runOptions. Flags are honored in
// any position; an unrecognized flag is an error. Multiple positionals are
// joined with a single space, which is how an unquoted multi-word workflow
// title (`skrptiq run Weekly Report`) resolves — every positional is used, so
// nothing is accepted and then discarded.
//
// The returned flag.FlagSet is only for rendering usage on a parse failure.
func parseRunArgs(args []string) (runOptions, *flag.FlagSet, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	// Errors are reported by the caller so every failure reads the same way.
	fs.SetOutput(io.Discard)

	var inputs inputFlag
	fs.Var(&inputs, "input", "Input variable (key=value, use value=- for stdin)")
	outputPath := fs.String("output", "", "Write final output to file")
	jsonOut := fs.Bool("json", false, "Output result as JSON")
	autoYes := fs.Bool("yes", false, "Auto-approve all gates")
	strict := fs.Bool("strict", false, "Fail on any gate step")
	gateTimeout := fs.Int("gate-timeout", 0, "Gate timeout in seconds (0 = no timeout)")

	flagArgs, positionals, err := splitRunArgs(args)
	if err != nil {
		return runOptions{}, fs, err
	}

	if err := fs.Parse(flagArgs); err != nil {
		return runOptions{}, fs, err
	}

	return runOptions{
		workflowName: strings.Join(positionals, " "),
		inputs:       inputs,
		outputPath:   *outputPath,
		jsonOut:      *jsonOut,
		autoYes:      *autoYes,
		strict:       *strict,
		gateTimeout:  *gateTimeout,
	}, fs, nil
}
