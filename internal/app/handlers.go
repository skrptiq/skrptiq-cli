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
// Returns true if the command was handled.
func handleSlashCommand(m *Model, cmd string, args string) (Model, tea.Cmd, bool) {
	switch cmd {
	case "help":
		m.repl.AddOutput(helpText())
		return *m, nil, true

	case "clear":
		m.repl = repl.NewWithPrompt(m.repl.Prompt(), m.commands)
		resizeView(m)
		return *m, m.repl.Init(), true

	case "list":
		handleList(m, args)
		return *m, nil, true

	case "show":
		handleShow(m, args)
		return *m, nil, true

	case "search":
		handleSearch(m, args)
		return *m, nil, true

	case "runs":
		handleRuns(m)
		return *m, nil, true

	case "profile":
		handleProfile(m, args)
		return *m, nil, true

	case "dials":
		handleDials(m)
		return *m, nil, true

	case "mcp":
		handleMCP(m)
		return *m, nil, true

	case "providers":
		handleProviders(m)
		return *m, nil, true

	case "workspace":
		handleWorkspace(m)
		return *m, nil, true

	case "tags":
		handleTags(m)
		return *m, nil, true

	case "config":
		handleConfig(m)
		return *m, nil, true

	case "hub":
		handleHub(m, args)
		return *m, nil, true
	}

	return *m, nil, false
}

func handleList(m *Model, args string) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return
	}

	nodeType := strings.TrimSpace(strings.ToLower(args))

	// Map plural/friendly names to node types.
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
			m.repl.AddOutput(theme.ErrorText.Render("Unknown node type: " + args))
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

func handleRuns(m *Model) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return
	}

	// The storage package doesn't have a ListExecutions yet — show what we can.
	m.repl.AddOutput(theme.Faint.Render("/runs — execution history display coming soon."))
}

func handleProfile(m *Model, args string) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return
	}

	arg := strings.TrimSpace(strings.ToLower(args))

	if arg == "list" || arg == "" {
		profiles, err := m.engine.Profiles()
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return
		}
		if len(profiles) == 0 {
			m.repl.AddOutput(theme.Faint.Render("No profiles configured."))
			return
		}
		var b strings.Builder
		typeStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(12)
		for _, p := range profiles {
			active := "  "
			if p.IsActive == 1 {
				active = theme.SuccessText.Render("● ")
			}
			b.WriteString(fmt.Sprintf("  %s%s %s\n", active, typeStyle.Render(p.Type), p.Name))
		}
		m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))
		return
	}

	// /profile use <name> — handled later with SetActiveProfile
	if strings.HasPrefix(arg, "use ") {
		name := strings.TrimSpace(args[4:])
		m.repl.AddOutput(theme.Faint.Render("Profile switching to \"" + name + "\" — coming soon."))
		return
	}

	m.repl.AddOutput(theme.Faint.Render("Usage: /profile [list] or /profile use <name>"))
}

func handleDials(m *Model) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return
	}

	// Dials are stored in profile metadata — show what we know.
	voice, _ := m.engine.ActiveProfile("voice")
	if voice == nil {
		m.repl.AddOutput(theme.Faint.Render("No active voice profile. Dials are configured per profile."))
		return
	}

	var b strings.Builder
	b.WriteString(theme.Title.Render("Persona Dials") + " — " + voice.Name + "\n")
	if voice.Metadata != nil && *voice.Metadata != "" {
		b.WriteString(theme.Faint.Render(*voice.Metadata))
	} else {
		b.WriteString(theme.Faint.Render("No dial configuration in profile metadata."))
	}
	m.repl.AddOutput(b.String())
}

func handleMCP(m *Model) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return
	}

	servers, err := m.engine.MCPServers()
	if err != nil {
		m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}

	if len(servers) == 0 {
		m.repl.AddOutput(theme.Faint.Render("No MCP servers configured."))
		return
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
}

func handleProviders(m *Model) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return
	}

	providers, err := m.engine.Providers()
	if err != nil {
		m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}

	if len(providers) == 0 {
		m.repl.AddOutput(theme.Faint.Render("No AI providers configured."))
		return
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
}

func handleWorkspace(m *Model) {
	var b strings.Builder
	b.WriteString(theme.Title.Render("Workspace") + "\n")
	b.WriteString("  " + theme.Faint.Render("Path: ") + m.statusBar.Workspace + "\n")
	b.WriteString("  " + theme.Faint.Render("Profile: ") + m.statusBar.Profile)
	if m.engine != nil {
		b.WriteString("\n  " + theme.Faint.Render("Database: ") + m.engine.DB.Path())
	}
	m.repl.AddOutput(b.String())
}

func handleTags(m *Model) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return
	}

	tags, err := m.engine.Tags()
	if err != nil {
		m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}

	if len(tags) == 0 {
		m.repl.AddOutput(theme.Faint.Render("No tags defined."))
		return
	}

	var b strings.Builder
	b.WriteString(theme.Title.Render("Tags") + "\n")
	for _, t := range tags {
		colour := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Colour))
		b.WriteString(fmt.Sprintf("  %s %s\n", colour.Render("●"), t.Name))
	}
	m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))
}

func handleConfig(m *Model) {
	var b strings.Builder
	b.WriteString(theme.Title.Render("Configuration") + "\n")

	if m.engine == nil {
		b.WriteString(theme.Faint.Render("  No engine connection."))
		m.repl.AddOutput(b.String())
		return
	}

	// Show key settings.
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
		b.WriteString(fmt.Sprintf("  %s %s\n", lipgloss.NewStyle().Foreground(theme.Muted).Width(20).Render(k.label+":"), val))
	}
	m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))
}

func handleHub(m *Model, args string) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return
	}

	arg := strings.TrimSpace(strings.ToLower(args))

	if arg == "" {
		// Show hub status — list imports.
		imports, err := m.engine.HubImports()
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return
		}

		var b strings.Builder
		b.WriteString(theme.Title.Render("Hub") + "\n")
		if len(imports) == 0 {
			b.WriteString(theme.Faint.Render("  No skrpts imported from Hub."))
		} else {
			b.WriteString(fmt.Sprintf("  %d imported skrpts:\n", len(imports)))
			for _, imp := range imports {
				ver := ""
				if imp.Version != nil {
					ver = theme.Faint.Render(" v" + *imp.Version)
				}
				b.WriteString(fmt.Sprintf("  %s%s\n", imp.Name, ver))
			}
		}
		m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))
		return
	}

	if strings.HasPrefix(arg, "search") || strings.HasPrefix(arg, "import") || strings.HasPrefix(arg, "update") {
		m.repl.AddOutput(theme.Faint.Render("/hub " + arg + " — requires Hub API client. Coming soon."))
		return
	}

	m.repl.AddOutput(theme.Faint.Render("Usage: /hub [search <query> | import <id> | update]"))
}

func noEngineMsg() string {
	return theme.ErrorText.Render("No database connection. Is ~/.skrptiq/skrptiq.db accessible?")
}
