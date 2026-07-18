package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	eng "github.com/skrptiq/skrptiq-cli/internal/engine"
)

// Show handles `skrptiq show <type> <name|id> [--json] [--step N]`.
// Types: node, run.
func Show(args []string, dbPath string) int {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON")
	stepNum := fs.Int("step", 0, "Show specific step output (for runs)")
	if err := fs.Parse(args); err != nil {
		return ExitBadArgs
	}

	if fs.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "Usage: skrptiq show <node|run> <name|id> [--json] [--step N]")
		return ExitBadArgs
	}

	target := fs.Arg(0)
	name := strings.Join(fs.Args()[1:], " ")

	engine := OpenEngine(dbPath)
	if engine == nil {
		return ExitFailed
	}
	defer engine.Close()

	switch target {
	case "node":
		return showNode(engine, name, *jsonOut)
	case "run":
		return showRun(engine, name, *stepNum, *jsonOut)
	default:
		fmt.Fprintf(os.Stderr, "Unknown show target: %s\nValid: node, run\n", target)
		return ExitBadArgs
	}
}

type nodeDetail struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Content     *string `json:"content,omitempty"`
}

func showNode(engine *eng.App, name string, jsonOut bool) int {
	node, err := engine.FindNodeByTitle(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}
	if node == nil {
		fmt.Fprintf(os.Stderr, "Node not found: %s\n", name)
		return ExitBadArgs
	}

	if jsonOut {
		outputJSON(os.Stdout, nodeDetail{
			ID:          node.ID,
			Type:        node.Type,
			Title:       node.Title,
			Description: node.Description,
			Content:     node.Content,
		})
		return ExitOK
	}

	fmt.Println(node.Title)
	fmt.Printf("  Type: %s\n", node.Type)
	if node.Description != nil && *node.Description != "" {
		fmt.Printf("  Description: %s\n", *node.Description)
	}
	if node.Content != nil && *node.Content != "" {
		fmt.Printf("\n%s\n", *node.Content)
	}
	return ExitOK
}

func showRun(engine *eng.App, idPrefix string, stepNum int, jsonOut bool) int {
	fullID, err := engine.FindRunByPrefix(idPrefix)
	if err != nil || fullID == nil {
		fmt.Fprintf(os.Stderr, "Run not found: %s\n", idPrefix)
		return ExitBadArgs
	}

	run, err := engine.GetRunDetail(*fullID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	// Show specific step output.
	if stepNum > 0 {
		for _, s := range run.Steps {
			if s.Position == stepNum {
				if jsonOut {
					outputJSON(os.Stdout, s)
					return ExitOK
				}
				title := s.NodeTitle
				if s.ToolObject != nil {
					title = s.ToolObject.Label()
				}
				fmt.Printf("%s — step %d\n", title, s.Position)
				fmt.Printf("  Status: %s\n", s.Status)
				if s.Provider != "" {
					p := s.Provider
					if s.Model != "" {
						p += " / " + s.Model
					}
					fmt.Printf("  Provider: %s\n", p)
				}
				if s.Duration != "" {
					fmt.Printf("  Duration: %s\n", s.Duration)
				}
				if s.Error != "" {
					fmt.Printf("  Error: %s\n", s.Error)
				}
				if s.Output != "" {
					fmt.Printf("\n%s\n", s.Output)
				} else {
					fmt.Println("\n(no output)")
				}
				return ExitOK
			}
		}
		fmt.Fprintf(os.Stderr, "Step %d not found.\n", stepNum)
		return ExitBadArgs
	}

	if jsonOut {
		outputJSON(os.Stdout, run)
		return ExitOK
	}

	fmt.Println(run.WorkflowTitle)
	fmt.Printf("  ID: %s\n", run.ID)
	fmt.Printf("  Status: %s\n", run.Status)
	fmt.Printf("  Started: %s\n", run.StartedAt)
	if run.CompletedAt != nil {
		fmt.Printf("  Completed: %s\n", *run.CompletedAt)
	}
	if run.TotalTokens > 0 {
		fmt.Printf("  Tokens: %d\n", run.TotalTokens)
	}
	if run.Error != nil && *run.Error != "" {
		fmt.Printf("  Error: %s\n", *run.Error)
	}
	if len(run.Steps) > 0 {
		fmt.Println("\nSteps")
		for _, s := range run.Steps {
			// GH#873 — render node-less builtins as distinct, read-only objects.
			if s.ToolObject != nil {
				fmt.Printf("  %d. %s [%s]\n", s.Position, s.ToolObject.Label(), s.Status)
				continue
			}
			line := fmt.Sprintf("  %d. %s [%s]", s.Position, s.NodeTitle, s.Status)
			if s.Provider != "" {
				line += " (" + s.Provider + ")"
			}
			if s.Duration != "" {
				line += " " + s.Duration
			}
			fmt.Println(line)
		}
		shortID := run.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		fmt.Printf("\nUse: skrptiq show run %s --step <n> to view step output.\n", shortID)
	}
	return ExitOK
}
