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
		{Name: "/runs", Description: "List recent executions"},
		{Name: "/resume", Description: "Resume a paused execution"},
		{Name: "/stop", Description: "Cancel the running workflow"},
		{Name: "/status", Description: "Show current execution status"},

		// Browse.
		{Name: "/list", Description: "List nodes (workflows, skills, prompts...)"},
		{Name: "/search", Description: "Search nodes by title", ArgProvider: allNodeCompleter},
		{Name: "/show", Description: "Show node content and metadata", ArgProvider: allNodeCompleter},

		// Hub.
		{Name: "/hub", Description: "Hub status and available updates"},
		{Name: "/hub search", Description: "Search community skrpts"},
		{Name: "/hub import", Description: "Import a skrpt from Hub"},
		{Name: "/hub update", Description: "Check for or apply updates"},

		// Profiles.
		{Name: "/profile", Description: "Show or switch voice profile", ArgProvider: profileCompleter},
		{Name: "/dials", Description: "Show or adjust persona dials"},

		// MCP & Services.
		{Name: "/mcp", Description: "Show MCP server connections"},
		{Name: "/providers", Description: "List configured AI providers"},

		// Workspace.
		{Name: "/workspace", Description: "Show or change workspace context"},
		{Name: "/repos", Description: "List or add linked repositories"},

		// Tags.
		{Name: "/tags", Description: "List all tags"},
		{Name: "/tag", Description: "Apply a tag to a node", ArgProvider: tagCompleter},
		{Name: "/untag", Description: "Remove a tag from a node", ArgProvider: tagCompleter},

		// Config.
		{Name: "/config", Description: "Show or update configuration"},

		// Prototype demos.
		{Name: "/demo", Description: "Run streaming progress demo"},
		{Name: "/tree", Description: "Show expandable execution tree demo"},
		{Name: "/gate", Description: "Show gate approval flow demo"},
		{Name: "/diff", Description: "Show diff review demo"},
	}
}
