package app

import "github.com/skrptiq/skrptiq-cli/internal/components"

// SlashCommands is the registry of all available slash commands.
var SlashCommands = []components.Command{
	// Session.
	{Name: "/help", Description: "List all available commands"},
	{Name: "/clear", Description: "Clear session history"},

	// Runs.
	{Name: "/run", Description: "Execute a workflow or pick from list"},
	{Name: "/runs", Description: "List recent executions"},
	{Name: "/resume", Description: "Resume a paused execution"},
	{Name: "/stop", Description: "Cancel the running workflow"},
	{Name: "/status", Description: "Show current execution status"},

	// Browse.
	{Name: "/list", Description: "List nodes (workflows, skills, prompts...)"},
	{Name: "/search", Description: "Search nodes by title"},
	{Name: "/show", Description: "Show node content and metadata"},

	// Hub.
	{Name: "/hub", Description: "Hub status and available updates"},
	{Name: "/hub search", Description: "Search community skrpts"},
	{Name: "/hub import", Description: "Import a skrpt from Hub"},
	{Name: "/hub update", Description: "Check for or apply updates"},

	// Profiles.
	{Name: "/profile", Description: "Show or switch voice profile"},
	{Name: "/dials", Description: "Show or adjust persona dials"},

	// MCP & Services.
	{Name: "/mcp", Description: "Show MCP server connections"},
	{Name: "/providers", Description: "List configured AI providers"},

	// Workspace.
	{Name: "/workspace", Description: "Show or change workspace context"},
	{Name: "/repos", Description: "List or add linked repositories"},

	// Tags.
	{Name: "/tags", Description: "List all tags"},
	{Name: "/tag", Description: "Apply a tag to a node"},
	{Name: "/untag", Description: "Remove a tag from a node"},

	// Config.
	{Name: "/config", Description: "Show or update configuration"},

	// Prototype demos.
	{Name: "/demo", Description: "Run streaming progress demo"},
	{Name: "/tree", Description: "Show expandable execution tree demo"},
	{Name: "/gate", Description: "Show gate approval flow demo"},
	{Name: "/diff", Description: "Show diff review demo"},
}
