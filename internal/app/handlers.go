package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/skrptiq/skrptiq-cli/internal/theme"
)

// handleSlashCommand processes slash commands. Returns true if handled.
func (a *App) handleSlashCommand(cmd string, args string) bool {
	sub, subArgs := splitFirst(args)

	switch cmd {
	case "help":
		a.Print(helpText())
	case "chat":
		a.handleEnterChat(args)
	case "command":
		if a.mode != ModeCommand {
			a.Print(theme.Faint.Render("Exited " + a.mode.Label() + " mode."))
			a.setMode(ModeCommand)
		} else {
			a.Print(theme.Faint.Render("Already in command mode."))
		}
	case "exit":
		if a.mode != ModeCommand {
			a.Print(theme.Faint.Render("Exited " + a.mode.Label() + " mode."))
			a.setMode(ModeCommand)
		} else {
			a.Print(theme.Faint.Render("Already in command mode. Use ctrl+d to exit skrptiq."))
		}
	case "run":
		a.handleEnterRun(args)
	case "clear":
		fmt.Print("\033[2J\033[H") // ANSI clear screen
	case "list":
		a.handleList(args)
	case "show":
		a.handleShow(args)
	case "search":
		a.handleSearch(args)
	case "hub":
		a.handleHub(sub, subArgs)
	case "runs":
		a.handleRuns(sub, subArgs)
	case "profile":
		a.handleProfile(sub, subArgs)
	case "mcp":
		a.handleMCPCmd(sub)
	case "providers":
		a.handleProvidersCmd(sub)
	case "workspace":
		a.handleWorkspaceCmd(sub, subArgs)
	case "tags":
		a.handleTagsCmd(sub)
	case "tag":
		a.handleTagNode(args)
	case "untag":
		a.handleUntagNode(args)
	case "config":
		a.handleConfigCmd(sub, subArgs)
	case "settings":
		a.handleSettings(sub, subArgs)
	default:
		return false
	}
	return true
}

func splitFirst(s string) (string, string) {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, " "); idx > 0 {
		return strings.ToLower(s[:idx]), strings.TrimSpace(s[idx+1:])
	}
	return strings.ToLower(s), ""
}

// --- /list ---

func (a *App) handleList(args string) {
	if a.engine == nil {
		a.Print(noEngineMsg())
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
		all, e := a.engine.DB.GetAllNodes()
		err = e
		for _, n := range all {
			nodes = append(nodes, struct{ Title, Type string }{n.Title, n.Type})
		}
	} else {
		mapped, ok := typeMap[nodeType]
		if !ok {
			a.Print(theme.ErrorText.Render("Unknown type: " + args + ". Try: workflows, skills, prompts, sources, documents, assets, services"))
			return
		}
		filtered, e := a.engine.NodesByType(mapped)
		err = e
		for _, n := range filtered {
			nodes = append(nodes, struct{ Title, Type string }{n.Title, n.Type})
		}
	}
	if err != nil {
		a.Print(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}
	if len(nodes) == 0 {
		a.Print(theme.Faint.Render("No nodes found."))
		return
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type < nodes[j].Type
		}
		return strings.ToLower(nodes[i].Title) < strings.ToLower(nodes[j].Title)
	})
	typeFilter := ""
	if nodeType != "" {
		typeFilter = " (" + nodeType + ")"
	}
	a.Print(fmt.Sprintf("%s%s — %d nodes", theme.Title.Render("Nodes"), typeFilter, len(nodes)))
	typeStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(12)
	for _, n := range nodes {
		a.Print(fmt.Sprintf("  %s %s", typeStyle.Render(n.Type), n.Title))
	}
}

// --- /show ---

func (a *App) handleShow(args string) {
	if a.engine == nil { a.Print(noEngineMsg()); return }
	title := strings.TrimSpace(args)
	if title == "" { a.Print(theme.Faint.Render("Usage: /show <node name>")); return }
	node, err := a.engine.FindNodeByTitle(title)
	if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
	if node == nil { a.Print(theme.Faint.Render("No node found: " + title)); return }
	a.Print(theme.Title.Render(node.Title))
	a.Print(theme.Faint.Render("Type: ") + node.Type)
	if node.Description != nil && *node.Description != "" {
		a.Print(theme.Faint.Render("Description: ") + *node.Description)
	}
	if node.Content != nil && *node.Content != "" {
		a.Print("\n" + *node.Content)
	}
}

// --- /search ---

func (a *App) handleSearch(args string) {
	if a.engine == nil { a.Print(noEngineMsg()); return }
	query := strings.TrimSpace(args)
	if query == "" { a.Print(theme.Faint.Render("Usage: /search <query>")); return }
	nodes, err := a.engine.SearchNodes(query)
	if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
	if len(nodes) == 0 { a.Print(theme.Faint.Render("No results for: " + query)); return }
	a.Print(fmt.Sprintf("%d results for %q:", len(nodes), query))
	typeStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(12)
	for _, n := range nodes {
		a.Print(fmt.Sprintf("  %s %s", typeStyle.Render(n.Type), n.Title))
	}
}

// --- /hub ---

func (a *App) handleHub(sub, args string) {
	if a.engine == nil { a.Print(noEngineMsg()); return }
	switch sub {
	case "list":
		imports, err := a.engine.HubImports()
		if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		a.Print(theme.Title.Render("Hub — Imported Skrpts"))
		if len(imports) == 0 {
			a.Print(theme.Faint.Render("  No skrpts imported from Hub."))
		} else {
			for _, imp := range imports {
				ver := ""
				if imp.Version != nil { ver = theme.Faint.Render(" v" + *imp.Version) }
				a.Print(fmt.Sprintf("  %s%s", imp.Name, ver))
			}
		}
	case "search":
		query := strings.TrimSpace(args)
		if query == "" { a.Print(theme.Faint.Render("Usage: /hub search <query>")); return }
		results, err := a.engine.Hub.Search(query)
		if err != nil { a.Print(theme.ErrorText.Render("Hub search error: " + err.Error())); return }
		if len(results) == 0 { a.Print(theme.Faint.Render("No results for: " + query)); return }
		a.Print(fmt.Sprintf("%d results for %q:", len(results), query))
		for _, r := range results {
			line := fmt.Sprintf("  %s", theme.Bold.Render(r.Name))
			if r.Category != "" { line += theme.Faint.Render(" [" + r.Category + "]") }
			a.Print(line)
			if r.Description != "" { a.Print("    " + theme.Faint.Render(r.Description)) }
		}
	case "import":
		slug := strings.TrimSpace(args)
		if slug == "" { a.Print(theme.Faint.Render("Usage: /hub import <slug>")); return }
		skrpt, err := a.engine.Hub.GetSkrpt(slug)
		if err != nil { a.Print(theme.ErrorText.Render("Hub error: " + err.Error())); return }
		if skrpt == nil { a.Print(theme.ErrorText.Render("Skrpt not found: " + slug)); return }
		a.Print(fmt.Sprintf("Found: %s v%s (%d nodes)\nImport download not yet wired.", theme.Bold.Render(skrpt.Name), skrpt.Version, skrpt.NodeCount))
	case "update":
		imports, err := a.engine.HubImports()
		if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(imports) == 0 { a.Print(theme.Faint.Render("No imported skrpts to update.")); return }
		a.Print(theme.Title.Render("Hub — Update Check"))
		for _, imp := range imports {
			versions, err := a.engine.Hub.GetVersions(imp.Slug)
			if err != nil { a.Print(fmt.Sprintf("  %s — %s", imp.Name, theme.ErrorText.Render("check failed"))); continue }
			currentVer := "(unknown)"
			if imp.Version != nil { currentVer = *imp.Version }
			if len(versions) > 0 && versions[0].Version != currentVer {
				a.Print(fmt.Sprintf("  %s %s → %s", theme.WarningText.Render("⬆"), imp.Name, theme.Bold.Render(versions[0].Version)))
			} else {
				a.Print(fmt.Sprintf("  %s %s %s", theme.SuccessText.Render("✓"), imp.Name, theme.Faint.Render("up to date")))
			}
		}
	default:
		a.Print(usageBlock("/hub", []string{"list", "search", "import", "update"}))
	}
}

// --- /runs ---

func (a *App) handleRuns(sub, args string) {
	if a.engine == nil { a.Print(noEngineMsg()); return }
	switch sub {
	case "list":
		runs, err := a.engine.ListExecutions(20)
		if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(runs) == 0 { a.Print(theme.Faint.Render("No executions found.")); return }
		a.Print(theme.Title.Render("Recent Runs"))
		for _, r := range runs {
			shortID := r.ID
			if len(shortID) > 8 { shortID = shortID[:8] }
			line := fmt.Sprintf("  %s  %s  %s  %s", statusIcon(r.Status), theme.Faint.Render(shortID), r.WorkflowTitle, theme.Faint.Render(r.StartedAt))
			if r.TotalTokens > 0 { line += theme.Faint.Render(fmt.Sprintf("  %d tokens", r.TotalTokens)) }
			a.Print(line)
			if r.Error != nil && *r.Error != "" { a.Print("         " + theme.ErrorText.Render(*r.Error)) }
		}
	case "status":
		runs, err := a.engine.ListExecutions(10)
		if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		var active []string
		for _, r := range runs {
			if r.Status == "running" || r.Status == "paused" {
				active = append(active, fmt.Sprintf("  %s  %s  %s", statusIcon(r.Status), r.WorkflowTitle, theme.Faint.Render(r.StartedAt)))
			}
		}
		if len(active) == 0 { a.Print(theme.Faint.Render("No active executions.")) } else {
			a.Print(theme.Title.Render("Active Runs"))
			for _, line := range active { a.Print(line) }
		}
	case "show":
		a.handleRunShow(args)
	default:
		a.Print(usageBlock("/runs", []string{"list", "status", "show <id>"}))
	}
}

func (a *App) handleRunShow(args string) {
	idArg, stepArg := splitFirst(args)
	if idArg == "" { a.Print(theme.Faint.Render("Usage: /runs show <id> [step <n>]")); return }
	fullID, err := a.engine.FindRunByPrefix(idArg)
	if err != nil || fullID == nil { a.Print(theme.ErrorText.Render("Run not found: " + idArg)); return }
	run, err := a.engine.GetRunDetail(*fullID)
	if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }

	if stepArg != "" {
		stepKey, stepNum := splitFirst(stepArg)
		if stepKey == "step" && stepNum != "" {
			n := 0
			fmt.Sscanf(stepNum, "%d", &n)
			for _, s := range run.Steps {
				if s.Position == n {
					a.Print(theme.Title.Render(s.NodeTitle) + " — step " + fmt.Sprintf("%d", s.Position))
					a.Print(theme.Faint.Render("Status: ") + statusIcon(s.Status) + " " + s.Status)
					if s.Provider != "" {
						p := s.Provider
						if s.Model != "" { p += " / " + s.Model }
						a.Print(theme.Faint.Render("Provider: ") + p)
					}
					if s.Duration != "" { a.Print(theme.Faint.Render("Duration: ") + s.Duration) }
					if s.Error != "" { a.Print(theme.ErrorText.Render("Error: ") + s.Error) }
					if s.Output != "" { a.Print("\n" + s.Output) } else { a.Print(theme.Faint.Render("\n(no output)")) }
					return
				}
			}
			a.Print(theme.ErrorText.Render(fmt.Sprintf("Step %d not found.", n)))
			return
		}
	}

	a.Print(theme.Title.Render(run.WorkflowTitle))
	a.Print(theme.Faint.Render("ID: ") + run.ID)
	a.Print(theme.Faint.Render("Status: ") + statusIcon(run.Status) + " " + run.Status)
	a.Print(theme.Faint.Render("Started: ") + run.StartedAt)
	if run.CompletedAt != nil { a.Print(theme.Faint.Render("Completed: ") + *run.CompletedAt) }
	if run.TotalTokens > 0 { a.Print(theme.Faint.Render("Tokens: ") + fmt.Sprintf("%d", run.TotalTokens)) }
	if run.Error != nil && *run.Error != "" { a.Print(theme.ErrorText.Render("Error: ") + *run.Error) }
	if len(run.Steps) > 0 {
		a.Print("\n" + theme.Bold.Render("Steps"))
		for _, s := range run.Steps {
			line := fmt.Sprintf("  %s %d. %s", statusIcon(s.Status), s.Position, s.NodeTitle)
			if s.Provider != "" { line += theme.Faint.Render(" (" + s.Provider + ")") }
			if s.Duration != "" { line += theme.Faint.Render(" " + s.Duration) }
			if len(s.Output) > 0 { line += theme.Faint.Render(fmt.Sprintf(" %d chars", len(s.Output))) }
			a.Print(line)
		}
		a.Print(theme.Faint.Render("\nUse /runs show " + run.ID[:8] + " step <n> to view step output."))
	}
}

func statusIcon(status string) string {
	switch status {
	case "completed": return theme.SuccessText.Render("✓")
	case "failed": return theme.ErrorText.Render("✗")
	case "running": return theme.Subtitle.Render("◌")
	case "paused": return theme.WarningText.Render("⏸")
	default: return theme.Faint.Render("○")
	}
}

// --- /profile ---

func (a *App) handleProfile(sub, args string) {
	if a.engine == nil { a.Print(noEngineMsg()); return }
	switch sub {
	case "list":
		profiles, err := a.engine.Profiles()
		if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(profiles) == 0 { a.Print(theme.Faint.Render("No profiles configured.")); return }
		a.Print(theme.Title.Render("Profiles"))
		typeStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(12)
		for _, p := range profiles {
			active := "  "
			if p.IsActive == 1 { active = theme.SuccessText.Render("● ") }
			a.Print(fmt.Sprintf("  %s%s %s", active, typeStyle.Render(p.Type), p.Name))
		}
	case "show":
		voice, _ := a.engine.ActiveProfile("voice")
		if voice == nil { a.Print(theme.Faint.Render("No active voice profile.")); return }
		a.Print(theme.Title.Render(voice.Name))
		a.Print(theme.Faint.Render("Type: ") + voice.Type)
		if voice.Content != "" { a.Print("\n" + voice.Content) }
	case "use":
		name := strings.TrimSpace(args)
		if name == "" { a.Print(theme.Faint.Render("Usage: /profile use <name>")); return }
		profile, err := a.engine.FindProfileByName(name)
		if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if profile == nil { a.Print(theme.ErrorText.Render("Profile not found: " + name)); return }
		if err := a.engine.SetActiveProfile(profile.ID, profile.Type); err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		a.Print(theme.SuccessText.Render("Switched to profile: ") + profile.Name)
		a.updatePrompt()
	case "controls":
		voice, _ := a.engine.ActiveProfile("voice")
		if voice == nil { a.Print(theme.Faint.Render("No active voice profile.")); return }
		a.Print(theme.Title.Render("Quality Controls") + " — " + voice.Name)
		if voice.Metadata != nil && *voice.Metadata != "" {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(*voice.Metadata), &parsed); err == nil {
				for k, v := range parsed {
					a.Print(fmt.Sprintf("  %s %v", lipgloss.NewStyle().Foreground(theme.Muted).Width(20).Render(k+":"), v))
				}
			} else {
				a.Print(theme.Faint.Render(*voice.Metadata))
			}
		} else {
			a.Print(theme.Faint.Render("No control settings in profile metadata."))
		}
	default:
		a.Print(usageBlock("/profile", []string{"list", "show", "use", "controls"}))
	}
}

// --- /mcp ---

func (a *App) handleMCPCmd(sub string) {
	if a.engine == nil { a.Print(noEngineMsg()); return }
	switch sub {
	case "list":
		servers, err := a.engine.MCPServers()
		if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(servers) == 0 { a.Print(theme.Faint.Render("No MCP servers configured.")); return }
		a.Print(theme.Title.Render("MCP Servers"))
		for _, s := range servers {
			indicator := theme.ErrorText.Render("●")
			if s.Status == "connected" { indicator = theme.SuccessText.Render("●") }
			line := fmt.Sprintf("  %s %s", indicator, s.Name)
			if s.Provider != "" { line += theme.Faint.Render(" (" + s.Provider + ")") }
			a.Print(line)
		}
	case "tools":
		servers, err := a.engine.MCPServers()
		if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(servers) == 0 { a.Print(theme.Faint.Render("No MCP servers configured.")); return }
		a.Print(theme.Title.Render("MCP Tools"))
		for _, s := range servers {
			a.Print("\n  " + theme.Bold.Render(s.Name))
			if s.Capabilities != nil && *s.Capabilities != "" {
				var caps []string
				if err := json.Unmarshal([]byte(*s.Capabilities), &caps); err == nil {
					for _, cap := range caps { a.Print("    " + cap) }
				} else { a.Print("    " + theme.Faint.Render(*s.Capabilities)) }
			} else { a.Print("    " + theme.Faint.Render("No tools listed.")) }
		}
	case "connect": a.Print(theme.Faint.Render("/mcp connect — requires MCP client runtime."))
	case "disconnect": a.Print(theme.Faint.Render("/mcp disconnect — requires MCP client runtime."))
	default:
		a.Print(usageBlock("/mcp", []string{"list", "tools", "connect", "disconnect"}))
	}
}

// --- /providers ---

func (a *App) handleProvidersCmd(sub string) {
	if a.engine == nil { a.Print(noEngineMsg()); return }
	switch sub {
	case "list":
		providers, err := a.engine.Providers()
		if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(providers) == 0 { a.Print(theme.Faint.Render("No AI providers configured.")); return }
		a.Print(theme.Title.Render("AI Providers"))
		for _, p := range providers {
			indicator := theme.ErrorText.Render("●")
			if p.Status == "connected" { indicator = theme.SuccessText.Render("●") }
			line := fmt.Sprintf("  %s %s", indicator, p.Name)
			if p.Provider != "" { line += theme.Faint.Render(" (" + p.Provider + ")") }
			a.Print(line)
		}
	case "add": a.Print(theme.Faint.Render("/providers add — requires interactive setup."))
	default:
		a.Print(usageBlock("/providers", []string{"list", "add"}))
	}
}

// --- /workspace ---

func (a *App) handleWorkspaceCmd(sub, args string) {
	switch sub {
	case "show":
		a.Print(theme.Title.Render("Workspace"))
		cwd, _ := os.Getwd()
		a.Print("  " + theme.Faint.Render("Path: ") + cwd)
		if a.engine != nil { a.Print("  " + theme.Faint.Render("Database: ") + a.engine.DB.Path()) }
	case "set":
		path := strings.TrimSpace(args)
		if path == "" { a.Print(theme.Faint.Render("Usage: /workspace set <path>")); return }
		if strings.HasPrefix(path, "~") {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, path[1:])
		}
		absPath, err := filepath.Abs(path)
		if err != nil { a.Print(theme.ErrorText.Render("Invalid path: " + err.Error())); return }
		info, err := os.Stat(absPath)
		if err != nil || !info.IsDir() { a.Print(theme.ErrorText.Render("Not a directory: " + absPath)); return }
		if err := os.Chdir(absPath); err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		a.Print(theme.SuccessText.Render("Workspace: ") + absPath)
	default:
		a.Print(usageBlock("/workspace", []string{"show", "set"}))
	}
}

// --- /tags ---

func (a *App) handleTagsCmd(sub string) {
	if a.engine == nil { a.Print(noEngineMsg()); return }
	switch sub {
	case "list":
		tags, err := a.engine.Tags()
		if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(tags) == 0 { a.Print(theme.Faint.Render("No tags defined.")); return }
		a.Print(theme.Title.Render("Tags"))
		for _, t := range tags {
			colour := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Colour))
			a.Print(fmt.Sprintf("  %s %s", colour.Render("●"), t.Name))
		}
	default:
		a.Print(usageBlock("/tags", []string{"list"}))
	}
}

// --- /tag and /untag ---

func (a *App) handleTagNode(args string) {
	if a.engine == nil { a.Print(noEngineMsg()); return }
	args = strings.TrimSpace(args)
	if args == "" { a.Print(theme.Faint.Render("Usage: /tag <node title> <tag name>")); return }
	tags, err := a.engine.Tags()
	if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
	var matchedTag, matchedTagID, nodeTitle string
	for _, t := range tags {
		if strings.HasSuffix(strings.ToLower(args), strings.ToLower(t.Name)) {
			matchedTag = t.Name
			matchedTagID = t.ID
			nodeTitle = strings.TrimSpace(args[:len(args)-len(t.Name)])
			break
		}
	}
	if matchedTag == "" { a.Print(theme.ErrorText.Render("Could not identify tag. Use /tags list.")); return }
	node, err := a.engine.FindNodeByTitle(nodeTitle)
	if err != nil || node == nil { a.Print(theme.ErrorText.Render("Node not found: " + nodeTitle)); return }
	if err := a.engine.DB.AssignTag(node.ID, matchedTagID); err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
	a.Print(theme.SuccessText.Render("Tagged ") + node.Title + " with " + matchedTag)
}

func (a *App) handleUntagNode(args string) {
	if a.engine == nil { a.Print(noEngineMsg()); return }
	args = strings.TrimSpace(args)
	if args == "" { a.Print(theme.Faint.Render("Usage: /untag <node title> <tag name>")); return }
	tags, err := a.engine.Tags()
	if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
	var matchedTag, matchedTagID, nodeTitle string
	for _, t := range tags {
		if strings.HasSuffix(strings.ToLower(args), strings.ToLower(t.Name)) {
			matchedTag = t.Name
			matchedTagID = t.ID
			nodeTitle = strings.TrimSpace(args[:len(args)-len(t.Name)])
			break
		}
	}
	if matchedTag == "" { a.Print(theme.ErrorText.Render("Could not identify tag. Use /tags list.")); return }
	node, err := a.engine.FindNodeByTitle(nodeTitle)
	if err != nil || node == nil { a.Print(theme.ErrorText.Render("Node not found: " + nodeTitle)); return }
	if err := a.engine.DB.UnassignTag(node.ID, matchedTagID); err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
	a.Print(theme.SuccessText.Render("Removed ") + matchedTag + " from " + node.Title)
}

// --- /config ---

func (a *App) handleConfigCmd(sub, args string) {
	switch sub {
	case "show":
		a.Print(theme.Title.Render("Configuration"))
		if a.engine == nil { a.Print(theme.Faint.Render("  No engine connection.")); return }
		keys := []struct{ key, label string }{
			{"defaultProvider", "Default Provider"},
			{"defaultModel", "Default Model"},
			{"workspacePath", "Workspace Path"},
			{"theme", "Theme"},
		}
		for _, k := range keys {
			val := a.engine.Setting(k.key)
			if val == "" { val = theme.Faint.Render("(not set)") }
			a.Print(fmt.Sprintf("  %s %s", lipgloss.NewStyle().Foreground(theme.Muted).Width(20).Render(k.label+":"), val))
		}
	case "set":
		if a.engine == nil { a.Print(noEngineMsg()); return }
		parts := strings.SplitN(args, " ", 2)
		if len(parts) < 2 { a.Print(theme.Faint.Render("Usage: /config set <key> <value>")); return }
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if err := a.engine.DB.SetSetting(key, value); err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		a.Print(theme.SuccessText.Render("Set ") + key + " = " + value)
	default:
		a.Print(usageBlock("/config", []string{"show", "set"}))
	}
}

// --- /settings ---

func (a *App) handleSettings(sub, args string) {
	switch sub {
	case "about":
		a.Print(theme.Title.Render("skrptiq") + " v0.1.0-prototype")
		a.Print(theme.Faint.Render("Interactive terminal for personalised AI agents"))
		if a.engine != nil { a.Print(theme.Faint.Render("Engine: ") + a.engine.DB.Path()) } else { a.Print(theme.Faint.Render("Engine: not connected")) }
		cwd, _ := os.Getwd()
		a.Print(theme.Faint.Render("Working directory: ") + cwd)
	case "providers": a.handleProvidersCmd("list")
	case "connections":
		if a.engine == nil { a.Print(noEngineMsg()); return }
		conns, err := a.engine.Connections()
		if err != nil { a.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(conns) == 0 { a.Print(theme.Faint.Render("No connections configured.")); return }
		a.Print(theme.Title.Render("Connections"))
		typeStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(16)
		for _, c := range conns {
			indicator := theme.ErrorText.Render("●")
			if c.Status == "connected" { indicator = theme.SuccessText.Render("●") }
			line := fmt.Sprintf("  %s %s %s", indicator, typeStyle.Render(c.Type), c.Name)
			if c.Provider != "" { line += theme.Faint.Render(" (" + c.Provider + ")") }
			a.Print(line)
		}
	case "config": a.handleConfigCmd("show", "")
	case "set": a.handleConfigCmd("set", args)
	default:
		a.Print(usageBlock("/settings", []string{"about", "providers", "connections", "config", "set"}))
	}
}

// --- /run ---

func (a *App) handleEnterRun(args string) {
	a.runWorkflow = strings.TrimSpace(args)
	a.setMode(ModeRun)
	if a.runWorkflow == "" {
		a.Print(theme.Title.Render("Run Mode"))
		a.Print(theme.Faint.Render("Type a workflow name (tab to complete), or /exit to return."))
		return
	}
	if a.engine == nil { a.Print(noEngineMsg()); return }
	node, err := a.engine.FindNodeByTitle(a.runWorkflow)
	if err != nil || node == nil || node.Type != "workflow" {
		a.Print(theme.ErrorText.Render("Workflow not found: " + a.runWorkflow))
		return
	}
	a.startExecution(node)
}

// --- /chat ---

func (a *App) handleEnterChat(args string) {
	provider := "not connected"
	if a.engine != nil {
		defaultProvider := a.engine.Setting("defaultProvider")
		if defaultProvider != "" {
			provider = defaultProvider
		} else {
			providers, err := a.engine.Providers()
			if err == nil && len(providers) > 0 { provider = providers[0].Name }
		}
	}
	a.chatProvider = provider
	a.setMode(ModeChat)
	a.Print(theme.Title.Render("Chat Mode"))
	a.Print(theme.Faint.Render("Provider: ") + provider)
	a.Print(theme.Faint.Render("Type naturally. /exit to return to command mode."))
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

func helpText() string {
	return `Available commands:

  Modes
  /chat                  Enter chat mode (talk to your AI team)
  /run <name>            Enter run mode (execute a workflow)
  /command               Return to command mode
  /exit                  Return to command mode

  Browse & search
  /list [type]           List nodes (workflows, skills, prompts...)
  /search <query>        Search nodes by title
  /show <name>           Show node content and metadata

  Execution
  /runs list             List recent executions
  /runs status           Show active executions
  /runs show <id>        Show run details and step outputs

  Profiles
  /profile list          List all profiles
  /profile show          Show active profile details
  /profile use <name>    Switch active profile
  /profile controls      Show quality control settings

  Hub
  /hub list              List imported skrpts
  /hub search <query>    Search community skrpts
  /hub import <slug>     Import a skrpt from Hub
  /hub update            Check for updates

  Infrastructure
  /mcp list              List MCP server connections
  /mcp tools             List available MCP tools
  /workspace show        Show workspace context
  /workspace set <path>  Change workspace directory
  /tags list             List all tags
  /tag <node> <tag>      Apply a tag to a node
  /untag <node> <tag>    Remove a tag from a node

  Settings
  /settings about        Version and system info
  /settings providers    AI provider configuration
  /settings connections  All connections
  /settings config       Show configuration values
  /settings set <k> <v>  Update a configuration value

  Session
  /clear                 Clear screen
  /help                  This message

  Tab to complete commands. Ctrl+C to cancel. Ctrl+D to exit.`
}
