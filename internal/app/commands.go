package app

import (
	"strings"

	eng "github.com/skrptiq/skrptiq-cli/internal/engine"
)

// BuildCommands creates the slash command registry with arg providers
// wired to the engine. If engine is nil, no arg completion is available.
func BuildCommands(app *eng.App) []Command {
	nodeCompleter := func(nodeType string) func(string) []Completion {
		return func(partial string) []Completion {
			if app == nil {
				return nil
			}
			nodes, err := app.NodesByType(nodeType)
			if err != nil {
				return nil
			}
			partial = strings.ToLower(partial)
			var results []Completion
			for _, n := range nodes {
				if partial == "" || strings.Contains(strings.ToLower(n.Title), partial) {
					desc := n.Type
					if n.Description != nil {
						d := *n.Description
						if len(d) > 60 {
							d = d[:57] + "..."
						}
						desc = d
					}
					results = append(results, Completion{
						Value:       n.Title,
						Description: desc,
					})
				}
			}
			return results
		}
	}

	allNodeCompleter := func(partial string) []Completion {
		if app == nil {
			return nil
		}
		nodes, err := app.DB.GetAllNodes()
		if err != nil {
			return nil
		}
		partial = strings.ToLower(partial)
		var results []Completion
		for _, n := range nodes {
			if partial == "" || strings.Contains(strings.ToLower(n.Title), partial) {
				results = append(results, Completion{
					Value:       n.Title,
					Description: n.Type,
				})
			}
		}
		return results
	}

	profileCompleter := func(partial string) []Completion {
		if app == nil {
			return nil
		}
		profiles, err := app.Profiles()
		if err != nil {
			return nil
		}
		partial = strings.ToLower(partial)
		var results []Completion
		for _, p := range profiles {
			if partial == "" || strings.Contains(strings.ToLower(p.Name), partial) {
				active := ""
				if p.IsActive == 1 {
					active = " (active)"
				}
				results = append(results, Completion{
					Value:       p.Name,
					Description: p.Type + active,
				})
			}
		}
		return results
	}

	tagCompleter := func(partial string) []Completion {
		if app == nil {
			return nil
		}
		tags, err := app.Tags()
		if err != nil {
			return nil
		}
		partial = strings.ToLower(partial)
		var results []Completion
		for _, t := range tags {
			if partial == "" || strings.Contains(strings.ToLower(t.Name), partial) {
				results = append(results, Completion{
					Value:       t.Name,
					Description: t.Colour,
				})
			}
		}
		return results
	}

	statusLabel := func(s string) string {
		switch s {
		case "completed":
			return "✓"
		case "failed":
			return "✗"
		case "running":
			return "◌"
		case "paused":
			return "⏸"
		default:
			return "○"
		}
	}

	runCompleter := func(a *eng.App) func(string) []Completion {
		return func(partial string) []Completion {
			if a == nil {
				return nil
			}
			runs, err := a.ListExecutions(20)
			if err != nil {
				return nil
			}
			partial = strings.ToLower(partial)
			var results []Completion
			for _, r := range runs {
				shortID := r.ID
				if len(shortID) > 8 {
					shortID = shortID[:8]
				}
				label := shortID + " " + r.WorkflowTitle
				if partial == "" || strings.Contains(strings.ToLower(label), partial) {
					results = append(results, Completion{
						Value:       shortID,
						Description: statusLabel(r.Status) + " " + r.WorkflowTitle + " " + r.StartedAt,
					})
				}
			}
			return results
		}
	}

	return []Command{
		// Modes.
		{Name: "/chat", Description: "Enter chat mode"},
		{Name: "/run", Description: "Enter run mode", ArgProvider: nodeCompleter("workflow")},
		{Name: "/command", Description: "Return to command mode"},
		{Name: "/exit", Description: "Exit current mode"},

		// Session.
		{Name: "/help", Description: "List all available commands"},
		{Name: "/clear", Description: "Clear session history"},

		// Execution (deferred — needs engine runner).
		{Name: "/resume", Description: "Resume a paused execution"},
		{Name: "/stop", Description: "Cancel the running workflow"},

		// Runs.
		{Name: "/runs", Description: "Execution history", Subcommands: []Subcommand{
			{Name: "list", Description: "List recent executions"},
			{Name: "status", Description: "Show active executions"},
			{Name: "show", Description: "Show run details", ArgProvider: runCompleter(app)},
		}},

		// Browse.
		{Name: "/list", Description: "List nodes by type"},
		{Name: "/search", Description: "Search nodes by title", ArgProvider: allNodeCompleter},
		{Name: "/show", Description: "Show node content", ArgProvider: allNodeCompleter},

		// Hub.
		{Name: "/hub", Description: "Hub operations", Subcommands: []Subcommand{
			{Name: "list", Description: "List imported skrpts"},
			{Name: "search", Description: "Search community skrpts"},
			{Name: "import", Description: "Import a skrpt from Hub"},
			{Name: "update", Description: "Check for or apply updates"},
		}},

		// Profiles (includes quality controls — they're a profile property).
		{Name: "/profile", Description: "Voice profiles", Subcommands: []Subcommand{
			{Name: "list", Description: "List all profiles"},
			{Name: "show", Description: "Show active profile details"},
			{Name: "use", Description: "Switch active profile", ArgProvider: profileCompleter},
			{Name: "controls", Description: "Show quality control settings"},
		}},

		// MCP.
		{Name: "/mcp", Description: "MCP servers", Subcommands: []Subcommand{
			{Name: "list", Description: "List server connections"},
			{Name: "tools", Description: "List available tools"},
			{Name: "connect", Description: "Connect to a server"},
			{Name: "disconnect", Description: "Disconnect a server"},
		}},

		// Tags.
		{Name: "/tags", Description: "Tags", Subcommands: []Subcommand{
			{Name: "list", Description: "List all tags"},
		}},
		{Name: "/tag", Description: "Apply a tag to a node", ArgProvider: tagCompleter},
		{Name: "/untag", Description: "Remove a tag from a node", ArgProvider: tagCompleter},

		// Workspace.
		{Name: "/workspace", Description: "Workspace context", Subcommands: []Subcommand{
			{Name: "show", Description: "Show current context"},
			{Name: "set", Description: "Change workspace directory"},
		}},

		// Settings.
		{Name: "/settings", Description: "App settings", Subcommands: []Subcommand{
			{Name: "about", Description: "Version and system info"},
			{Name: "providers", Description: "AI provider configuration"},
			{Name: "connections", Description: "All connections (providers, MCP, services)"},
			{Name: "config", Description: "Show configuration values"},
			{Name: "set", Description: "Update a configuration value"},
		}},

		// Prototype demos.
		{Name: "/demo", Description: "Streaming progress demo"},
		{Name: "/tree", Description: "Execution tree demo"},
		{Name: "/gate", Description: "Gate approval demo"},
		{Name: "/diff", Description: "Diff review demo"},
	}
}
