package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/skrptiq/skrptiq-cli/internal/theme"
	"github.com/skrptiq/skrptiq-cli/internal/views/repl"
)

// handleSlashCommand processes implemented slash commands.
// cmd is the first word (e.g. "hub"), args is everything after (e.g. "list" or "search query").
// Returns the updated model, a tea command, and whether the command was handled.
func handleSlashCommand(m *Model, cmd string, args string) (Model, tea.Cmd, bool) {
	// Split args into subcommand and remaining arguments.
	sub, subArgs := splitFirst(args)

	switch cmd {
	case "help":
		m.repl.AddOutput(helpText())
		return *m, nil, true

	case "clear":
		m.repl = repl.NewWithPrompt(m.repl.Prompt(), m.commands)
		resizeView(m)
		return *m, m.repl.Init(), true

	case "list":
		handleList(m, args) // args is the type filter
		return *m, nil, true

	case "show":
		handleShow(m, args)
		return *m, nil, true

	case "search":
		handleSearch(m, args)
		return *m, nil, true

	case "hub":
		return handleHub(m, sub, subArgs)

	case "runs":
		return handleRuns(m, sub)

	case "profile":
		return handleProfile(m, sub, subArgs)

	case "dials":
		return handleDials(m, sub, subArgs)

	case "mcp":
		return handleMCPCmd(m, sub)

	case "providers":
		return handleProvidersCmd(m, sub)

	case "workspace":
		return handleWorkspaceCmd(m, sub)

	case "tags":
		return handleTagsCmd(m, sub)

	case "config":
		return handleConfigCmd(m, sub)
	}

	return *m, nil, false
}

// splitFirst splits a string into the first word and the rest.
func splitFirst(s string) (string, string) {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, " "); idx > 0 {
		return strings.ToLower(s[:idx]), strings.TrimSpace(s[idx+1:])
	}
	return strings.ToLower(s), ""
}

// --- /list ---

func handleList(m *Model, args string) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return
	}

	nodeType := strings.TrimSpace(strings.ToLower(args))

	typeMap := map[string]string{
		"workflows": "workflow", "workflow": "workflow",
		"skills": "skill", "skill": "skill",
		"agents": "skill", "agent": "skill",
		"prompts": "prompt", "prompt": "prompt",
		"sources": "source", "source": "source",
		"documents": "document", "document": "document",
		"assets": "asset", "asset": "asset",
		"services": "service", "service": "service",
	}

	var nodes []struct{ Title, Type string }
	var err error

	if nodeType == "" {
		all, e := m.engine.DB.GetAllNodes()
		err = e
		for _, n := range all {
			nodes = append(nodes, struct{ Title, Type string }{n.Title, n.Type})
		}
	} else {
		mapped, ok := typeMap[nodeType]
		if !ok {
			m.repl.AddOutput(theme.ErrorText.Render("Unknown node type: " + args + ". Try: workflows, skills, prompts, sources, documents, assets, services"))
			return
		}
		filtered, e := m.engine.NodesByType(mapped)
		err = e
		for _, n := range filtered {
			nodes = append(nodes, struct{ Title, Type string }{n.Title, n.Type})
		}
	}

	if err != nil {
		m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}

	if len(nodes) == 0 {
		m.repl.AddOutput(theme.Faint.Render("No nodes found."))
		return
	}

	var b strings.Builder
	typeStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(12)
	for _, n := range nodes {
		b.WriteString(fmt.Sprintf("  %s %s\n", typeStyle.Render(n.Type), n.Title))
	}
	m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))
}

// --- /show ---

func handleShow(m *Model, args string) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return
	}

	title := strings.TrimSpace(args)
	if title == "" {
		m.repl.AddOutput(theme.Faint.Render("Usage: /show <node name>"))
		return
	}

	node, err := m.engine.FindNodeByTitle(title)
	if err != nil {
		m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}
	if node == nil {
		m.repl.AddOutput(theme.Faint.Render("No node found: " + title))
		return
	}

	var b strings.Builder
	b.WriteString(theme.Title.Render(node.Title) + "\n")
	b.WriteString(theme.Faint.Render("Type: ") + node.Type + "\n")
	if node.Description != nil && *node.Description != "" {
		b.WriteString(theme.Faint.Render("Description: ") + *node.Description + "\n")
	}
	if node.Content != nil && *node.Content != "" {
		b.WriteString("\n" + *node.Content)
	}
	m.repl.AddOutput(b.String())
}

// --- /search ---

func handleSearch(m *Model, args string) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return
	}

	query := strings.TrimSpace(args)
	if query == "" {
		m.repl.AddOutput(theme.Faint.Render("Usage: /search <query>"))
		return
	}

	nodes, err := m.engine.SearchNodes(query)
	if err != nil {
		m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}

	if len(nodes) == 0 {
		m.repl.AddOutput(theme.Faint.Render("No results for: " + query))
		return
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%d results for %q:\n", len(nodes), query))
	typeStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(12)
	for _, n := range nodes {
		b.WriteString(fmt.Sprintf("  %s %s\n", typeStyle.Render(n.Type), n.Title))
	}
	m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))
}

// --- /hub ---

func handleHub(m *Model, sub, args string) (Model, tea.Cmd, bool) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return *m, nil, true
	}

	switch sub {
	case "list":
		imports, err := m.engine.HubImports()
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return *m, nil, true
		}

		var b strings.Builder
		b.WriteString(theme.Title.Render("Hub — Imported Skrpts") + "\n")
		if len(imports) == 0 {
			b.WriteString(theme.Faint.Render("  No skrpts imported from Hub."))
		} else {
			for _, imp := range imports {
				ver := ""
				if imp.Version != nil {
					ver = theme.Faint.Render(" v" + *imp.Version)
				}
				b.WriteString(fmt.Sprintf("  %s%s\n", imp.Name, ver))
			}
		}
		m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))

	case "search":
		m.repl.AddOutput(theme.Faint.Render("/hub search — requires Hub API client. Coming soon."))

	case "import":
		m.repl.AddOutput(theme.Faint.Render("/hub import — requires Hub API client. Coming soon."))

	case "update":
		m.repl.AddOutput(theme.Faint.Render("/hub update — requires Hub API client. Coming soon."))

	default:
		m.repl.AddOutput(usageBlock("/hub", []string{
			"list    — List imported skrpts",
			"search  — Search community skrpts",
			"import  — Import a skrpt from Hub",
			"update  — Check for or apply updates",
		}))
	}

	return *m, nil, true
}

// --- /runs ---

func handleRuns(m *Model, sub string) (Model, tea.Cmd, bool) {
	switch sub {
	case "list", "":
		m.repl.AddOutput(theme.Faint.Render("/runs list — execution history display coming soon."))
	default:
		m.repl.AddOutput(usageBlock("/runs", []string{
			"list    — List recent executions",
		}))
	}
	return *m, nil, true
}

// --- /profile ---

func handleProfile(m *Model, sub, args string) (Model, tea.Cmd, bool) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return *m, nil, true
	}

	switch sub {
	case "list":
		profiles, err := m.engine.Profiles()
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return *m, nil, true
		}
		if len(profiles) == 0 {
			m.repl.AddOutput(theme.Faint.Render("No profiles configured."))
			return *m, nil, true
		}
		var b strings.Builder
		b.WriteString(theme.Title.Render("Profiles") + "\n")
		typeStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(12)
		for _, p := range profiles {
			active := "  "
			if p.IsActive == 1 {
				active = theme.SuccessText.Render("● ")
			}
			b.WriteString(fmt.Sprintf("  %s%s %s\n", active, typeStyle.Render(p.Type), p.Name))
		}
		m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))

	case "show":
		voice, _ := m.engine.ActiveProfile("voice")
		if voice == nil {
			m.repl.AddOutput(theme.Faint.Render("No active voice profile."))
			return *m, nil, true
		}
		var b strings.Builder
		b.WriteString(theme.Title.Render(voice.Name) + "\n")
		b.WriteString(theme.Faint.Render("Type: ") + voice.Type + "\n")
		if voice.Content != "" {
			b.WriteString("\n" + voice.Content)
		}
		m.repl.AddOutput(b.String())

	case "use":
		name := strings.TrimSpace(args)
		if name == "" {
			m.repl.AddOutput(theme.Faint.Render("Usage: /profile use <name>"))
			return *m, nil, true
		}
		m.repl.AddOutput(theme.Faint.Render("Profile switching to \"" + name + "\" — coming soon."))

	default:
		m.repl.AddOutput(usageBlock("/profile", []string{
			"list    — List all profiles",
			"show    — Show active profile details",
			"use     — Switch active profile",
		}))
	}

	return *m, nil, true
}

// --- /dials ---

func handleDials(m *Model, sub, args string) (Model, tea.Cmd, bool) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return *m, nil, true
	}

	switch sub {
	case "show":
		voice, _ := m.engine.ActiveProfile("voice")
		if voice == nil {
			m.repl.AddOutput(theme.Faint.Render("No active voice profile. Dials are configured per profile."))
			return *m, nil, true
		}
		var b strings.Builder
		b.WriteString(theme.Title.Render("Persona Dials") + " — " + voice.Name + "\n")
		if voice.Metadata != nil && *voice.Metadata != "" {
			b.WriteString(theme.Faint.Render(*voice.Metadata))
		} else {
			b.WriteString(theme.Faint.Render("No dial configuration in profile metadata."))
		}
		m.repl.AddOutput(b.String())

	case "set":
		m.repl.AddOutput(theme.Faint.Render("/dials set — coming soon."))

	default:
		m.repl.AddOutput(usageBlock("/dials", []string{
			"show    — Show current persona dial settings",
			"set     — Adjust a persona dial value",
		}))
	}

	return *m, nil, true
}

// --- /mcp ---

func handleMCPCmd(m *Model, sub string) (Model, tea.Cmd, bool) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return *m, nil, true
	}

	switch sub {
	case "list":
		servers, err := m.engine.MCPServers()
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return *m, nil, true
		}
		if len(servers) == 0 {
			m.repl.AddOutput(theme.Faint.Render("No MCP servers configured."))
			return *m, nil, true
		}
		var b strings.Builder
		b.WriteString(theme.Title.Render("MCP Servers") + "\n")
		for _, s := range servers {
			indicator := theme.ErrorText.Render("●")
			if s.Status == "connected" {
				indicator = theme.SuccessText.Render("●")
			}
			b.WriteString(fmt.Sprintf("  %s %s", indicator, s.Name))
			if s.Provider != "" {
				b.WriteString(theme.Faint.Render(" (" + s.Provider + ")"))
			}
			b.WriteString("\n")
		}
		m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))

	case "connect":
		m.repl.AddOutput(theme.Faint.Render("/mcp connect — coming soon."))
	case "disconnect":
		m.repl.AddOutput(theme.Faint.Render("/mcp disconnect — coming soon."))
	case "tools":
		m.repl.AddOutput(theme.Faint.Render("/mcp tools — coming soon."))

	default:
		m.repl.AddOutput(usageBlock("/mcp", []string{
			"list        — List MCP server connections",
			"connect     — Connect to an MCP server",
			"disconnect  — Disconnect an MCP server",
			"tools       — List available MCP tools",
		}))
	}

	return *m, nil, true
}

// --- /providers ---

func handleProvidersCmd(m *Model, sub string) (Model, tea.Cmd, bool) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return *m, nil, true
	}

	switch sub {
	case "list":
		providers, err := m.engine.Providers()
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return *m, nil, true
		}
		if len(providers) == 0 {
			m.repl.AddOutput(theme.Faint.Render("No AI providers configured."))
			return *m, nil, true
		}
		var b strings.Builder
		b.WriteString(theme.Title.Render("AI Providers") + "\n")
		for _, p := range providers {
			indicator := theme.ErrorText.Render("●")
			if p.Status == "connected" {
				indicator = theme.SuccessText.Render("●")
			}
			b.WriteString(fmt.Sprintf("  %s %s", indicator, p.Name))
			if p.Provider != "" {
				b.WriteString(theme.Faint.Render(" (" + p.Provider + ")"))
			}
			b.WriteString("\n")
		}
		m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))

	case "add":
		m.repl.AddOutput(theme.Faint.Render("/providers add — coming soon."))

	default:
		m.repl.AddOutput(usageBlock("/providers", []string{
			"list    — List configured AI providers",
			"add     — Configure a new provider",
		}))
	}

	return *m, nil, true
}

// --- /workspace ---

func handleWorkspaceCmd(m *Model, sub string) (Model, tea.Cmd, bool) {
	switch sub {
	case "show":
		var b strings.Builder
		b.WriteString(theme.Title.Render("Workspace") + "\n")
		b.WriteString("  " + theme.Faint.Render("Path: ") + m.statusBar.Workspace + "\n")
		b.WriteString("  " + theme.Faint.Render("Profile: ") + m.statusBar.Profile)
		if m.engine != nil {
			b.WriteString("\n  " + theme.Faint.Render("Database: ") + m.engine.DB.Path())
		}
		m.repl.AddOutput(b.String())

	case "set":
		m.repl.AddOutput(theme.Faint.Render("/workspace set — coming soon."))

	default:
		m.repl.AddOutput(usageBlock("/workspace", []string{
			"show    — Show current workspace context",
			"set     — Change workspace directory",
		}))
	}

	return *m, nil, true
}

// --- /tags ---

func handleTagsCmd(m *Model, sub string) (Model, tea.Cmd, bool) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return *m, nil, true
	}

	switch sub {
	case "list":
		tags, err := m.engine.Tags()
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return *m, nil, true
		}
		if len(tags) == 0 {
			m.repl.AddOutput(theme.Faint.Render("No tags defined."))
			return *m, nil, true
		}
		var b strings.Builder
		b.WriteString(theme.Title.Render("Tags") + "\n")
		for _, t := range tags {
			colour := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Colour))
			b.WriteString(fmt.Sprintf("  %s %s\n", colour.Render("●"), t.Name))
		}
		m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))

	default:
		m.repl.AddOutput(usageBlock("/tags", []string{
			"list    — List all tags",
		}))
	}

	return *m, nil, true
}

// --- /config ---

func handleConfigCmd(m *Model, sub string) (Model, tea.Cmd, bool) {
	switch sub {
	case "show":
		var b strings.Builder
		b.WriteString(theme.Title.Render("Configuration") + "\n")

		if m.engine == nil {
			b.WriteString(theme.Faint.Render("  No engine connection."))
			m.repl.AddOutput(b.String())
			return *m, nil, true
		}

		keys := []struct{ key, label string }{
			{"defaultProvider", "Default Provider"},
			{"defaultModel", "Default Model"},
			{"workspacePath", "Workspace Path"},
			{"theme", "Theme"},
		}

		for _, k := range keys {
			val := m.engine.Setting(k.key)
			if val == "" {
				val = theme.Faint.Render("(not set)")
			}
			b.WriteString(fmt.Sprintf("  %s %s\n",
				lipgloss.NewStyle().Foreground(theme.Muted).Width(20).Render(k.label+":"), val))
		}
		m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))

	case "set":
		m.repl.AddOutput(theme.Faint.Render("/config set — coming soon."))

	default:
		m.repl.AddOutput(usageBlock("/config", []string{
			"show    — Show current configuration",
			"set     — Update a configuration value",
		}))
	}

	return *m, nil, true
}

// --- helpers ---

func usageBlock(cmd string, subcommands []string) string {
	var b strings.Builder
	b.WriteString(theme.Title.Render(cmd) + "\n")
	for _, s := range subcommands {
		b.WriteString("  " + cmd + " " + s + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func noEngineMsg() string {
	return theme.ErrorText.Render("No database connection. Is ~/.skrptiq/skrptiq.db accessible?")
}
