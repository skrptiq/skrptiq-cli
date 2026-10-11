package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// GH#992 — Go's `flag` package stops parsing at the first non-flag argument, so
// `skrptiq run "Gated WF" --yes` discarded `--yes` as a positional: no error, no
// warning, and on a gated skrpt an unexpected interactive prompt or a blocked
// non-interactive run. The same accept-then-ignore shape as GH#968's exit 0.
//
// The fix is a pre-pass that separates flags from positionals before the flag
// set sees them, so a flag is honoured wherever it appears — and anything that
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
// this rather than on an exit code: exit 0 is precisely what made GH#992
// invisible, so an exit-code assertion would have passed against the bug.
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

// parseRunArgs parses `run`'s arguments into runOptions. Flags are honoured in
// any position; an unrecognised flag is an error. Multiple positionals are
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
