package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"

	eng "github.com/skrptiq/skrptiq-cli/internal/engine"
	"github.com/skrptiq/skrptiq-cli/internal/theme"
	"github.com/skrptiq/skrptiq-cli/internal/version"
)

// commandHelp returns help text for a command. Returns "" if unknown.
func commandHelp(cmd string) string {
	help := map[string]string{
		"help":      "Usage: /help\n\n  Show all available commands.",
		"chat":      "Usage: /chat\n\n  Enter chat mode. Talk to your AI team using natural language.\n  Use /exit to return to command mode.",
		"command":   "Usage: /command\n\n  Return to command mode from chat or run mode.",
		"exit":      "Usage: /exit\n\n  Exit the current mode and return to command mode.",
		"quit":      "Usage: /quit\n\n  Exit skrptiq. Shortcut: ctrl+d ctrl+d.",
		"run":       "Usage: /run [workflow name]\n\n  Enter run mode. If a workflow name is provided, start it immediately.\n  Tab to complete workflow names. Alias: /r",
		"clear":     "Usage: /clear\n\n  Clear the screen.",
		"status":    "Usage: /status\n\n  Show the current execution status with step detail.",
		"list":      "Usage: /list [type] [--tag <name>] [--json]\n\n  List nodes. Types: workflows, skills, prompts, sources,\n  documents, assets, services.\n\n  Flags:\n    --tag <name>   Filter by tag\n    --json         Output as JSON",
		"show":      "Usage: /show [type] <name>\n\n  Show a node's content and metadata.\n  Types: workflow, skill, prompt, source, document, asset, service.\n\n  Examples:\n    /show Blog Post Pipeline\n    /show workflow Blog Post Pipeline",
		"search":    "Usage: /search <query> [--json]\n\n  Search nodes by title. Alias: /s",
		"hub":       "Usage: /hub <subcommand>\n\n  Subcommands:\n    list              List imported skrpts\n    search <query>    Search community skrpts\n    import <slug>     Import a skrpt from Hub\n    update            Check for updates",
		"runs":      "Usage: /runs <subcommand> [--status <s>] [--workflow <name>] [--json]\n\n  Subcommands:\n    list              List recent executions\n    status            Show active executions\n    show <id>         Show run details\n\n  Flags (for list):\n    --status <s>      Filter by status (running, paused, completed, failed)\n    --workflow <name> Filter by workflow name\n    --json            Output as JSON",
		"profile":   "Usage: /profile <subcommand>\n\n  Subcommands:\n    list              List all profiles\n    show              Show active profile details\n    use <name>        Switch active profile\n    controls          Show quality control settings",
		"dials":     "Usage: /dials [subcommand]\n\n  Subcommands:\n    show              Show current persona dial values\n    set <dial> <val>  Set a dial value\n\n  Dials adjust the persona characteristics used during execution.",
		"mcp":       "Usage: /mcp <subcommand>\n\n  Subcommands:\n    list              List MCP server connections\n    tools             List available tools\n    connect <name>    Connect to a server\n    disconnect <name> Disconnect a server",
		"providers":  "Usage: /providers <subcommand>\n\n  Subcommands:\n    list                          List configured providers\n    add <name> <type> <api-key>   Add a new provider\n\n  Types: anthropic, openai, gemini",
		"workspace": "Usage: /workspace <subcommand>\n\n  Subcommands:\n    show              Show current workspace context\n    set <path>        Change workspace directory (persisted)",
		"repos":     "Usage: /repos [list] [--json]\n\n  List workspace dependencies from skrptiq.yaml.\n  Shows clone status and object counts.",
		"tags":      "Usage: /tags list [--json]\n\n  List all tags.",
		"tag":       "Usage: /tag <node title> <tag name>\n\n  Apply a tag to a node.",
		"untag":     "Usage: /untag <node title> <tag name>\n\n  Remove a tag from a node.",
		"config":    "Usage: /config <subcommand>\n\n  Subcommands:\n    show              Show configuration values\n    set <key> <value> Update a configuration value",
		"settings":  "Usage: /settings <subcommand>\n\n  Subcommands:\n    about             Version and system info\n    providers         AI provider configuration\n    connections       All connections\n    config            Show configuration values\n    set <key> <value> Update a configuration value",
	}
	return help[cmd]
}

// handleSlashCommand processes slash commands. Returns true if handled.
func (m *Model) handleSlashCommand(cmd string, args string) bool {
	// Check for --help flag on any command.
	if strings.Contains(args, "--help") || args == "help" {
		if h := commandHelp(cmd); h != "" {
			m.Print(h)
			return true
		}
	}

	sub, subArgs := splitFirst(args)

	switch cmd {
	case "help":
		m.Print(helpText())
	case "chat":
		m.handleEnterChat(args)
	case "command":
		if m.mode != ModeCommand {
			m.Print(theme.Faint.Render("Exited " + m.mode.Label() + " mode."))
			m.setMode(ModeCommand)
		} else {
			m.Print(theme.Faint.Render("Already in command mode."))
		}
	case "exit":
		if m.mode != ModeCommand {
			m.Print(theme.Faint.Render("Exited " + m.mode.Label() + " mode."))
			m.setMode(ModeCommand)
		} else {
			m.Print(theme.Faint.Render("Already in command mode. Use ctrl+d to exit skrptiq."))
		}
	case "quit", "q":
		m.quitRequested = true
	case "run", "r":
		m.handleEnterRun(args)
	case "clear":
		clearToBottom()
	case "list":
		m.handleList(args)
	case "show":
		m.handleShow(args)
	case "search", "s":
		m.handleSearch(args)
	case "status":
		m.handleStatus()
	case "hub":
		m.handleHub(sub, subArgs)
	case "runs":
		m.handleRuns(sub, subArgs)
	case "profile":
		m.handleProfile(sub, subArgs)
	case "dials":
		m.handleDials(sub, subArgs)
	case "mcp":
		m.handleMCPCmd(sub)
	case "providers":
		m.handleProvidersCmd(sub, subArgs)
	case "workspace":
		m.handleWorkspaceCmd(sub, subArgs)
	case "repos":
		m.handleRepos(sub, subArgs)
	case "tags":
		m.handleTagsCmd(sub)
	case "tag":
		m.handleTagNode(args)
	case "untag":
		m.handleUntagNode(args)
	case "config":
		m.handleConfigCmd(sub, subArgs)
	case "settings":
		m.handleSettings(sub, subArgs)
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

func (m *Model) handleList(args string) {
	if m.engine == nil {
		m.Print(noEngineMsg())
		return
	}

	// Parse flags from args.
	nodeType, flags := parseFlags(args)
	tagFilter := flags["tag"]
	jsonOut := flags["json"] != ""

	nodeType = strings.ToLower(nodeType)
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
			m.Print(theme.ErrorText.Render("Unknown type: " + nodeType + ". Try: workflows, skills, prompts, sources, documents, assets, services"))
			return
		}
		filtered, e := m.engine.NodesByType(mapped)
		err = e
		for _, n := range filtered {
			nodes = append(nodes, struct{ Title, Type string }{n.Title, n.Type})
		}
	}
	if err != nil {
		m.Print(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}

	// Apply tag filter if specified.
	if tagFilter != "" {
		var filtered []struct{ Title, Type string }
		allNodes, _ := m.engine.DB.GetAllNodes()
		nodeIDs := make(map[string]bool)
		for _, n := range allNodes {
			tags, _ := m.engine.DB.GetTagsForNode(n.ID)
			for _, t := range tags {
				if strings.EqualFold(t.Name, tagFilter) {
					nodeIDs[n.Title] = true
					break
				}
			}
		}
		for _, n := range nodes {
			if nodeIDs[n.Title] {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
	}

	if len(nodes) == 0 {
		m.Print(theme.Faint.Render("No nodes found."))
		return
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type < nodes[j].Type
		}
		return strings.ToLower(nodes[i].Title) < strings.ToLower(nodes[j].Title)
	})

	if jsonOut {
		type nodeJSON struct {
			Title string `json:"title"`
			Type  string `json:"type"`
		}
		var out []nodeJSON
		for _, n := range nodes { out = append(out, nodeJSON{Title: n.Title, Type: n.Type}) }
		data, _ := json.MarshalIndent(out, "", "  ")
		m.Print(string(data))
		return
	}

	typeFilter2 := ""
	if nodeType != "" {
		typeFilter2 = " (" + nodeType + ")"
	}
	if tagFilter != "" {
		typeFilter2 += " [tag: " + tagFilter + "]"
	}
	m.Print(fmt.Sprintf("%s%s — %d nodes", theme.Title.Render("Nodes"), typeFilter2, len(nodes)))
	typeStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(12)
	for _, n := range nodes {
		m.Print(fmt.Sprintf("  %s %s", typeStyle.Render(n.Type), n.Title))
	}
}

// --- /show ---

func (m *Model) handleShow(args string) {
	if m.engine == nil { m.Print(noEngineMsg()); return }

	// Support both "/show <name>" and "/show <type> <name>".
	typeMap := map[string]bool{
		"workflow": true, "skill": true, "prompt": true,
		"source": true, "document": true, "asset": true, "service": true,
	}
	title := strings.TrimSpace(args)
	if title == "" { m.Print(theme.Faint.Render("Usage: /show <type> <name> or /show <name>")); return }

	parts := strings.SplitN(title, " ", 2)
	if len(parts) == 2 && typeMap[strings.ToLower(parts[0])] {
		title = strings.TrimSpace(parts[1])
	}

	node, err := m.engine.FindNodeByTitle(title)
	if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
	if node == nil { m.Print(theme.Faint.Render("No node found: " + title)); return }
	m.Print(theme.Title.Render(node.Title))
	m.Print(theme.Faint.Render("Type: ") + node.Type)
	if node.Description != nil && *node.Description != "" {
		m.Print(theme.Faint.Render("Description: ") + *node.Description)
	}
	if node.Content != nil && *node.Content != "" {
		m.Print("\n" + *node.Content)
	}
}

// --- /search ---

func (m *Model) handleSearch(args string) {
	if m.engine == nil { m.Print(noEngineMsg()); return }
	query, flags := parseFlags(args)
	jsonOut := flags["json"] != ""
	if query == "" { m.Print(theme.Faint.Render("Usage: /search <query> [--json]")); return }
	nodes, err := m.engine.SearchNodes(query)
	if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
	if len(nodes) == 0 { m.Print(theme.Faint.Render("No results for: " + query)); return }

	if jsonOut {
		type nodeJSON struct {
			Title string `json:"title"`
			Type  string `json:"type"`
		}
		var out []nodeJSON
		for _, n := range nodes { out = append(out, nodeJSON{Title: n.Title, Type: n.Type}) }
		data, _ := json.MarshalIndent(out, "", "  ")
		m.Print(string(data))
		return
	}

	m.Print(fmt.Sprintf("%d results for %q:", len(nodes), query))
	typeStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(12)
	for _, n := range nodes {
		m.Print(fmt.Sprintf("  %s %s", typeStyle.Render(n.Type), n.Title))
	}
}

// --- /hub ---

func (m *Model) handleHub(sub, args string) {
	if m.engine == nil { m.Print(noEngineMsg()); return }
	switch sub {
	case "list":
		imports, err := m.engine.HubImports()
		if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		m.Print(theme.Title.Render("Hub — Imported Skrpts"))
		if len(imports) == 0 {
			m.Print(theme.Faint.Render("  No skrpts imported from Hub."))
		} else {
			for _, imp := range imports {
				ver := ""
				if imp.Version != nil { ver = theme.Faint.Render(" v" + *imp.Version) }
				m.Print(fmt.Sprintf("  %s%s", imp.Name, ver))
			}
		}
	case "search":
		query := strings.TrimSpace(args)
		if query == "" { m.Print(theme.Faint.Render("Usage: /hub search <query>")); return }
		results, err := m.engine.Hub.Search(query)
		if err != nil { m.Print(theme.ErrorText.Render("Hub search error: " + err.Error())); return }
		if len(results) == 0 { m.Print(theme.Faint.Render("No results for: " + query)); return }
		m.Print(fmt.Sprintf("%d results for %q:", len(results), query))
		for _, r := range results {
			line := fmt.Sprintf("  %s", theme.Bold.Render(r.Name))
			if r.Category != "" { line += theme.Faint.Render(" [" + r.Category + "]") }
			m.Print(line)
			if r.Description != "" { m.Print("    " + theme.Faint.Render(r.Description)) }
		}
	case "import":
		slug := strings.TrimSpace(args)
		if slug == "" { m.Print(theme.Faint.Render("Usage: /hub import <slug>")); return }
		skrpt, err := m.engine.Hub.GetSkrpt(slug)
		if err != nil { m.Print(theme.ErrorText.Render("Hub error: " + err.Error())); return }
		if skrpt == nil { m.Print(theme.ErrorText.Render("Skrpt not found: " + slug)); return }
		m.Print(fmt.Sprintf("Found: %s v%s (%d nodes)", theme.Bold.Render(skrpt.Name), skrpt.Version, skrpt.NodeCount))
		// GH#842 — a deprecated skrpt is still importable (K-033); warn and point to the successor, don't block.
		if note, ok := deprecationNotice(skrpt.Deprecated, skrpt.SupersededBy); ok { m.Print(theme.WarningText.Render(note)) }
		m.Print("Import download not yet wired.")
	case "update":
		imports, err := m.engine.HubImports()
		if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(imports) == 0 { m.Print(theme.Faint.Render("No imported skrpts to update.")); return }
		m.Print(theme.Title.Render("Hub — Update Check"))
		for _, imp := range imports {
			versions, err := m.engine.Hub.GetVersions(imp.Slug)
			if err != nil { m.Print(fmt.Sprintf("  %s — %s", imp.Name, theme.ErrorText.Render("check failed"))); continue }
			currentVer := "(unknown)"
			if imp.Version != nil { currentVer = *imp.Version }
			if len(versions) > 0 && versions[0].Version != currentVer {
				m.Print(fmt.Sprintf("  %s %s → %s", theme.WarningText.Render("⬆"), imp.Name, theme.Bold.Render(versions[0].Version)))
			} else {
				m.Print(fmt.Sprintf("  %s %s %s", theme.SuccessText.Render("✓"), imp.Name, theme.Faint.Render("up to date")))
			}
		}
	default:
		m.Print(usageBlock("/hub", []string{"list", "search", "import", "update"}))
	}
}

// deprecationNotice builds the "deprecated — superseded by X" note for a
// skrpt (GH#842). Returns ("", false) when not deprecated so the caller
// prints nothing. Pure + free of the Hub client so it's unit-testable
// without a network round-trip.
func deprecationNotice(deprecated bool, supersededBy *string) (string, bool) {
	if !deprecated {
		return "", false
	}
	note := "⚠ deprecated"
	if supersededBy != nil && *supersededBy != "" {
		note += " — superseded by " + *supersededBy
	}
	return note, true
}

// --- /runs ---

func (m *Model) handleRuns(sub, args string) {
	if m.engine == nil { m.Print(noEngineMsg()); return }
	switch sub {
	case "list", "":
		_, flags := parseFlags(args)
		statusFilter := flags["status"]
		workflowFilter := flags["workflow"]
		jsonOut := flags["json"] != ""

		runs, err := m.engine.ListExecutions(50)
		if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }

		// Apply filters.
		if statusFilter != "" || workflowFilter != "" {
			var filtered []eng.ExecutionSummary
			for _, r := range runs {
				if statusFilter != "" && !strings.EqualFold(r.Status, statusFilter) { continue }
				if workflowFilter != "" && !strings.Contains(strings.ToLower(r.WorkflowTitle), strings.ToLower(workflowFilter)) { continue }
				filtered = append(filtered, r)
			}
			runs = filtered
		}

		if len(runs) == 0 { m.Print(theme.Faint.Render("No executions found.")); return }

		if jsonOut {
			data, _ := json.MarshalIndent(runs, "", "  ")
			m.Print(string(data))
			return
		}

		title := "Recent Runs"
		if statusFilter != "" { title += " [" + statusFilter + "]" }
		if workflowFilter != "" { title += " [" + workflowFilter + "]" }
		m.Print(theme.Title.Render(title))
		for _, r := range runs {
			shortID := r.ID
			if len(shortID) > 8 { shortID = shortID[:8] }
			line := fmt.Sprintf("  %s  %s  %s  %s", statusIcon(r.Status), theme.Faint.Render(shortID), r.WorkflowTitle, theme.Faint.Render(r.StartedAt))
			if r.TotalTokens > 0 { line += theme.Faint.Render(fmt.Sprintf("  %d tokens", r.TotalTokens)) }
			m.Print(line)
			if r.Error != nil && *r.Error != "" { m.Print("         " + theme.ErrorText.Render(*r.Error)) }
		}
	case "status":
		runs, err := m.engine.ListExecutions(10)
		if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		var active []string
		for _, r := range runs {
			if r.Status == "running" || r.Status == "paused" {
				active = append(active, fmt.Sprintf("  %s  %s  %s", statusIcon(r.Status), r.WorkflowTitle, theme.Faint.Render(r.StartedAt)))
			}
		}
		if len(active) == 0 { m.Print(theme.Faint.Render("No active executions.")) } else {
			m.Print(theme.Title.Render("Active Runs"))
			for _, line := range active { m.Print(line) }
		}
	case "show":
		m.handleRunShow(args)
	default:
		m.Print(usageBlock("/runs", []string{"list [--status <s>] [--workflow <name>] [--json]", "status", "show <id>"}))
	}
}

func (m *Model) handleRunShow(args string) {
	idArg, stepArg := splitFirst(args)
	if idArg == "" { m.Print(theme.Faint.Render("Usage: /runs show <id> [step <n>]")); return }
	fullID, err := m.engine.FindRunByPrefix(idArg)
	if err != nil || fullID == nil { m.Print(theme.ErrorText.Render("Run not found: " + idArg)); return }
	run, err := m.engine.GetRunDetail(*fullID)
	if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }

	if stepArg != "" {
		stepKey, stepNum := splitFirst(stepArg)
		if stepKey == "step" && stepNum != "" {
			n := 0
			fmt.Sscanf(stepNum, "%d", &n)
			for _, s := range run.Steps {
				if s.Position == n {
					m.Print(theme.Title.Render(s.NodeTitle) + " — step " + fmt.Sprintf("%d", s.Position))
					m.Print(theme.Faint.Render("Status: ") + statusIcon(s.Status) + " " + s.Status)
					if s.Provider != "" {
						p := s.Provider
						if s.Model != "" { p += " / " + s.Model }
						m.Print(theme.Faint.Render("Provider: ") + p)
					}
					if s.Duration != "" { m.Print(theme.Faint.Render("Duration: ") + s.Duration) }
					if s.Error != "" { m.Print(theme.ErrorText.Render("Error: ") + s.Error) }
					if s.Output != "" { m.Print("\n" + s.Output) } else { m.Print(theme.Faint.Render("\n(no output)")) }
					return
				}
			}
			m.Print(theme.ErrorText.Render(fmt.Sprintf("Step %d not found.", n)))
			return
		}
	}

	m.Print(theme.Title.Render(run.WorkflowTitle))
	m.Print(theme.Faint.Render("ID: ") + run.ID)
	m.Print(theme.Faint.Render("Status: ") + statusIcon(run.Status) + " " + run.Status)
	m.Print(theme.Faint.Render("Started: ") + run.StartedAt)
	if run.CompletedAt != nil { m.Print(theme.Faint.Render("Completed: ") + *run.CompletedAt) }
	if run.TotalTokens > 0 { m.Print(theme.Faint.Render("Tokens: ") + fmt.Sprintf("%d", run.TotalTokens)) }
	if run.Error != nil && *run.Error != "" { m.Print(theme.ErrorText.Render("Error: ") + *run.Error) }
	if len(run.Steps) > 0 {
		m.Print("\n" + theme.Bold.Render("Steps"))
		for _, s := range run.Steps {
			line := fmt.Sprintf("  %s %d. %s", statusIcon(s.Status), s.Position, s.NodeTitle)
			if s.Provider != "" { line += theme.Faint.Render(" (" + s.Provider + ")") }
			if s.Duration != "" { line += theme.Faint.Render(" " + s.Duration) }
			if len(s.Output) > 0 { line += theme.Faint.Render(fmt.Sprintf(" %d chars", len(s.Output))) }
			m.Print(line)
		}
		m.Print(theme.Faint.Render("\nUse /runs show " + run.ID[:8] + " step <n> to view step output."))
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

func (m *Model) handleProfile(sub, args string) {
	if m.engine == nil { m.Print(noEngineMsg()); return }
	switch sub {
	case "list":
		_, flags := parseFlags(args)
		jsonOut := flags["json"] != ""
		profiles, err := m.engine.Profiles()
		if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(profiles) == 0 { m.Print(theme.Faint.Render("No profiles configured.")); return }
		if jsonOut {
			data, _ := json.MarshalIndent(profiles, "", "  ")
			m.Print(string(data))
			return
		}
		m.Print(theme.Title.Render("Profiles"))
		typeStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(12)
		for _, p := range profiles {
			active := "  "
			if p.IsActive == 1 { active = theme.SuccessText.Render("● ") }
			m.Print(fmt.Sprintf("  %s%s %s", active, typeStyle.Render(p.Type), p.Name))
		}
	case "show":
		voice, _ := m.engine.ActiveProfile("voice")
		if voice == nil { m.Print(theme.Faint.Render("No active voice profile.")); return }
		m.Print(theme.Title.Render(voice.Name))
		m.Print(theme.Faint.Render("Type: ") + voice.Type)
		if voice.Content != "" { m.Print("\n" + voice.Content) }
	case "use":
		name := strings.TrimSpace(args)
		if name == "" { m.Print(theme.Faint.Render("Usage: /profile use <name>")); return }
		profile, err := m.engine.FindProfileByName(name)
		if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if profile == nil { m.Print(theme.ErrorText.Render("Profile not found: " + name)); return }
		if err := m.engine.SetActiveProfile(profile.ID, profile.Type); err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		m.Print(theme.SuccessText.Render("Switched to profile: ") + profile.Name)
		m.setMode(m.mode) // refresh prompt
	case "controls":
		voice, _ := m.engine.ActiveProfile("voice")
		if voice == nil { m.Print(theme.Faint.Render("No active voice profile.")); return }
		m.Print(theme.Title.Render("Quality Controls") + " — " + voice.Name)
		if voice.Metadata != nil && *voice.Metadata != "" {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(*voice.Metadata), &parsed); err == nil {
				for k, v := range parsed {
					m.Print(fmt.Sprintf("  %s %v", lipgloss.NewStyle().Foreground(theme.Muted).Width(20).Render(k+":"), v))
				}
			} else {
				m.Print(theme.Faint.Render(*voice.Metadata))
			}
		} else {
			m.Print(theme.Faint.Render("No control settings in profile metadata."))
		}
	default:
		m.Print(usageBlock("/profile", []string{"list", "show", "use", "controls"}))
	}
}

// --- /mcp ---

func (m *Model) handleMCPCmd(sub string) {
	if m.engine == nil { m.Print(noEngineMsg()); return }
	switch sub {
	case "list":
		servers, err := m.engine.MCPServers()
		if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(servers) == 0 { m.Print(theme.Faint.Render("No MCP servers configured.")); return }
		m.Print(theme.Title.Render("MCP Servers"))
		for _, s := range servers {
			indicator := theme.ErrorText.Render("●")
			if s.Status == "connected" { indicator = theme.SuccessText.Render("●") }
			line := fmt.Sprintf("  %s %s", indicator, s.Name)
			if s.Provider != "" { line += theme.Faint.Render(" (" + s.Provider + ")") }
			m.Print(line)
		}
	case "tools":
		servers, err := m.engine.MCPServers()
		if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(servers) == 0 { m.Print(theme.Faint.Render("No MCP servers configured.")); return }
		m.Print(theme.Title.Render("MCP Tools"))
		for _, s := range servers {
			m.Print("\n  " + theme.Bold.Render(s.Name))
			if s.Capabilities != nil && *s.Capabilities != "" {
				var caps []string
				if err := json.Unmarshal([]byte(*s.Capabilities), &caps); err == nil {
					for _, cap := range caps { m.Print("    " + cap) }
				} else { m.Print("    " + theme.Faint.Render(*s.Capabilities)) }
			} else { m.Print("    " + theme.Faint.Render("No tools listed.")) }
		}
	case "connect": m.Print(theme.Faint.Render("/mcp connect — requires MCP client runtime."))
	case "disconnect": m.Print(theme.Faint.Render("/mcp disconnect — requires MCP client runtime."))
	default:
		m.Print(usageBlock("/mcp", []string{"list", "tools", "connect", "disconnect"}))
	}
}

// --- /providers ---

func (m *Model) handleProvidersCmd(sub, args string) {
	if m.engine == nil { m.Print(noEngineMsg()); return }
	switch sub {
	case "list":
		providers, err := m.engine.Providers()
		if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(providers) == 0 { m.Print(theme.Faint.Render("No AI providers configured.")); return }
		m.Print(theme.Title.Render("AI Providers"))
		for _, p := range providers {
			indicator := theme.ErrorText.Render("●")
			if p.Status == "connected" { indicator = theme.SuccessText.Render("●") }
			line := fmt.Sprintf("  %s %s", indicator, p.Name)
			if p.Provider != "" { line += theme.Faint.Render(" (" + p.Provider + ")") }
			m.Print(line)
		}
	case "add":
		m.handleProviderAdd(args)
	default:
		m.Print(usageBlock("/providers", []string{"list", "add"}))
	}
}

// --- /workspace ---

func (m *Model) handleWorkspaceCmd(sub, args string) {
	switch sub {
	case "show":
		m.Print(theme.Title.Render("Workspace"))
		cwd, _ := os.Getwd()
		m.Print("  " + theme.Faint.Render("Path: ") + cwd)
		if m.engine != nil { m.Print("  " + theme.Faint.Render("Database: ") + m.engine.DB.Path()) }
	case "set":
		path := strings.TrimSpace(args)
		if path == "" { m.Print(theme.Faint.Render("Usage: /workspace set <path>")); return }
		if strings.HasPrefix(path, "~") {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, path[1:])
		}
		absPath, err := filepath.Abs(path)
		if err != nil { m.Print(theme.ErrorText.Render("Invalid path: " + err.Error())); return }
		info, err := os.Stat(absPath)
		if err != nil || !info.IsDir() { m.Print(theme.ErrorText.Render("Not a directory: " + absPath)); return }
		if err := os.Chdir(absPath); err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if m.engine != nil {
			m.engine.DB.SetSetting("workspacePath", absPath)
		}
		m.Print(theme.SuccessText.Render("Workspace: ") + absPath)
	default:
		m.Print(usageBlock("/workspace", []string{"show", "set"}))
	}
}

// --- /repos ---

func (m *Model) handleRepos(sub, args string) {
	if m.engine == nil { m.Print(noEngineMsg()); return }
	_, flags := parseFlags(args)
	jsonOut := flags["json"] != ""

	switch sub {
	case "list", "":
		cwd, err := os.Getwd()
		if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		deps, err := m.engine.ListDeps(cwd)
		if err != nil {
			if os.IsNotExist(err) {
				m.Print(theme.Faint.Render("No skrptiq.yaml found in current workspace."))
				return
			}
			m.Print(theme.ErrorText.Render("Error: " + err.Error()))
			return
		}
		if len(deps) == 0 {
			m.Print(theme.Faint.Render("No dependencies defined in skrptiq.yaml."))
			return
		}
		if jsonOut {
			data, _ := json.MarshalIndent(deps, "", "  ")
			m.Print(string(data))
			return
		}
		m.Print(theme.Title.Render("Workspace Dependencies"))
		for _, d := range deps {
			indicator := theme.ErrorText.Render("○")
			status := "not cloned"
			if d.Cloned {
				indicator = theme.SuccessText.Render("●")
				status = "cloned"
			}
			line := fmt.Sprintf("  %s %s", indicator, theme.Bold.Render(d.Name))
			line += theme.Faint.Render(" — " + status)
			if d.ObjectCount > 0 { line += theme.Faint.Render(fmt.Sprintf(" (%d objects)", d.ObjectCount)) }
			m.Print(line)
			m.Print("    " + theme.Faint.Render(d.Source))
		}
	default:
		m.Print(usageBlock("/repos", []string{"list [--json]"}))
	}
}

// --- /tags ---

func (m *Model) handleTagsCmd(sub string) {
	if m.engine == nil { m.Print(noEngineMsg()); return }
	// sub may contain --json flag.
	_, flags := parseFlags(sub)
	jsonOut := flags["json"] != ""
	cleanSub := strings.TrimSpace(strings.Replace(sub, "--json", "", 1))
	switch cleanSub {
	case "list", "":
		tags, err := m.engine.Tags()
		if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(tags) == 0 { m.Print(theme.Faint.Render("No tags defined.")); return }
		if jsonOut {
			data, _ := json.MarshalIndent(tags, "", "  ")
			m.Print(string(data))
			return
		}
		m.Print(theme.Title.Render("Tags"))
		for _, t := range tags {
			colour := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Colour))
			m.Print(fmt.Sprintf("  %s %s", colour.Render("●"), t.Name))
		}
	default:
		m.Print(usageBlock("/tags", []string{"list [--json]"}))
	}
}

// --- /tag and /untag ---

func (m *Model) handleTagNode(args string) {
	if m.engine == nil { m.Print(noEngineMsg()); return }
	args = strings.TrimSpace(args)
	if args == "" { m.Print(theme.Faint.Render("Usage: /tag <node title> <tag name>")); return }
	tags, err := m.engine.Tags()
	if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
	var matchedTag, matchedTagID, nodeTitle string
	for _, t := range tags {
		if strings.HasSuffix(strings.ToLower(args), strings.ToLower(t.Name)) {
			matchedTag = t.Name
			matchedTagID = t.ID
			nodeTitle = strings.TrimSpace(args[:len(args)-len(t.Name)])
			break
		}
	}
	if matchedTag == "" { m.Print(theme.ErrorText.Render("Could not identify tag. Use /tags list.")); return }
	node, err := m.engine.FindNodeByTitle(nodeTitle)
	if err != nil || node == nil { m.Print(theme.ErrorText.Render("Node not found: " + nodeTitle)); return }
	if err := m.engine.DB.AssignTag(node.ID, matchedTagID); err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
	m.Print(theme.SuccessText.Render("Tagged ") + node.Title + " with " + matchedTag)
}

func (m *Model) handleUntagNode(args string) {
	if m.engine == nil { m.Print(noEngineMsg()); return }
	args = strings.TrimSpace(args)
	if args == "" { m.Print(theme.Faint.Render("Usage: /untag <node title> <tag name>")); return }
	tags, err := m.engine.Tags()
	if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
	var matchedTag, matchedTagID, nodeTitle string
	for _, t := range tags {
		if strings.HasSuffix(strings.ToLower(args), strings.ToLower(t.Name)) {
			matchedTag = t.Name
			matchedTagID = t.ID
			nodeTitle = strings.TrimSpace(args[:len(args)-len(t.Name)])
			break
		}
	}
	if matchedTag == "" { m.Print(theme.ErrorText.Render("Could not identify tag. Use /tags list.")); return }
	node, err := m.engine.FindNodeByTitle(nodeTitle)
	if err != nil || node == nil { m.Print(theme.ErrorText.Render("Node not found: " + nodeTitle)); return }
	if err := m.engine.DB.UnassignTag(node.ID, matchedTagID); err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
	m.Print(theme.SuccessText.Render("Removed ") + matchedTag + " from " + node.Title)
}

// --- /config ---

func (m *Model) handleConfigCmd(sub, args string) {
	switch sub {
	case "show":
		m.Print(theme.Title.Render("Configuration"))
		if m.engine == nil { m.Print(theme.Faint.Render("  No engine connection.")); return }
		keys := []struct{ key, label string }{
			{"defaultProvider", "Default Provider"},
			{"defaultModel", "Default Model"},
			{"workspacePath", "Workspace Path"},
			{"theme", "Theme"},
		}
		for _, k := range keys {
			val := m.engine.Setting(k.key)
			if val == "" { val = theme.Faint.Render("(not set)") }
			m.Print(fmt.Sprintf("  %s %s", lipgloss.NewStyle().Foreground(theme.Muted).Width(20).Render(k.label+":"), val))
		}
	case "set":
		if m.engine == nil { m.Print(noEngineMsg()); return }
		parts := strings.SplitN(args, " ", 2)
		if len(parts) < 2 { m.Print(theme.Faint.Render("Usage: /config set <key> <value>")); return }
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if err := m.engine.DB.SetSetting(key, value); err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		m.Print(theme.SuccessText.Render("Set ") + key + " = " + value)
	default:
		m.Print(usageBlock("/config", []string{"show", "set"}))
	}
}

// --- /settings ---

func (m *Model) handleSettings(sub, args string) {
	switch sub {
	case "about":
		m.Print(theme.Title.Render("skrptiq") + " " + version.Full())
		m.Print(theme.Faint.Render("Interactive terminal for personalised AI agents"))
		if m.engine != nil { m.Print(theme.Faint.Render("Engine: ") + m.engine.DB.Path()) } else { m.Print(theme.Faint.Render("Engine: not connected")) }
		cwd, _ := os.Getwd()
		m.Print(theme.Faint.Render("Working directory: ") + cwd)
	case "providers": m.handleProvidersCmd("list", "")
	case "connections":
		if m.engine == nil { m.Print(noEngineMsg()); return }
		conns, err := m.engine.Connections()
		if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
		if len(conns) == 0 { m.Print(theme.Faint.Render("No connections configured.")); return }
		m.Print(theme.Title.Render("Connections"))
		typeStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(16)
		for _, c := range conns {
			indicator := theme.ErrorText.Render("●")
			if c.Status == "connected" { indicator = theme.SuccessText.Render("●") }
			line := fmt.Sprintf("  %s %s %s", indicator, typeStyle.Render(c.Type), c.Name)
			if c.Provider != "" { line += theme.Faint.Render(" (" + c.Provider + ")") }
			m.Print(line)
		}
	case "config": m.handleConfigCmd("show", "")
	case "set": m.handleConfigCmd("set", args)
	default:
		m.Print(usageBlock("/settings", []string{"about", "providers", "connections", "config", "set"}))
	}
}

// --- /run ---

func (m *Model) handleEnterRun(args string) {
	m.runWorkflow = strings.TrimSpace(args)
	m.setMode(ModeRun)
	if m.runWorkflow == "" {
		m.Print(theme.Title.Render("Run Mode"))
		m.Print(theme.Faint.Render("Type a workflow name (tab to complete), or /exit to return."))
		return
	}
	if m.engine == nil { m.Print(noEngineMsg()); return }
	node, err := m.engine.FindNodeByTitle(m.runWorkflow)
	if err != nil || node == nil || node.Type != "workflow" {
		m.Print(theme.ErrorText.Render("Workflow not found: " + m.runWorkflow))
		return
	}
	m.startExecution(node)
}

// --- /chat ---

func (m *Model) handleEnterChat(args string) {
	provider := "not connected"
	if m.engine != nil {
		defaultProvider := m.engine.Setting("defaultProvider")
		if defaultProvider != "" {
			provider = defaultProvider
		} else {
			providers, err := m.engine.Providers()
			if err == nil && len(providers) > 0 { provider = providers[0].Name }
		}
	}
	m.chatProvider = provider
	m.setMode(ModeChat)
	m.Print(theme.Title.Render("Chat Mode"))
	m.Print(theme.Faint.Render("Provider: ") + provider)
	m.Print(theme.Faint.Render("Type naturally. /exit to return to command mode."))
}

// --- /status ---

func (m *Model) handleStatus() {
	if m.engine == nil { m.Print(noEngineMsg()); return }
	if m.executionID == "" {
		m.Print(theme.Faint.Render("No active execution."))
		return
	}
	run, err := m.engine.GetRunDetail(m.executionID)
	if err != nil { m.Print(theme.ErrorText.Render("Error: " + err.Error())); return }
	m.Print(theme.Title.Render(run.WorkflowTitle))
	m.Print(theme.Faint.Render("ID: ") + run.ID[:8])
	m.Print(theme.Faint.Render("Status: ") + statusIcon(run.Status) + " " + run.Status)
	m.Print(theme.Faint.Render("Started: ") + run.StartedAt)
	if run.TotalTokens > 0 { m.Print(theme.Faint.Render("Tokens: ") + fmt.Sprintf("%d", run.TotalTokens)) }
	if run.Error != nil && *run.Error != "" { m.Print(theme.ErrorText.Render("Error: ") + *run.Error) }
	if len(run.Steps) > 0 {
		m.Print("")
		for _, s := range run.Steps {
			line := fmt.Sprintf("  %s %d. %s", statusIcon(s.Status), s.Position, s.NodeTitle)
			if s.Provider != "" { line += theme.Faint.Render(" (" + s.Provider + ")") }
			if s.Duration != "" { line += theme.Faint.Render(" " + s.Duration) }
			m.Print(line)
		}
	}
}

// --- /dials ---

func (m *Model) handleDials(sub, args string) {
	if m.engine == nil { m.Print(noEngineMsg()); return }
	switch sub {
	case "show", "":
		raw := m.engine.Setting("PERSONA_DIALS")
		if raw == "" {
			m.Print(theme.Faint.Render("No persona dials configured."))
			return
		}
		var dials map[string]any
		if err := json.Unmarshal([]byte(raw), &dials); err != nil {
			m.Print(theme.ErrorText.Render("Error parsing dials: " + err.Error()))
			return
		}
		m.Print(theme.Title.Render("Persona Dials"))
		labelStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(20)
		// Sort keys for consistent display.
		keys := make([]string, 0, len(dials))
		for k := range dials { keys = append(keys, k) }
		sort.Strings(keys)
		for _, k := range keys {
			m.Print(fmt.Sprintf("  %s %v", labelStyle.Render(k+":"), dials[k]))
		}
	case "set":
		parts := strings.SplitN(args, " ", 2)
		if len(parts) < 2 { m.Print(theme.Faint.Render("Usage: /dials set <dial> <value>")); return }
		dialName, dialValue := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		raw := m.engine.Setting("PERSONA_DIALS")
		var dials map[string]any
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &dials); err != nil {
				dials = make(map[string]any)
			}
		} else {
			dials = make(map[string]any)
		}
		// Try to parse as number.
		var val any = dialValue
		var numVal float64
		if _, err := fmt.Sscanf(dialValue, "%f", &numVal); err == nil {
			val = numVal
		}
		dials[dialName] = val
		out, _ := json.Marshal(dials)
		if err := m.engine.DB.SetSetting("PERSONA_DIALS", string(out)); err != nil {
			m.Print(theme.ErrorText.Render("Error: " + err.Error()))
			return
		}
		m.Print(theme.SuccessText.Render("Set ") + dialName + " = " + dialValue)
	default:
		m.Print(usageBlock("/dials", []string{"show", "set <dial> <value>"}))
	}
}

// --- /providers add ---

func (m *Model) handleProviderAdd(args string) {
	// Expect: /providers add <name> <provider-type> <api-key>
	parts := strings.SplitN(strings.TrimSpace(args), " ", 3)
	if len(parts) < 3 {
		m.Print(theme.Faint.Render("Usage: /providers add <name> <type> <api-key>"))
		m.Print(theme.Faint.Render("  Types: anthropic, openai, gemini"))
		m.Print(theme.Faint.Render("  Example: /providers add claude anthropic sk-ant-..."))
		return
	}
	name, provType, apiKey := parts[0], parts[1], parts[2]
	id := fmt.Sprintf("conn_%s_%d", provType, time.Now().UnixMilli())
	authData := &apiKey
	if err := m.engine.DB.CreateConnection(id, name, "llm-provider", provType, "connected", authData, nil, nil); err != nil {
		m.Print(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}
	m.Print(theme.SuccessText.Render("Added provider: ") + name + theme.Faint.Render(" ("+provType+")"))
}

// --- helpers ---

// parseFlags splits args into a positional part and --flag values.
// e.g. "workflows --tag content --json" → ("workflows", {"tag":"content","json":"true"})
func parseFlags(args string) (string, map[string]string) {
	flags := make(map[string]string)
	var positional []string
	parts := strings.Fields(args)
	for i := 0; i < len(parts); i++ {
		if strings.HasPrefix(parts[i], "--") {
			key := strings.TrimPrefix(parts[i], "--")
			if key == "json" || key == "help" {
				flags[key] = "true"
			} else if i+1 < len(parts) && !strings.HasPrefix(parts[i+1], "--") {
				flags[key] = parts[i+1]
				i++
			} else {
				flags[key] = "true"
			}
		} else {
			positional = append(positional, parts[i])
		}
	}
	return strings.Join(positional, " "), flags
}

func usageBlock(cmd string, subcommands []string) string {
	var b strings.Builder
	b.WriteString(theme.Title.Render(cmd) + "\n")
	for _, s := range subcommands {
		b.WriteString("  " + cmd + " " + s + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func noEngineMsg() string {
	return theme.ErrorText.Render("No database connection.") +
		"\n  Try restarting skrptiq, or run with --db-path to specify the database location."
}

func helpText() string {
	return `Available commands:

  Modes
  /chat                  Enter chat mode (talk to your AI team)
  /run <name>            Run a workflow (/r is a shortcut)
  /command               Return to command mode
  /exit                  Return to command mode
  /quit                  Exit skrptiq (/q is a shortcut)

  Browse & search
  /list [type]           List nodes (--tag <name>, --json)
  /search <query>        Search nodes by title (/s is a shortcut, --json)
  /show [type] <name>    Show node content and metadata
  /status                Show current execution status

  Execution
  /runs list             List executions (--status, --workflow, --json)
  /runs status           Show active executions
  /runs show <id>        Show run details and step outputs

  Profiles & Persona
  /profile list          List all profiles (--json)
  /profile show          Show active profile details
  /profile use <name>    Switch active profile
  /profile controls      Show quality control settings
  /dials                 Show persona dial settings
  /dials set <d> <v>     Set a persona dial value

  Hub
  /hub list              List imported skrpts
  /hub search <query>    Search community skrpts
  /hub import <slug>     Import a skrpt from Hub
  /hub update            Check for updates

  Infrastructure
  /mcp list              List MCP server connections
  /mcp tools             List available MCP tools
  /providers list        List AI providers
  /providers add         Add a new provider
  /workspace show        Show workspace context
  /workspace set <path>  Change workspace directory
  /repos list            List workspace dependencies (--json)
  /tags list             List all tags (--json)
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

  All commands support --help. Tab to complete commands.
  Ctrl+C to cancel. Ctrl+D to exit.`
}

// clearToBottom clears the screen and pushes the cursor to the bottom
// so the prompt sits at the bottom of the terminal window.
func clearToBottom() {
	_, rows, _ := term.GetSize(os.Stdout.Fd())
	if rows < 10 {
		rows = 24
	}
	fmt.Print("\033[2J\033[H")
	// Leave room for the prompt area (separator + input + separator + status).
	for i := 0; i < rows-4; i++ {
		fmt.Println()
	}
}
