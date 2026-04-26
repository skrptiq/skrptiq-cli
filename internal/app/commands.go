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
		{Name: "/runs list", Description: "List recent executions"},
		{Name: "/resume", Description: "Resume a paused execution"},
		{Name: "/stop", Description: "Cancel the running workflow"},
		{Name: "/status", Description: "Show current execution status"},

		// Browse.
		{Name: "/list", Description: "List nodes by type (workflows, skills, prompts...)"},
		{Name: "/search", Description: "Search nodes by title", ArgProvider: allNodeCompleter},
		{Name: "/show", Description: "Show node content and metadata", ArgProvider: allNodeCompleter},

		// Hub.
		{Name: "/hub list", Description: "List imported skrpts"},
		{Name: "/hub search", Description: "Search community skrpts"},
		{Name: "/hub import", Description: "Import a skrpt from Hub"},
		{Name: "/hub update", Description: "Check for or apply updates"},

		// Profiles.
		{Name: "/profile list", Description: "List all voice profiles"},
		{Name: "/profile use", Description: "Switch active profile", ArgProvider: profileCompleter},
		{Name: "/profile show", Description: "Show active profile details"},

		// Persona dials.
		{Name: "/dials show", Description: "Show current persona dial settings"},
		{Name: "/dials set", Description: "Adjust a persona dial value"},

		// MCP & Services.
		{Name: "/mcp list", Description: "List MCP server connections"},
		{Name: "/mcp connect", Description: "Connect to an MCP server"},
		{Name: "/mcp disconnect", Description: "Disconnect an MCP server"},
		{Name: "/mcp tools", Description: "List available MCP tools"},
		{Name: "/providers list", Description: "List configured AI providers"},
		{Name: "/providers add", Description: "Configure a new provider"},

		// Workspace.
		{Name: "/workspace show", Description: "Show current workspace context"},
		{Name: "/workspace set", Description: "Change workspace directory"},

		// Tags.
		{Name: "/tags list", Description: "List all tags"},
		{Name: "/tag", Description: "Apply a tag to a node", ArgProvider: tagCompleter},
		{Name: "/untag", Description: "Remove a tag from a node", ArgProvider: tagCompleter},

		// Config.
		{Name: "/config show", Description: "Show current configuration"},
		{Name: "/config set", Description: "Update a configuration value"},

		// Prototype demos.
		{Name: "/demo", Description: "Run streaming progress demo"},
		{Name: "/tree", Description: "Show expandable execution tree demo"},
		{Name: "/gate", Description: "Show gate approval flow demo"},
		{Name: "/diff", Description: "Show diff review demo"},
	}
}
