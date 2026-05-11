package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	exec "github.com/skrptiq/engine/execution"
	eng "github.com/skrptiq/skrptiq-cli/internal/engine"
)

// inputFlag collects multiple --input k=v flags.
type inputFlag []string

func (f *inputFlag) String() string { return strings.Join(*f, ", ") }
func (f *inputFlag) Set(val string) error {
	*f = append(*f, val)
	return nil
}

// Run handles `skrptiq run <workflow> [--input k=v] [--output path] [--json] [--yes|--strict] [--gate-timeout N]`.
func Run(args []string, dbPath string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	var inputs inputFlag
	fs.Var(&inputs, "input", "Input variable (key=value, use value=- for stdin)")
	outputPath := fs.String("output", "", "Write final output to file")
	jsonOut := fs.Bool("json", false, "Output result as JSON")
	autoYes := fs.Bool("yes", false, "Auto-approve all gates")
	strict := fs.Bool("strict", false, "Fail on any gate step")
	gateTimeout := fs.Int("gate-timeout", 0, "Gate timeout in seconds (0 = no timeout)")

	if err := fs.Parse(args); err != nil {
		return ExitBadArgs
	}

	workflowName := strings.Join(fs.Args(), " ")
	if workflowName == "" {
		fmt.Fprintln(os.Stderr, "Usage: skrptiq run <workflow> [--input key=value] [--output path] [--json] [--yes|--strict]")
		return ExitBadArgs
	}

	// Parse inputs.
	inputMap, err := parseInputs(inputs)
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
	if *autoYes {
		gateMode = gateAutoApprove
	}
	if *strict {
		gateMode = gateStrict
	}

	result := runWorkflow(ctx, engine, node.ID, inputMap, gateMode, *gateTimeout, &lastOutput, &executionID)

	// Output.
	if *jsonOut {
		outputJSON(os.Stdout, map[string]any{
			"workflow":    plan.WorkflowTitle,
			"executionId": executionID,
			"status":      result.status,
			"output":      lastOutput,
			"error":       result.err,
		})
	} else if *outputPath != "" {
		if err := os.WriteFile(*outputPath, []byte(lastOutput), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			return ExitFailed
		}
		fmt.Fprintf(os.Stderr, "Output written to %s\n", *outputPath)
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
	gateDefault     gateHandling = iota
	gateAutoApprove
	gateStrict
)

type runResult struct {
	status   string
	err      string
	exitCode int
}

func runWorkflow(ctx context.Context, engine *eng.App, nodeID string, inputs map[string]string, gate gateHandling, gateTimeoutSec int, lastOutput *string, execID *string) runResult {
	var finalErr string

	_, err := engine.RunWorkflow(ctx, nodeID, inputs, func(evt exec.ProgressEvent) {
		switch evt.Type {
		case "execution-started":
			*execID = evt.ExecutionID
		case "step-started":
			fmt.Fprintf(os.Stderr, "  ◌ %s\n", evt.NodeTitle)
		case "step-chunk":
			fmt.Fprint(os.Stderr, evt.Chunk)
			*lastOutput += evt.Chunk
		case "step-completed":
			line := fmt.Sprintf("  ✓ %s", evt.NodeTitle)
			if evt.Provider != "" {
				line += " (" + evt.Provider + ")"
			}
			if evt.TokenUsage != nil {
				line += fmt.Sprintf(" %d tokens", evt.TokenUsage.Total)
			}
			fmt.Fprintln(os.Stderr, line)
		case "step-failed":
			fmt.Fprintf(os.Stderr, "  ✗ %s: %s\n", evt.NodeTitle, evt.Error)
			finalErr = evt.Error
		case "step-awaiting-input":
			handleGate(engine, *execID, evt, gate, gateTimeoutSec)
		}
	})

	if err != nil {
		if finalErr == "" {
			finalErr = err.Error()
		}
		fmt.Fprintf(os.Stderr, "Execution failed: %s\n", err.Error())
		return runResult{status: "failed", err: finalErr, exitCode: ExitFailed}
	}

	return runResult{status: "completed", exitCode: ExitOK}
}

func handleGate(engine *eng.App, execID string, evt exec.ProgressEvent, mode gateHandling, timeoutSec int) {
	switch mode {
	case gateAutoApprove:
		fmt.Fprintf(os.Stderr, "  ⏸ Gate: %s — auto-approved (--yes)\n", evt.NodeTitle)
		go engine.ResumeExecution(context.Background(), execID, "approved", func(e exec.ProgressEvent) {})
	case gateStrict:
		fmt.Fprintf(os.Stderr, "  ⏸ Gate: %s — rejected (--strict)\n", evt.NodeTitle)
		engine.StopExecution(execID)
	default:
		fmt.Fprintf(os.Stderr, "  ⏸ Gate: %s\n", evt.NodeTitle)
		if evt.GateInstructions != "" {
			fmt.Fprintln(os.Stderr, "    "+evt.GateInstructions)
		}
		fmt.Fprint(os.Stderr, "    Approve? [y/N] ")

		response := readGateInput(timeoutSec)
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(response)), "y") {
			go engine.ResumeExecution(context.Background(), execID, "approved", func(e exec.ProgressEvent) {})
		} else {
			fmt.Fprintln(os.Stderr, "    Gate rejected.")
			engine.StopExecution(execID)
		}
	}
}

func readGateInput(timeoutSec int) string {
	if timeoutSec <= 0 {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			return scanner.Text()
		}
		return ""
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
		return response
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		fmt.Fprintln(os.Stderr, "\n    Gate timed out.")
		return ""
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
