package app

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/skrptiq/skrptiq-cli/internal/components"
	"github.com/skrptiq/skrptiq-cli/internal/theme"
	"github.com/skrptiq/skrptiq-cli/internal/views/diff"
	"github.com/skrptiq/skrptiq-cli/internal/views/gate"
	"github.com/skrptiq/skrptiq-cli/internal/views/progress"
	"github.com/skrptiq/skrptiq-cli/internal/views/repl"
	"github.com/skrptiq/skrptiq-cli/internal/views/tree"
)

// exitGracePeriod is the maximum time between two Ctrl+D presses to exit.
const exitGracePeriod = 500 * time.Millisecond

// clearExitHintMsg clears the exit hint after the grace period expires.
type clearExitHintMsg struct{}

// View identifiers.
type viewID int

const (
	viewREPL viewID = iota
	viewProgress
	viewTree
	viewGate
	viewDiff
)

// Model is the root bubbletea model.
type Model struct {
	keys       KeyMap
	header     components.Header
	statusBar  components.StatusBar
	width      int
	height     int
	activeView viewID
	ready      bool

	// Double Ctrl+D exit state.
	lastExitPress time.Time
	exitHint      bool

	repl     repl.Model
	progress progress.Model
	tree     tree.Model
	gate     gate.Model
	diff     diff.Model
}

// New creates a new root app model.
func New() Model {
	prompt := repl.PromptConfig{
		Symbol:       "❯ ",
		Style:        theme.Prompt,
		ContextLeft:  "default",
		ContextRight: "ctrl+d ctrl+d to exit",
	}

	return Model{
		keys:       DefaultKeyMap(),
		header:     components.NewHeader("skrptiq", "v0.1.0-prototype"),
		statusBar:  components.NewStatusBar(),
		activeView: viewREPL,
		repl:       repl.NewWithPrompt(prompt),
	}
}

func (m Model) Init() tea.Cmd {
	return m.repl.Init()
}

// contentHeight returns the available height for the active view.
func (m Model) contentHeight() int {
	h := m.height - 2 // header + status bar
	if h < 1 {
		return 1
	}
	return h
}

// resizeView sets the size on the currently active view.
// Must be called via pointer (&m) to propagate changes.
func resizeView(m *Model) {
	h := m.contentHeight()
	switch m.activeView {
	case viewREPL:
		m.repl.SetSize(m.width, h)
	case viewProgress:
		m.progress.SetSize(m.width, h)
	case viewTree:
		m.tree.SetSize(m.width, h)
	case viewGate:
		m.gate.SetSize(m.width, h)
	case viewDiff:
		m.diff.SetSize(m.width, h)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.header.Width = msg.Width
		m.statusBar.Width = msg.Width
		m.ready = true
		resizeView(&m)
		return m, nil

	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Exit) {
			now := time.Now()
			if m.exitHint && now.Sub(m.lastExitPress) < exitGracePeriod {
				return m, tea.Quit
			}
			m.lastExitPress = now
			m.exitHint = true
			m.repl.AddOutput(theme.Faint.Render("Press Ctrl+D again to exit."))
			return m, tea.Tick(exitGracePeriod, func(_ time.Time) tea.Msg {
				return clearExitHintMsg{}
			})
		}

	case clearExitHintMsg:
		m.exitHint = false
		return m, nil

	// REPL submitted a command.
	case repl.SubmitMsg:
		return handleCommand(m, msg.Input)

	// Progress completed.
	case progress.DoneMsg:
		m.repl.AddOutput(msg.Summary)
		m.activeView = viewREPL
		resizeView(&m)
		return m, nil

	// Tree dismissed.
	case tree.DismissMsg:
		m.activeView = viewREPL
		resizeView(&m)
		return m, nil

	// Gate result.
	case gate.ResultMsg:
		var action string
		switch msg.Action {
		case gate.ActionApprove:
			action = "approved"
		case gate.ActionEdit:
			action = "edited and approved"
		case gate.ActionReject:
			action = "rejected"
		}
		m.repl.AddOutput("Gate: " + action)
		m.activeView = viewREPL
		resizeView(&m)
		return m, nil

	case gate.CancelMsg:
		m.repl.AddOutput("Gate: cancelled")
		m.activeView = viewREPL
		resizeView(&m)
		return m, nil

	// Diff result.
	case diff.ResultMsg:
		var action string
		switch msg.Action {
		case diff.ActionAccept:
			action = "accepted"
		case diff.ActionReject:
			action = "rejected"
		}
		m.repl.AddOutput("Diff for " + msg.File + ": " + action)
		m.activeView = viewREPL
		resizeView(&m)
		return m, nil

	case diff.DismissMsg:
		m.activeView = viewREPL
		resizeView(&m)
		return m, nil
	}

	// Route update to active view.
	var cmd tea.Cmd
	switch m.activeView {
	case viewREPL:
		m.repl, cmd = m.repl.Update(msg)
		cmds = append(cmds, cmd)
	case viewProgress:
		m.progress, cmd = m.progress.Update(msg)
		cmds = append(cmds, cmd)
	case viewTree:
		m.tree, cmd = m.tree.Update(msg)
		cmds = append(cmds, cmd)
	case viewGate:
		m.gate, cmd = m.gate.Update(msg)
		cmds = append(cmds, cmd)
	case viewDiff:
		m.diff, cmd = m.diff.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	header := m.header.View()
	status := m.statusBar.View()

	var content string
	switch m.activeView {
	case viewREPL:
		content = m.repl.View()
	case viewProgress:
		content = m.progress.View()
	case viewTree:
		content = m.tree.View()
	case viewGate:
		content = m.gate.View()
	case viewDiff:
		content = m.diff.View()
	}

	return header + "\n" + content + "\n" + status
}

func handleCommand(m Model, input string) (Model, tea.Cmd) {
	cmd := strings.TrimSpace(strings.ToLower(input))

	switch {
	case cmd == "help":
		m.repl.AddOutput(helpText())
		return m, nil

	case cmd == "run demo" || cmd == "demo":
		m.progress = progress.New([]string{
			"Drafting Agent",
			"Review Agent (GPT-4)",
			"Revision Agent",
			"Voice Agent",
			"Polish Agent",
		})
		m.activeView = viewProgress
		resizeView(&m)
		return m, m.progress.Init()

	case cmd == "tree":
		m.tree = tree.New("Blog Post Pipeline", mockTree())
		m.activeView = viewTree
		resizeView(&m)
		return m, m.tree.Init()

	case cmd == "gate":
		m.gate = gate.New("Review draft before continuing", mockGateContent())
		m.activeView = viewGate
		resizeView(&m)
		return m, m.gate.Init()

	case cmd == "diff":
		m.diff = diff.New("README.md", mockDiffHunks())
		m.activeView = viewDiff
		resizeView(&m)
		return m, m.diff.Init()

	default:
		m.repl.AddOutput("Unknown command: " + input + ". Type help for available commands.")
		return m, nil
	}
}

func helpText() string {
	return `Available commands:
  run demo  — streaming step progress display
  tree      — expandable execution tree
  gate      — gate approval flow
  diff      — diff review with accept/reject
  help      — this message`
}

func mockTree() *tree.Node {
	return &tree.Node{
		Name:     "Blog Post Pipeline",
		Status:   tree.NodeRunning,
		Expanded: true,
		Children: []*tree.Node{
			{Name: "Drafting Agent", Status: tree.NodeDone, Detail: "847 words"},
			{
				Name:     "Review Agent (GPT-4)",
				Status:   tree.NodeWarning,
				Detail:   "2 findings",
				Expanded: true,
				Children: []*tree.Node{
					{Name: "Finding 1: unused import", Status: tree.NodeWarning},
					{Name: "Finding 2: missing error check", Status: tree.NodeWarning},
				},
			},
			{Name: "Revision Agent", Status: tree.NodeDone, Detail: "addressed all 2"},
			{Name: "Voice Agent", Status: tree.NodeDone, Detail: "92% match"},
			{Name: "Polish Agent", Status: tree.NodeRunning, Detail: "grammar: Professional"},
		},
	}
}

func mockGateContent() string {
	return `# Blog Post Draft: Getting Started with MCP

Model Context Protocol (MCP) is an open standard that enables AI assistants
to connect with external data sources and tools. In this post, we'll walk
through setting up your first MCP server and connecting it to your workflow.

## What is MCP?

MCP provides a standardised way for AI applications to:
- Connect to external data sources
- Use tools provided by servers
- Maintain context across interactions

## Setting Up

First, install the MCP SDK:

    npm install @modelcontextprotocol/sdk

Then create a basic server:

    import { Server } from '@modelcontextprotocol/sdk';
    const server = new Server({ name: 'my-server' });

## Next Steps

- Configure authentication
- Add custom tools
- Connect to your AI assistant

This draft is ready for review.`
}

func mockDiffHunks() []diff.Hunk {
	return []diff.Hunk{
		{
			Header: "@@ -1,8 +1,12 @@",
			Lines: []diff.DiffLine{
				{Type: diff.LineContext, Content: "# My Project"},
				{Type: diff.LineContext, Content: ""},
				{Type: diff.LineRemove, Content: "A simple project."},
				{Type: diff.LineAdd, Content: "A powerful toolkit for automating content workflows"},
				{Type: diff.LineAdd, Content: "with personalised AI agents."},
				{Type: diff.LineContext, Content: ""},
				{Type: diff.LineContext, Content: "## Installation"},
				{Type: diff.LineContext, Content: ""},
				{Type: diff.LineRemove, Content: "Run `npm install`."},
				{Type: diff.LineAdd, Content: "```bash"},
				{Type: diff.LineAdd, Content: "brew install skrptiq"},
				{Type: diff.LineAdd, Content: "```"},
				{Type: diff.LineAdd, Content: ""},
				{Type: diff.LineAdd, Content: "Or install from source:"},
				{Type: diff.LineAdd, Content: ""},
				{Type: diff.LineAdd, Content: "```bash"},
				{Type: diff.LineAdd, Content: "go install github.com/skrptiq/skrptiq-cli@latest"},
				{Type: diff.LineAdd, Content: "```"},
			},
		},
	}
}
