package app

import (
	"strings"

	"github.com/skrptiq/skrptiq-cli/internal/components"
	eng "github.com/skrptiq/skrptiq-cli/internal/engine"
)

// BuildCommands creates the slash command registry with arg providers
// wired to the engine. If engine is nil, no arg completion is available.
func BuildCommands(app *eng.App) []components.Command {
	nodeCompleter := func(nodeType string) func(string) []components.Completion {
		return func(partial string) []components.Completion {
			if app == nil {
				return nil
			}
			nodes, err := app.NodesByType(nodeType)
			if err != nil {
				return nil
			}
			partial = strings.ToLower(partial)
			var results []components.Completion
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
					results = append(results, components.Completion{
						Value:       n.Title,
						Description: desc,
					})
				}
			}
			return results
		}
	}

	allNodeCompleter := func(partial string) []components.Completion {
		if app == nil {
			return nil
		}
		nodes, err := app.DB.GetAllNodes()
		if err != nil {
			return nil
		}
		partial = strings.ToLower(partial)
		var results []components.Completion
		for _, n := range nodes {
			if partial == "" || strings.Contains(strings.ToLower(n.Title), partial) {
				results = append(results, components.Completion{
					Value:       n.Title,
					Description: n.Type,
				})
			}
		}
		return results
	}

	profileCompleter := func(partial string) []components.Completion {
		if app == nil {
			return nil
		}
		profiles, err := app.Profiles()
		if err != nil {
			return nil
		}
		partial = strings.ToLower(partial)
		var results []components.Completion
		for _, p := range profiles {
			if partial == "" || strings.Contains(strings.ToLower(p.Name), partial) {
				active := ""
				if p.IsActive == 1 {
					active = " (active)"
				}
				results = append(results, components.Completion{
					Value:       p.Name,
					Description: p.Type + active,
				})
			}
		}
		return results
	}

	tagCompleter := func(partial string) []components.Completion {
		if app == nil {
			return nil
		}
		tags, err := app.Tags()
		if err != nil {
			return nil
		}
		partial = strings.ToLower(partial)
		var results []components.Completion
		for _, t := range tags {
			if partial == "" || strings.Contains(strings.ToLower(t.Name), partial) {
				results = append(results, components.Completion{
					Value:       t.Name,
					Description: t.Colour,
				})
			}
		}
		return results
	}

	return []components.Command{
		// Session.
		{Name: "/help", Description: "List all available commands"},
		{Name: "/clear", Description: "Clear session history"},

		// Runs.
		{Name: "/run", Description: "Execute a workflow", ArgProvider: nodeCompleter("workflow")},
		{Name: "/runs", Description: "Execution history", Subcommands: []components.Subcommand{
			{Name: "list", Description: "List recent executions"},
		}},
		{Name: "/resume", Description: "Resume a paused execution"},
		{Name: "/stop", Description: "Cancel the running workflow"},
		{Name: "/status", Description: "Show current execution status"},

		// Browse.
		{Name: "/list", Description: "List nodes by type"},
		{Name: "/search", Description: "Search nodes by title", ArgProvider: allNodeCompleter},
		{Name: "/show", Description: "Show node content", ArgProvider: allNodeCompleter},

		// Hub.
		{Name: "/hub", Description: "Hub operations", Subcommands: []components.Subcommand{
			{Name: "list", Description: "List imported skrpts"},
			{Name: "search", Description: "Search community skrpts"},
			{Name: "import", Description: "Import a skrpt from Hub"},
			{Name: "update", Description: "Check for or apply updates"},
		}},

		// Profiles.
		{Name: "/profile", Description: "Voice profiles", Subcommands: []components.Subcommand{
			{Name: "list", Description: "List all profiles"},
			{Name: "show", Description: "Show active profile details"},
			{Name: "use", Description: "Switch active profile", ArgProvider: profileCompleter},
		}},

		// Persona dials.
		{Name: "/dials", Description: "Persona dials", Subcommands: []components.Subcommand{
			{Name: "show", Description: "Show current dial settings"},
			{Name: "set", Description: "Adjust a dial value"},
		}},

		// MCP & Services.
		{Name: "/mcp", Description: "MCP servers", Subcommands: []components.Subcommand{
			{Name: "list", Description: "List server connections"},
			{Name: "connect", Description: "Connect to a server"},
			{Name: "disconnect", Description: "Disconnect a server"},
			{Name: "tools", Description: "List available tools"},
		}},
		{Name: "/providers", Description: "AI providers", Subcommands: []components.Subcommand{
			{Name: "list", Description: "List configured providers"},
			{Name: "add", Description: "Configure a new provider"},
		}},

		// Workspace.
		{Name: "/workspace", Description: "Workspace context", Subcommands: []components.Subcommand{
			{Name: "show", Description: "Show current context"},
			{Name: "set", Description: "Change workspace directory"},
		}},

		// Tags.
		{Name: "/tags", Description: "Tags", Subcommands: []components.Subcommand{
			{Name: "list", Description: "List all tags"},
		}},
		{Name: "/tag", Description: "Apply a tag to a node", ArgProvider: tagCompleter},
		{Name: "/untag", Description: "Remove a tag from a node", ArgProvider: tagCompleter},

		// Config.
		{Name: "/config", Description: "Configuration", Subcommands: []components.Subcommand{
			{Name: "show", Description: "Show current configuration"},
			{Name: "set", Description: "Update a configuration value"},
		}},

		// Prototype demos.
		{Name: "/demo", Description: "Streaming progress demo"},
		{Name: "/tree", Description: "Execution tree demo"},
		{Name: "/gate", Description: "Gate approval demo"},
		{Name: "/diff", Description: "Diff review demo"},
	}
}
