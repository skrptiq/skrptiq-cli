package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	exec "github.com/skrptiq/engine/execution"
	eng "github.com/skrptiq/skrptiq-cli/internal/engine"
	"github.com/skrptiq/skrptiq-cli/internal/toolobject"
)

// runUsage is the one usage line every `run` argument failure prints, so a bad
// invocation always says the same thing.
const runUsage = "Usage: skrptiq run [--input key=value] [--output path] [--json] [--yes|--strict] [--gate-timeout N] <workflow>"

// inputFlag collects multiple --input k=v flags.
type inputFlag []string

func (f *inputFlag) String() string { return strings.Join(*f, ", ") }
func (f *inputFlag) Set(val string) error {
	*f = append(*f, val)
	return nil
}

// Run handles `skrptiq run <workflow> [--input k=v] [--output path] [--json] [--yes|--strict] [--gate-timeout N]`.
func Run(args []string, dbPath string) int {
	// GH#992 — parseRunArgs honors flags in any position and rejects an
	// unknown flag outright, rather than letting Go's flag package stop at the
	// workflow name and discard everything after it.
	opts, fs, err := parseRunArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, runUsage)
			fs.SetOutput(os.Stderr)
			fs.PrintDefaults()
			return ExitBadArgs
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, runUsage)
		return ExitBadArgs
	}

	if opts.workflowName == "" {
		fmt.Fprintln(os.Stderr, runUsage)
		return ExitBadArgs
	}
	workflowName := opts.workflowName

	// Parse inputs.
	inputMap, err := parseInputs(opts.inputs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitBadArgs
	}

	engine := OpenEngine(dbPath)
	if engine == nil {
		return ExitFailed
	}
	defer engine.Close()

	// Find workflow.
	node, err := engine.FindNodeByTitle(workflowName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}
	if node == nil || node.Type != "workflow" {
		fmt.Fprintf(os.Stderr, "Workflow not found: %s\n", workflowName)
		return ExitBadArgs
	}

	// Build plan to check for required inputs.
	plan, err := engine.BuildPlan(node.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Plan error: %v\n", err)
		return ExitFailed
	}

	// Check all required inputs are provided.
	for _, varName := range plan.InputVariables {
		if _, ok := inputMap[varName]; !ok {
			fmt.Fprintf(os.Stderr, "Missing required input: %s\n", varName)
			fmt.Fprintf(os.Stderr, "  Use: --input %s=<value>\n", varName)
			return ExitBadArgs
		}
	}

	// Execute.
	fmt.Fprintf(os.Stderr, "Running: %s\n", plan.WorkflowTitle)
	ctx := context.Background()
	var lastOutput string
	var executionID string
	gateMode := gateDefault
	if opts.autoYes {
		gateMode = gateAutoApprove
	}
	if opts.strict {
		gateMode = gateStrict
	}

	// GH#873 — map builtin steps by position so the live timeline renders them as
	// distinct, read-only objects rather than plain node rows.
	builtins := toolobject.NewRegistry().ByPosition(plan)

	result := runWorkflow(ctx, engine, node.ID, inputMap, gateMode, opts.gateTimeout, &lastOutput, &executionID, builtins)

	// Output.
	if opts.jsonOut {
		outputJSON(os.Stdout, map[string]any{
			"workflow":    plan.WorkflowTitle,
			"executionId": executionID,
			"status":      result.status,
			"output":      lastOutput,
			"error":       result.err,
		})
	} else if opts.outputPath != "" {
		if err := os.WriteFile(opts.outputPath, []byte(lastOutput), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			return ExitFailed
		}
		fmt.Fprintf(os.Stderr, "Output written to %s\n", opts.outputPath)
	} else if lastOutput != "" {
		fmt.Print(lastOutput)
		if !strings.HasSuffix(lastOutput, "\n") {
			fmt.Println()
		}
	}

	return result.exitCode
}

type gateHandling int

const (
	gateDefault gateHandling = iota
	gateAutoApprove
	gateStrict
)

type runResult struct {
	status   string
	err      string
	exitCode int
}

// gateDecision is handleGate's verdict on a pause. handleGate renders the gate
// and reads the decision; it does NOT call the engine. GH#968 — resuming from
// inside the runner's progress callback is the bug: the callback fires before
// the runner writes status='paused', and ResumeExecution refuses anything that
// is not already paused ("execution is not paused (status: running)").
type gateDecision struct {
	approved bool
	input    string
	// reason is the stderr line explaining a decline; empty when approved.
	reason string
	// timedOut distinguishes "the human never answered" from "the human said no",
	// so automation can match the two apart (ExitTimeout vs ExitFailed).
	timedOut bool
}

func runWorkflow(ctx context.Context, engine *eng.App, nodeID string, inputs map[string]string, gate gateHandling, gateTimeoutSec int, lastOutput *string, execID *string, builtins map[int]toolobject.ToolObject) runResult {
	var finalErr string

	// pendingGate holds the gate event the engine paused on, for the resume loop
	// below to act on once the engine call has returned.
	var pendingGate *exec.ProgressEvent

	// titleByPos remembers the title the planner gave each position, and
	// gatePositions which of them are gates. Resume's events for the approved
	// gate step carry no title (and its decline event carries the node ID), so
	// without this the resumed leg renders a blank or an opaque label.
	titleByPos := make(map[int]string)
	gatePositions := make(map[int]bool)

	// stepLabel is the builtin object's label for a builtin step, else the node
	// title — falling back to the remembered title only when the event has none,
	// so a parallel group's siblings can never borrow each other's label.
	stepLabel := func(evt exec.ProgressEvent) string {
		if obj, ok := builtins[evt.Position]; ok {
			return obj.Label()
		}
		if evt.NodeTitle != "" {
			return evt.NodeTitle
		}
		if t, ok := titleByPos[evt.Position]; ok {
			return t
		}
		return fmt.Sprintf("step %d", evt.Position)
	}

	// chunked collects the streaming output of the step currently running, so a
	// non-streaming step's output can be taken from step-completed WITHOUT
	// double-counting a streaming step that already delivered the same text as
	// chunks. Deterministic steps (local.*, cache hits) emit no chunks at all —
	// their output arrives only on step-completed, and ignoring it is why a
	// template-tailed workflow printed nothing.
	var chunked strings.Builder

	// onProgress is the ONE renderer, shared by the run and every resume below.
	// Threading the same handler is what makes post-gate silence structurally
	// impossible rather than fixed-for-now: there is no second callback to leave
	// empty. Do not inline a closure at a resume call site.
	onProgress := func(evt exec.ProgressEvent) {
		switch evt.Type {
		case "execution-started":
			*execID = evt.ExecutionID
		case "step-started":
			chunked.Reset()
			if evt.NodeTitle != "" {
				titleByPos[evt.Position] = evt.NodeTitle
			}
			fmt.Fprintf(os.Stderr, "  ◌ %s\n", stepLabel(evt))
		case "step-chunk":
			fmt.Fprint(os.Stderr, evt.Chunk)
			chunked.WriteString(evt.Chunk)
			*lastOutput += evt.Chunk
		case "step-completed":
			// A gate step's output is the merged pre-gate context plus the
			// approval text — progress, not product. Render it, never pipe it.
			if chunked.Len() == 0 && !gatePositions[evt.Position] {
				*lastOutput += evt.Output
			}
			chunked.Reset()
			line := fmt.Sprintf("  ✓ %s", stepLabel(evt))
			// Builtins carry their offline/0-token treatment in the label; only
			// authorable-node steps show provider + token counts.
			if _, isBuiltin := builtins[evt.Position]; !isBuiltin {
				if evt.Provider != "" {
					line += " (" + evt.Provider + ")"
				}
				if evt.TokenUsage != nil {
					line += fmt.Sprintf(" %d tokens", evt.TokenUsage.Total)
				}
			}
			fmt.Fprintln(os.Stderr, line)
		case "step-failed":
			chunked.Reset()
			fmt.Fprintf(os.Stderr, "  ✗ %s: %s\n", stepLabel(evt), evt.Error)
			finalErr = evt.Error
		case "step-rejected":
			chunked.Reset()
			// RejectGate sets NodeTitle to the node ID; prefer the planner's title.
			label := stepLabel(evt)
			if t, ok := titleByPos[evt.Position]; ok {
				label = t
			}
			fmt.Fprintf(os.Stderr, "  ⊘ %s — declined\n", label)
		case "step-awaiting-input":
			if evt.NodeTitle != "" {
				titleByPos[evt.Position] = evt.NodeTitle
			}
			gatePositions[evt.Position] = true
			// Record only — the resume runs after the engine call returns.
			paused := evt
			pendingGate = &paused
		}
	}

	_, err := engine.RunWorkflow(ctx, nodeID, inputs, onProgress)
	if err != nil {
		if finalErr == "" {
			finalErr = err.Error()
		}
		fmt.Fprintf(os.Stderr, "Execution failed: %s\n", err.Error())
		return runResult{status: "failed", err: finalErr, exitCode: ExitFailed}
	}

	// Gate loop. A workflow can pause more than once, and a resume can itself
	// come back awaiting input — a single-gate fix would still be broken.
	var declined *gateDecision
	for pendingGate != nil {
		evt := *pendingGate
		pendingGate = nil

		decision := handleGate(evt, gate, gateTimeoutSec)
		if !decision.approved {
			if decision.reason != "" {
				fmt.Fprintln(os.Stderr, "    "+decision.reason)
			}
			// RejectGate is the real decline: it marks the gate step 'skipped' and
			// ends the execution 'cancelled'. StopExecution (the old call) only
			// cancels a REGISTERED RUNNING context — on a paused execution it did
			// nothing at all, leaving the row 'paused' forever.
			if rerr := engine.RejectGate(*execID, onProgress); rerr != nil {
				fmt.Fprintf(os.Stderr, "Gate decline failed: %s\n", rerr.Error())
				return runResult{status: "failed", err: rerr.Error(), exitCode: ExitFailed}
			}
			declined = &decision
			break
		}

		if _, rerr := engine.ResumeExecution(ctx, *execID, decision.input, onProgress); rerr != nil {
			// Never swallow this. Discarding the resume error is what hid GH#968.
			fmt.Fprintf(os.Stderr, "Resume failed after gate %q: %s\n", evt.NodeTitle, rerr.Error())
			if finalErr == "" {
				finalErr = rerr.Error()
			}
			return runResult{status: "failed", err: finalErr, exitCode: ExitFailed}
		}
	}

	return finalRunStatus(engine, *execID, finalErr, declined)
}

// finalRunStatus derives the reported status and exit code from the PERSISTED
// execution row. GH#968 — the runner returns a nil error on a gate pause, so
// `err == nil` never meant "completed": a run that paused and never resumed
// reported completed/exit 0 with empty output, which a caller cannot tell from a
// run that genuinely produced nothing. The database is the only honest source.
func finalRunStatus(engine *eng.App, execID, finalErr string, declined *gateDecision) runResult {
	if execID == "" {
		fmt.Fprintln(os.Stderr, "Execution status unknown: no execution was started.")
		return runResult{status: "failed", err: "no execution started", exitCode: ExitFailed}
	}

	status, errMsg, err := engine.ExecutionStatus(execID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read execution status: %s\n", err.Error())
		return runResult{status: "failed", err: err.Error(), exitCode: ExitFailed}
	}
	if errMsg != "" && finalErr == "" {
		finalErr = errMsg
	}

	switch status {
	case "completed":
		return runResult{status: "completed", exitCode: ExitOK}
	case "failed":
		if finalErr == "" {
			finalErr = "execution failed"
		}
		fmt.Fprintf(os.Stderr, "Execution failed: %s\n", finalErr)
		return runResult{status: "failed", err: finalErr, exitCode: ExitFailed}
	case "cancelled":
		// A declined gate is a deliberate stop, not a crash — but it must not
		// exit 0: nothing after the gate ran.
		code := ExitFailed
		msg := "Run stopped: gate declined — no step after the gate ran."
		if declined != nil && declined.timedOut {
			code = ExitTimeout
			msg = "Run stopped: gate timed out with no answer — no step after the gate ran."
		}
		fmt.Fprintln(os.Stderr, msg)
		return runResult{status: "cancelled", err: finalErr, exitCode: code}
	case "paused":
		fmt.Fprintln(os.Stderr, "Run is still awaiting gate input — no step after the gate ran.")
		return runResult{status: "paused", err: finalErr, exitCode: ExitFailed}
	default:
		fmt.Fprintf(os.Stderr, "Run ended in unexpected state %q — no completion was recorded.\n", status)
		return runResult{status: status, err: finalErr, exitCode: ExitFailed}
	}
}

// handleGate renders a gate and returns the decision. It performs NO engine
// calls — see gateDecision.
func handleGate(evt exec.ProgressEvent, mode gateHandling, timeoutSec int) gateDecision {
	switch mode {
	case gateAutoApprove:
		fmt.Fprintf(os.Stderr, "  ⏸ Gate: %s — auto-approved (--yes)\n", evt.NodeTitle)
		return gateDecision{approved: true, input: "approved"}
	case gateStrict:
		fmt.Fprintf(os.Stderr, "  ⏸ Gate: %s — rejected (--strict)\n", evt.NodeTitle)
		return gateDecision{approved: false}
	default:
		fmt.Fprintf(os.Stderr, "  ⏸ Gate: %s\n", evt.NodeTitle)
		if evt.GateInstructions != "" {
			fmt.Fprintln(os.Stderr, "    "+evt.GateInstructions)
		}
		fmt.Fprint(os.Stderr, "    Approve? [y/N] ")

		response, timedOut := readGateInput(timeoutSec)
		if !timedOut && strings.HasPrefix(strings.ToLower(strings.TrimSpace(response)), "y") {
			return gateDecision{approved: true, input: "approved"}
		}
		if timedOut {
			return gateDecision{approved: false, reason: "Gate timed out.", timedOut: true}
		}
		return gateDecision{approved: false, reason: "Gate rejected."}
	}
}

// readGateInput reads the approval answer, reporting whether the wait timed out
// (an unanswered gate is not the same decision as a typed "no").
func readGateInput(timeoutSec int) (string, bool) {
	if timeoutSec <= 0 {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			return scanner.Text(), false
		}
		return "", false
	}

	ch := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			ch <- scanner.Text()
		} else {
			ch <- ""
		}
	}()

	select {
	case response := <-ch:
		return response, false
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		fmt.Fprintln(os.Stderr)
		return "", true
	}
}

// parseInputs converts --input k=v flags into a map. Handles stdin via value=-.
func parseInputs(flags []string) (map[string]string, error) {
	result := make(map[string]string)
	stdinUsed := false

	for _, f := range flags {
		idx := strings.Index(f, "=")
		if idx < 0 {
			return nil, fmt.Errorf("invalid input format: %q (expected key=value)", f)
		}
		key := f[:idx]
		value := f[idx+1:]

		if value == "-" {
			if stdinUsed {
				return nil, fmt.Errorf("stdin (-) can only be used for one input")
			}
			stdinUsed = true
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil, fmt.Errorf("error reading stdin: %w", err)
			}
			value = string(data)
		}

		result[key] = value
	}

	return result, nil
}
