package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/skrptiq/skrptiq-cli/internal/theme"
	"github.com/skrptiq/skrptiq-cli/internal/views/repl"
)

// handleSlashCommand processes implemented slash commands.
func handleSlashCommand(m *Model, cmd string, args string) (Model, tea.Cmd, bool) {
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
		handleList(m, args)
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
		return handleRuns(m, sub, subArgs)

	case "profile":
		return handleProfile(m, sub, subArgs)

	case "mcp":
		return handleMCPCmd(m, sub)

	case "providers":
		return handleProvidersCmd(m, sub)

	case "workspace":
		return handleWorkspaceCmd(m, sub, subArgs)

	case "tags":
		return handleTagsCmd(m, sub)

	case "tag":
		handleTagNode(m, args)
		return *m, nil, true

	case "untag":
		handleUntagNode(m, args)
		return *m, nil, true

	case "config":
		return handleConfigCmd(m, sub, subArgs)

	case "settings":
		return handleSettings(m, sub, subArgs)
	}

	return *m, nil, false
}

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
			m.repl.AddOutput(theme.ErrorText.Render("Unknown type: " + args + ". Try: workflows, skills, prompts, sources, documents, assets, services"))
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
		query := strings.TrimSpace(args)
		if query == "" {
			m.repl.AddOutput(theme.Faint.Render("Usage: /hub search <query>"))
			return *m, nil, true
		}
		results, err := m.engine.Hub.Search(query)
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Hub search error: " + err.Error()))
			return *m, nil, true
		}
		if len(results) == 0 {
			m.repl.AddOutput(theme.Faint.Render("No results for: " + query))
			return *m, nil, true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%d results for %q:\n", len(results), query))
		for _, r := range results {
			b.WriteString(fmt.Sprintf("  %s", theme.Bold.Render(r.Name)))
			if r.Category != "" {
				b.WriteString(theme.Faint.Render(" [" + r.Category + "]"))
			}
			b.WriteString("\n")
			if r.Description != "" {
				b.WriteString("    " + theme.Faint.Render(r.Description) + "\n")
			}
		}
		m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))

	case "import":
		slug := strings.TrimSpace(args)
		if slug == "" {
			m.repl.AddOutput(theme.Faint.Render("Usage: /hub import <slug>"))
			return *m, nil, true
		}
		// Look up the skrpt to confirm it exists.
		skrpt, err := m.engine.Hub.GetSkrpt(slug)
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Hub error: " + err.Error()))
			return *m, nil, true
		}
		if skrpt == nil {
			m.repl.AddOutput(theme.ErrorText.Render("Skrpt not found: " + slug))
			return *m, nil, true
		}
		m.repl.AddOutput(fmt.Sprintf("Found: %s v%s (%d nodes)\nImport download not yet wired — requires workspace file operations.",
			theme.Bold.Render(skrpt.Name), skrpt.Version, skrpt.NodeCount))

	case "update":
		imports, err := m.engine.HubImports()
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return *m, nil, true
		}
		if len(imports) == 0 {
			m.repl.AddOutput(theme.Faint.Render("No imported skrpts to update."))
			return *m, nil, true
		}
		var b strings.Builder
		b.WriteString(theme.Title.Render("Hub — Update Check") + "\n")
		for _, imp := range imports {
			versions, err := m.engine.Hub.GetVersions(imp.Slug)
			if err != nil {
				b.WriteString(fmt.Sprintf("  %s — %s\n", imp.Name, theme.ErrorText.Render("check failed")))
				continue
			}
			currentVer := "(unknown)"
			if imp.Version != nil {
				currentVer = *imp.Version
			}
			if len(versions) > 0 && versions[0].Version != currentVer {
				b.WriteString(fmt.Sprintf("  %s %s → %s\n",
					theme.WarningText.Render("⬆"),
					imp.Name,
					theme.Bold.Render(versions[0].Version)))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s %s\n",
					theme.SuccessText.Render("✓"),
					imp.Name,
					theme.Faint.Render("up to date")))
			}
		}
		m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))

	default:
		m.repl.AddOutput(usageBlock("/hub", []string{
			"list    — List imported skrpts",
			"search  — Search community skrpts",
			"import  — Import a skrpt from Hub",
			"update  — Check for updates",
		}))
	}
	return *m, nil, true
}

// --- /runs ---

func handleRuns(m *Model, sub, args string) (Model, tea.Cmd, bool) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return *m, nil, true
	}

	switch sub {
	case "list":
		runs, err := m.engine.ListExecutions(20)
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return *m, nil, true
		}
		if len(runs) == 0 {
			m.repl.AddOutput(theme.Faint.Render("No executions found."))
			return *m, nil, true
		}
		var b strings.Builder
		b.WriteString(theme.Title.Render("Recent Runs") + "\n")
		for _, r := range runs {
			// Show short ID for use with /runs show.
			shortID := r.ID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			b.WriteString(fmt.Sprintf("  %s  %s  %s  %s",
				statusIcon(r.Status),
				theme.Faint.Render(shortID),
				r.WorkflowTitle,
				theme.Faint.Render(r.StartedAt),
			))
			if r.TotalTokens > 0 {
				b.WriteString(theme.Faint.Render(fmt.Sprintf("  %d tokens", r.TotalTokens)))
			}
			if r.Error != nil && *r.Error != "" {
				b.WriteString("\n         " + theme.ErrorText.Render(*r.Error))
			}
			b.WriteString("\n")
		}
		m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))

	case "status":
		runs, err := m.engine.ListExecutions(10)
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return *m, nil, true
		}
		var active []string
		for _, r := range runs {
			if r.Status == "running" || r.Status == "paused" {
				active = append(active, fmt.Sprintf("  %s  %s  %s",
					statusIcon(r.Status), r.WorkflowTitle, theme.Faint.Render(r.StartedAt)))
			}
		}
		if len(active) == 0 {
			m.repl.AddOutput(theme.Faint.Render("No active executions."))
		} else {
			m.repl.AddOutput(theme.Title.Render("Active Runs") + "\n" + strings.Join(active, "\n"))
		}

	case "show":
		handleRunShow(m, args)

	default:
		m.repl.AddOutput(usageBlock("/runs", []string{
			"list         — List recent executions",
			"status       — Show active executions",
			"show <id>    — Show run details and step outputs",
		}))
	}
	return *m, nil, true
}

func handleRunShow(m *Model, args string) {
	idArg, stepArg := splitFirst(args)
	if idArg == "" {
		m.repl.AddOutput(theme.Faint.Render("Usage: /runs show <id> [step <n>]"))
		return
	}

	// Support short IDs by prefix match.
	fullID, err := m.engine.FindRunByPrefix(idArg)
	if err != nil || fullID == nil {
		m.repl.AddOutput(theme.ErrorText.Render("Run not found: " + idArg))
		return
	}

	run, err := m.engine.GetRunDetail(*fullID)
	if err != nil {
		m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}

	// If a step number was requested, show that step's output.
	if stepArg != "" {
		stepKey, _ := splitFirst(stepArg)
		if stepKey == "step" {
			_, stepNum := splitFirst(stepArg)
			if stepNum == "" {
				m.repl.AddOutput(theme.Faint.Render("Usage: /runs show <id> step <number>"))
				return
			}
			n := 0
			fmt.Sscanf(stepNum, "%d", &n)
			for _, s := range run.Steps {
				if s.Position == n {
					var b strings.Builder
					b.WriteString(theme.Title.Render(s.NodeTitle) + " — step " + fmt.Sprintf("%d", s.Position) + "\n")
					b.WriteString(theme.Faint.Render("Status: ") + statusIcon(s.Status) + " " + s.Status + "\n")
					if s.Provider != "" {
						b.WriteString(theme.Faint.Render("Provider: ") + s.Provider)
						if s.Model != "" {
							b.WriteString(" / " + s.Model)
						}
						b.WriteString("\n")
					}
					if s.Duration != "" {
						b.WriteString(theme.Faint.Render("Duration: ") + s.Duration + "\n")
					}
					if s.Error != "" {
						b.WriteString(theme.ErrorText.Render("Error: ") + s.Error + "\n")
					}
					if s.Output != "" {
						b.WriteString("\n" + s.Output)
					} else {
						b.WriteString(theme.Faint.Render("\n(no output)"))
					}
					m.repl.AddOutput(b.String())
					return
				}
			}
			m.repl.AddOutput(theme.ErrorText.Render(fmt.Sprintf("Step %d not found in this run.", n)))
			return
		}
	}

	// Show full run summary.
	var b strings.Builder
	b.WriteString(theme.Title.Render(run.WorkflowTitle) + "\n")
	b.WriteString(theme.Faint.Render("ID: ") + run.ID + "\n")
	b.WriteString(theme.Faint.Render("Status: ") + statusIcon(run.Status) + " " + run.Status + "\n")
	b.WriteString(theme.Faint.Render("Started: ") + run.StartedAt + "\n")
	if run.CompletedAt != nil {
		b.WriteString(theme.Faint.Render("Completed: ") + *run.CompletedAt + "\n")
	}
	if run.TotalTokens > 0 {
		b.WriteString(theme.Faint.Render("Tokens: ") + fmt.Sprintf("%d", run.TotalTokens) + "\n")
	}
	if run.Error != nil && *run.Error != "" {
		b.WriteString(theme.ErrorText.Render("Error: ") + *run.Error + "\n")
	}

	if len(run.Steps) > 0 {
		b.WriteString("\n" + theme.Bold.Render("Steps") + "\n")
		for _, s := range run.Steps {
			b.WriteString(fmt.Sprintf("  %s %d. %s", statusIcon(s.Status), s.Position, s.NodeTitle))
			if s.Provider != "" {
				b.WriteString(theme.Faint.Render(" (" + s.Provider + ")"))
			}
			if s.Duration != "" {
				b.WriteString(theme.Faint.Render(" " + s.Duration))
			}
			outLen := len(s.Output)
			if outLen > 0 {
				b.WriteString(theme.Faint.Render(fmt.Sprintf(" %d chars", outLen)))
			}
			b.WriteString("\n")
			if s.Error != "" {
				b.WriteString("    " + theme.ErrorText.Render(s.Error) + "\n")
			}
		}
		b.WriteString(theme.Faint.Render("\nUse /runs show " + run.ID[:8] + " step <n> to view step output."))
	}

	m.repl.AddOutput(b.String())
}

func statusIcon(status string) string {
	switch status {
	case "completed":
		return theme.SuccessText.Render("✓")
	case "failed":
		return theme.ErrorText.Render("✗")
	case "running":
		return theme.Subtitle.Render("◌")
	case "paused":
		return theme.WarningText.Render("⏸")
	default:
		return theme.Faint.Render("○")
	}
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
		profile, err := m.engine.FindProfileByName(name)
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return *m, nil, true
		}
		if profile == nil {
			m.repl.AddOutput(theme.ErrorText.Render("Profile not found: " + name))
			return *m, nil, true
		}
		if err := m.engine.SetActiveProfile(profile.ID, profile.Type); err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return *m, nil, true
		}
		m.repl.AddOutput(theme.SuccessText.Render("Switched to profile: ") + profile.Name)
		cfg := m.repl.Prompt()
		cfg.ContextLeft = profile.Name
		m.repl.SetPrompt(cfg)
		m.statusBar.Profile = profile.Name

	case "controls":
		voice, _ := m.engine.ActiveProfile("voice")
		if voice == nil {
			m.repl.AddOutput(theme.Faint.Render("No active voice profile. Controls are configured per profile."))
			return *m, nil, true
		}
		var b strings.Builder
		b.WriteString(theme.Title.Render("Quality Controls") + " — " + voice.Name + "\n")
		if voice.Metadata != nil && *voice.Metadata != "" {
			// Try to pretty-print the JSON metadata.
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(*voice.Metadata), &parsed); err == nil {
				for k, v := range parsed {
					b.WriteString(fmt.Sprintf("  %s %v\n",
						lipgloss.NewStyle().Foreground(theme.Muted).Width(20).Render(k+":"), v))
				}
			} else {
				b.WriteString(theme.Faint.Render(*voice.Metadata))
			}
		} else {
			b.WriteString(theme.Faint.Render("No control settings in profile metadata."))
		}
		m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))

	default:
		m.repl.AddOutput(usageBlock("/profile", []string{
			"list      — List all profiles",
			"show      — Show active profile details",
			"use       — Switch active profile",
			"controls  — Show quality control settings",
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

	case "tools":
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
		b.WriteString(theme.Title.Render("MCP Tools") + "\n")
		for _, s := range servers {
			b.WriteString("\n  " + theme.Bold.Render(s.Name) + "\n")
			if s.Capabilities != nil && *s.Capabilities != "" {
				var caps []string
				if err := json.Unmarshal([]byte(*s.Capabilities), &caps); err == nil {
					for _, cap := range caps {
						b.WriteString("    " + cap + "\n")
					}
				} else {
					b.WriteString("    " + theme.Faint.Render(*s.Capabilities) + "\n")
				}
			} else {
				b.WriteString("    " + theme.Faint.Render("No tools listed — connect to discover.") + "\n")
			}
		}
		m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))

	case "connect":
		m.repl.AddOutput(theme.Faint.Render("/mcp connect — requires MCP client runtime. Needs engine execution wiring."))

	case "disconnect":
		m.repl.AddOutput(theme.Faint.Render("/mcp disconnect — requires MCP client runtime. Needs engine execution wiring."))

	default:
		m.repl.AddOutput(usageBlock("/mcp", []string{
			"list        — List server connections",
			"tools       — List available tools",
			"connect     — Connect to a server",
			"disconnect  — Disconnect a server",
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
		// Also check CLI-detected providers.
		allConns, _ := m.engine.Connections()
		var llmConns []struct{ name, provider, status string }
		for _, c := range allConns {
			if c.Type == "llm-provider" {
				llmConns = append(llmConns, struct{ name, provider, status string }{c.Name, c.Provider, c.Status})
			}
		}
		if len(providers) == 0 && len(llmConns) == 0 {
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
		m.repl.AddOutput(theme.Faint.Render("/providers add — requires interactive provider setup flow. Needs engine execution wiring."))

	default:
		m.repl.AddOutput(usageBlock("/providers", []string{
			"list    — List configured providers",
			"add     — Configure a new provider",
		}))
	}
	return *m, nil, true
}

// --- /workspace ---

func handleWorkspaceCmd(m *Model, sub, args string) (Model, tea.Cmd, bool) {
	switch sub {
	case "show":
		var b strings.Builder
		b.WriteString(theme.Title.Render("Workspace") + "\n")
		cwd, _ := os.Getwd()
		b.WriteString("  " + theme.Faint.Render("Path: ") + cwd + "\n")
		b.WriteString("  " + theme.Faint.Render("Profile: ") + m.statusBar.Profile)
		if m.engine != nil {
			b.WriteString("\n  " + theme.Faint.Render("Database: ") + m.engine.DB.Path())
		}
		m.repl.AddOutput(b.String())

	case "set":
		path := strings.TrimSpace(args)
		if path == "" {
			m.repl.AddOutput(theme.Faint.Render("Usage: /workspace set <path>"))
			return *m, nil, true
		}
		// Expand ~ to home directory.
		if strings.HasPrefix(path, "~") {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, path[1:])
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Invalid path: " + err.Error()))
			return *m, nil, true
		}
		info, err := os.Stat(absPath)
		if err != nil || !info.IsDir() {
			m.repl.AddOutput(theme.ErrorText.Render("Not a directory: " + absPath))
			return *m, nil, true
		}
		if err := os.Chdir(absPath); err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return *m, nil, true
		}
		// Update status bar.
		home, _ := os.UserHomeDir()
		display := absPath
		if home != "" && strings.HasPrefix(absPath, home) {
			display = "~" + absPath[len(home):]
		}
		m.statusBar.Workspace = display
		m.repl.AddOutput(theme.SuccessText.Render("Workspace: ") + display)

	default:
		m.repl.AddOutput(usageBlock("/workspace", []string{
			"show    — Show current context",
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

// --- /tag and /untag ---

func handleTagNode(m *Model, args string) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return
	}
	// Expect: <node title> <tag name> — but both can have spaces.
	// Use the tag list to find the tag name at the end.
	args = strings.TrimSpace(args)
	if args == "" {
		m.repl.AddOutput(theme.Faint.Render("Usage: /tag <node title> <tag name>"))
		return
	}

	tags, err := m.engine.Tags()
	if err != nil {
		m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}

	// Try to match tag name from the end of the string.
	var matchedTag string
	var matchedTagID string
	var nodeTitle string
	for _, t := range tags {
		if strings.HasSuffix(strings.ToLower(args), strings.ToLower(t.Name)) {
			matchedTag = t.Name
			matchedTagID = t.ID
			nodeTitle = strings.TrimSpace(args[:len(args)-len(t.Name)])
			break
		}
	}
	if matchedTag == "" {
		m.repl.AddOutput(theme.ErrorText.Render("Could not identify tag name. Available tags: /tags list"))
		return
	}

	node, err := m.engine.FindNodeByTitle(nodeTitle)
	if err != nil || node == nil {
		m.repl.AddOutput(theme.ErrorText.Render("Node not found: " + nodeTitle))
		return
	}

	if err := m.engine.DB.AssignTag(node.ID, matchedTagID); err != nil {
		m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}
	m.repl.AddOutput(theme.SuccessText.Render("Tagged ") + node.Title + " with " + matchedTag)
}

func handleUntagNode(m *Model, args string) {
	if m.engine == nil {
		m.repl.AddOutput(noEngineMsg())
		return
	}
	args = strings.TrimSpace(args)
	if args == "" {
		m.repl.AddOutput(theme.Faint.Render("Usage: /untag <node title> <tag name>"))
		return
	}

	tags, err := m.engine.Tags()
	if err != nil {
		m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}

	var matchedTag string
	var matchedTagID string
	var nodeTitle string
	for _, t := range tags {
		if strings.HasSuffix(strings.ToLower(args), strings.ToLower(t.Name)) {
			matchedTag = t.Name
			matchedTagID = t.ID
			nodeTitle = strings.TrimSpace(args[:len(args)-len(t.Name)])
			break
		}
	}
	if matchedTag == "" {
		m.repl.AddOutput(theme.ErrorText.Render("Could not identify tag name. Available tags: /tags list"))
		return
	}

	node, err := m.engine.FindNodeByTitle(nodeTitle)
	if err != nil || node == nil {
		m.repl.AddOutput(theme.ErrorText.Render("Node not found: " + nodeTitle))
		return
	}

	if err := m.engine.DB.UnassignTag(node.ID, matchedTagID); err != nil {
		m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}
	m.repl.AddOutput(theme.SuccessText.Render("Removed ") + matchedTag + " from " + node.Title)
}

// --- /config ---

func handleConfigCmd(m *Model, sub, args string) (Model, tea.Cmd, bool) {
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
		if m.engine == nil {
			m.repl.AddOutput(noEngineMsg())
			return *m, nil, true
		}
		parts := strings.SplitN(args, " ", 2)
		if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" {
			m.repl.AddOutput(theme.Faint.Render("Usage: /config set <key> <value>"))
			return *m, nil, true
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if err := m.engine.DB.SetSetting(key, value); err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return *m, nil, true
		}
		m.repl.AddOutput(theme.SuccessText.Render("Set ") + key + " = " + value)

	default:
		m.repl.AddOutput(usageBlock("/config", []string{
			"show    — Show current configuration",
			"set     — Update a configuration value",
		}))
	}
	return *m, nil, true
}

// --- /settings ---

func handleSettings(m *Model, sub, args string) (Model, tea.Cmd, bool) {
	switch sub {
	case "about":
		var b strings.Builder
		b.WriteString(theme.Title.Render("skrptiq") + " v0.1.0-prototype\n")
		b.WriteString(theme.Faint.Render("Interactive terminal for personalised AI agents\n"))
		b.WriteString(theme.Faint.Render("Engine: "))
		if m.engine != nil {
			b.WriteString(m.engine.DB.Path())
		} else {
			b.WriteString("not connected")
		}
		b.WriteString("\n")
		cwd, _ := os.Getwd()
		b.WriteString(theme.Faint.Render("Working directory: ") + cwd + "\n")
		b.WriteString(theme.Faint.Render("Platform: ") + "darwin/arm64")
		m.repl.AddOutput(b.String())

	case "providers":
		// Delegate to existing providers handler.
		return handleProvidersCmd(m, "list")

	case "connections":
		if m.engine == nil {
			m.repl.AddOutput(noEngineMsg())
			return *m, nil, true
		}
		conns, err := m.engine.Connections()
		if err != nil {
			m.repl.AddOutput(theme.ErrorText.Render("Error: " + err.Error()))
			return *m, nil, true
		}
		if len(conns) == 0 {
			m.repl.AddOutput(theme.Faint.Render("No connections configured."))
			return *m, nil, true
		}
		var b strings.Builder
		b.WriteString(theme.Title.Render("Connections") + "\n")
		typeStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(16)
		for _, c := range conns {
			indicator := theme.ErrorText.Render("●")
			if c.Status == "connected" {
				indicator = theme.SuccessText.Render("●")
			}
			b.WriteString(fmt.Sprintf("  %s %s %s", indicator, typeStyle.Render(c.Type), c.Name))
			if c.Provider != "" {
				b.WriteString(theme.Faint.Render(" (" + c.Provider + ")"))
			}
			b.WriteString("\n")
		}
		m.repl.AddOutput(strings.TrimRight(b.String(), "\n"))

	case "config":
		return handleConfigCmd(m, "show", "")

	case "set":
		return handleConfigCmd(m, "set", args)

	default:
		m.repl.AddOutput(usageBlock("/settings", []string{
			"about        — Version and system info",
			"providers    — AI provider configuration",
			"connections  — All connections (providers, MCP, services)",
			"config       — Show configuration values",
			"set          — Update a configuration value",
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
