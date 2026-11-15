package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/outputs"
	"github.com/skrptiq/engine/storage"

	"github.com/charmbracelet/x/term"
)

// GH#1221 — the CLI never asked anyone anything, because it never declared that
// it could. `exec.RunOptions.Interactive` is caller-declared and its zero value
// is non-interactive by design, so a `file` destination with no path was refused
// even with a person sitting at the terminal able to answer.
//
// THE PART THAT IS NOT THE PROMPT. `PerformDestinations` branches on
// `interactive` TWICE. The second branch is `clipboard`:
//
//	clipboard + !interactive  ->  refusal, naming the type and where the output is
//	clipboard +  interactive  ->  DestinationResult{HostPending: true}
//
// and `RunOptions.OnDestinations` is nil in the CLI, which the engine's own
// comment calls "correct for the CLI" ON THE ASSUMPTION the CLI is
// non-interactive. So declaring Interactive without a handler would turn a
// working refusal into a pending item nobody completes — which is exactly the
// silence GH#1203 was built to remove, on the path whose refusal cites GH#1202
// T0-14 as its reason. Interactive and the handler are one decision.
//
// WHAT THE HANDLER MAY AND MAY NOT DO. It completes a `file` ask by prompting and
// handing the answer back as `ChosenPath` — THE ENGINE STILL WRITES
// (`outputs.Write`), because moving the write inside that boundary is the point
// of GH#1203 and a second writer here would be a second authority for a rule the
// engine owns. `clipboard` is REFUSED, reworded for a terminal: copying is an OS
// capability this binary does not have today (`atotto/clipboard` is present but
// indirect and imported by nothing), so implementing it would be a new
// capability and leaving it pending would be a regression. Refusing keeps
// today's outcome on that path while the file path gains the ask.

// interactiveRun reports whether a person can be asked, for the run about to
// start.
//
// Stdin, because stdin is who answers — not stdout, which is the output channel
// and is legitimately a pipe during an interactive run (`skrptiq run x > out`).
//
// THE OVERRIDE IS EVALUATED LAST, ON PURPOSE: a CI job with a pty allocated is
// still nobody to ask, so `--non-interactive` must beat detection rather than
// merely agree with it. Nothing here reads an environment variable: detection
// plus one explicit flag is auditable, and an env-var allowlist is a second
// authority that rots.
func interactiveRun(nonInteractive bool) bool {
	if nonInteractive {
		return false
	}
	return term.IsTerminal(os.Stdin.Fd())
}

// cliDestinationHandler resolves every host-pending destination the engine hands
// back. Returns a handler suitable for RunConfig.OnDestinations.
//
// db is needed to rebuild the `outputs.Request` for the completion. That rebuild
// duplicates `execution.buildRequest`, which is itself already duplicated by
// `engine-lib/outputs.go: buildOutputRequest` — two byte-equivalent copies before
// this one. The constants are exported so this works, but THREE copies of one
// derivation is the drift K-034 names; the engine should export a single builder
// and all three hosts should call it. Flagged rather than silently triplicated.
func cliDestinationHandler(db *storage.DB) func([]exec.DestinationResult) {
	return func(results []exec.DestinationResult) {
		for _, r := range results {
			if !r.HostPending {
				// Already resolved by the engine, success or refusal. The run
				// record carries it; nothing to add.
				continue
			}
			switch r.Type {
			case "file":
				completeFileDestination(db, r)
			case "clipboard":
				fmt.Fprintf(os.Stderr,
					"  ⊘ clipboard destination not available: this terminal cannot copy for you. "+
						"Use a file destination, or copy from the run's output above — "+
						"the run's output is kept in its history either way.\n")
			default:
				// A pending type this build does not know. Say so rather than
				// drop it: an unhandled pending result is the silence this whole
				// change exists to prevent, and a new type must not inherit it.
				fmt.Fprintf(os.Stderr,
					"  ⊘ destination type %q needs something this terminal cannot do, "+
						"and this build does not know how to complete it. "+
						"The run's output is kept in its history.\n", r.Type)
			}
		}
	}
}

// completeFileDestination asks for a path and lets the engine write there.
func completeFileDestination(db *storage.DB, r exec.DestinationResult) {
	req, err := outputRequestFor(db, r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ⊘ file destination: %v\n", err)
		return
	}

	// Resolve again to learn WHICH of the two asks this is. The engine
	// distinguishes them (GH#1209) and so must the prompt: an unset output
	// directory wants a full path, a destination with no `filePath` wants a name
	// inside a directory that already exists. Collapsing them into one question
	// would teach the reader something untrue.
	res, rerr := outputs.Resolve(req)
	if rerr != nil {
		// The engine's refusal IS the message — it names the setting, the
		// directory and that the output is still in history. Never reworded here.
		fmt.Fprintf(os.Stderr, "  ⊘ %v\n", rerr)
		return
	}
	if !res.NeedsAsk {
		// It resolved without asking, so there was nothing to complete. Should
		// not happen for a HostPending result; reported rather than ignored.
		fmt.Fprintf(os.Stderr,
			"  ⊘ file destination came back pending but resolved to %q without asking — "+
				"nothing was written. The run's output is kept in its history.\n", res.Path)
		return
	}

	answer, ok := askForPath(res.Directory)
	if !ok {
		return
	}
	req.ChosenPath = answer

	written, werr := outputs.Write(req)
	if werr != nil {
		fmt.Fprintf(os.Stderr, "  ⊘ file destination: %v\n", werr)
		return
	}
	fmt.Fprintf(os.Stderr, "  ✓ output written to %s\n", written)
}

// askForPath prompts and returns the answer, or ok=false when nobody answered.
//
// EOF, interrupt and an empty line are DEFINED here rather than left to the
// terminal:
//
//   - EOF (Ctrl-D, or stdin closing): the same outcome as nobody being there. The
//     person declined to name a file, so this is the non-interactive refusal, not
//     an empty path and not a guessed name.
//   - Empty line: re-prompted ONCE, then treated as EOF. An empty answer is a
//     slip, not a decision, and a silent accept here writes a file nobody named.
//   - Interrupt (Ctrl-C): the process dies, and that is survivable because
//     destinations route AFTER the run — the execution row is already `completed`
//     and its results already persisted. Every message on this path says so, so
//     an interrupted or refused ask never reads as lost work.
func askForPath(dir string) (string, bool) {
	reader := bufio.NewReader(os.Stdin)

	for attempt := 0; attempt < 2; attempt++ {
		if dir == "" {
			fmt.Fprintf(os.Stderr,
				"  ⏸ This run has a file destination but no output directory is configured.\n"+
					"    Enter a full path to write it to (or press Ctrl-D to skip): ")
		} else {
			fmt.Fprintf(os.Stderr,
				"  ⏸ This run's file destination declares no filename.\n"+
					"    Enter a name relative to %s (or press Ctrl-D to skip): ", dir)
		}

		line, err := reader.ReadString('\n')
		answer := strings.TrimSpace(line)
		if answer != "" {
			return answer, true
		}
		if err != nil {
			break // EOF or a read error: nobody is answering.
		}
		// Empty line with more input available — one more chance.
	}

	fmt.Fprintln(os.Stderr,
		"  ⊘ no destination given, so nothing was written. "+
			"The run's output is kept in its history.")
	return "", false
}

// outputRequestFor rebuilds the engine's own request shape for a completion. See
// the note on cliDestinationHandler about this being a third copy.
func outputRequestFor(db *storage.DB, r exec.DestinationResult) (outputs.Request, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return outputs.Request{}, fmt.Errorf(
			"this account has no resolvable home directory, so the output directory "+
				"setting cannot be expanded: %w", err)
	}
	return outputs.Request{
		Setting:      db.GetSetting(outputs.SettingOutputDir),
		Collision:    db.GetSetting(outputs.SettingOutputCollision),
		Home:         home,
		Workspace:    db.GetSetting("WORKSPACE_PATH"),
		DeclaredPath: r.DeclaredPath,
		Interactive:  true,
		Content:      r.Content,
	}, nil
}
