package cli

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	eng "github.com/skrptiq/skrptiq-cli/internal/engine"
)

// List handles `skrptiq list <type> [--json] [-q]`.
// Types: nodes, workflows, skills, prompts, profiles, runs.
func List(args []string, dbPath string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON")
	quiet := fs.Bool("q", false, "Quiet — IDs only")
	nodeType := fs.String("type", "", "Filter nodes by type")
	if err := fs.Parse(args); err != nil {
		return ExitBadArgs
	}

	target := fs.Arg(0)
	if target == "" {
		target = "nodes"
	}

	engine := OpenEngine(dbPath)
	if engine == nil {
		return ExitFailed
	}
	defer engine.Close()

	switch target {
	case "nodes":
		return listNodes(engine, *nodeType, *jsonOut, *quiet)
	case "workflows":
		return listNodes(engine, "workflow", *jsonOut, *quiet)
	case "skills":
		return listNodes(engine, "skill", *jsonOut, *quiet)
	case "prompts":
		return listNodes(engine, "prompt", *jsonOut, *quiet)
	case "profiles":
		return listProfiles(engine, *jsonOut, *quiet)
	case "runs":
		return listRuns(engine, *jsonOut, *quiet)
	default:
		fmt.Fprintf(os.Stderr, "Unknown list target: %s\nValid: nodes, workflows, skills, prompts, profiles, runs\n", target)
		return ExitBadArgs
	}
}

type nodeEntry struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

func listNodes(engine *eng.App, typeFilter string, jsonOut, quiet bool) int {
	var entries []nodeEntry
	if typeFilter != "" {
		nodes, err := engine.NodesByType(typeFilter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return ExitFailed
		}
		for _, n := range nodes {
			entries = append(entries, nodeEntry{ID: n.ID, Type: n.Type, Title: n.Title})
		}
	} else {
		nodes, err := engine.DB.GetAllNodes()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return ExitFailed
		}
		for _, n := range nodes {
			entries = append(entries, nodeEntry{ID: n.ID, Type: n.Type, Title: n.Title})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type < entries[j].Type
		}
		return strings.ToLower(entries[i].Title) < strings.ToLower(entries[j].Title)
	})

	if jsonOut {
		outputJSON(os.Stdout, entries)
		return ExitOK
	}
	for _, e := range entries {
		if quiet {
			fmt.Println(e.ID)
		} else {
			fmt.Printf("  %-12s %s\n", e.Type, e.Title)
		}
	}
	return ExitOK
}

type profileEntry struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Active bool   `json:"active"`
}

func listProfiles(engine *eng.App, jsonOut, quiet bool) int {
	profiles, err := engine.Profiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	var entries []profileEntry
	for _, p := range profiles {
		entries = append(entries, profileEntry{
			ID:     p.ID,
			Name:   p.Name,
			Type:   p.Type,
			Active: p.IsActive == 1,
		})
	}

	if jsonOut {
		outputJSON(os.Stdout, entries)
		return ExitOK
	}
	for _, e := range entries {
		if quiet {
			fmt.Println(e.ID)
		} else {
			active := "  "
			if e.Active {
				active = "● "
			}
			fmt.Printf("  %s%-12s %s\n", active, e.Type, e.Name)
		}
	}
	return ExitOK
}

type runEntry struct {
	ID       string `json:"id"`
	Workflow string `json:"workflow"`
	Status   string `json:"status"`
	Started  string `json:"started"`
	Tokens   int    `json:"tokens"`
}

func listRuns(engine *eng.App, jsonOut, quiet bool) int {
	runs, err := engine.ListExecutions(20)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	var entries []runEntry
	for _, r := range runs {
		entries = append(entries, runEntry{
			ID:       r.ID,
			Workflow: r.WorkflowTitle,
			Status:   r.Status,
			Started:  r.StartedAt,
			Tokens:   r.TotalTokens,
		})
	}

	if jsonOut {
		outputJSON(os.Stdout, entries)
		return ExitOK
	}
	for _, e := range entries {
		if quiet {
			fmt.Println(e.ID)
		} else {
			shortID := e.ID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			fmt.Printf("  %-10s %s  %s  %s\n", e.Status, shortID, e.Workflow, e.Started)
		}
	}
	return ExitOK
}
