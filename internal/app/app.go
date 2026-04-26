package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/llm"
	"github.com/skrptiq/engine/storage"

	"github.com/skrptiq/skrptiq-cli/internal/components"
	eng "github.com/skrptiq/skrptiq-cli/internal/engine"
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

// AppMode represents the current interaction mode.
type AppMode int

const (
	ModeCommand AppMode = iota // Default — slash commands, bare text shows chat hint
	ModeChat                   // Chat — all input goes to LLM
	ModeRun                    // Run — executing a workflow, input is for gates/approvals
)

// ModeLabel returns the display label for the mode.
func (m AppMode) ModeLabel() string {
	switch m {
	case ModeChat:
		return "chat"
	case ModeRun:
		return "run"
	default:
		return "command"
	}
}

// inputMetaInfo holds display metadata for a workflow input variable.
type inputMetaInfo struct {
	Label       string
	Description string
	Example     string
}

// Model is the root bubbletea model.
type Model struct {
	keys       KeyMap
	header     components.Header
	statusBar  components.StatusBar
	engine     *eng.App
	commands   []components.Command
	mode       AppMode
	width      int
	height     int
	activeView viewID
	ready      bool

	// Mode state.
	chatProvider string // active LLM provider name in chat mode
	runWorkflow  string // active workflow name in run mode

	// Input collection state (for workflow variables before execution).
	pendingInputs []string            // remaining input variable names to collect
	collectedInputs map[string]string // collected so far
	inputMeta     map[string]inputMetaInfo // label/description/example per variable
	pendingNode   *storage.Node       // workflow node waiting for inputs

	// Streaming state.
	streamCh     streamChannel // active stream channel (chat or execution)
	streamBuf    string        // accumulated streaming output for current response
	executionID  string        // active execution ID (for gate resume)
	cancelStream context.CancelFunc // cancels the active stream/execution

	// Printer for terminal scrollback output.
	printer    *Printer
	startupMsg string // printed once when program starts

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
	// Open the engine (shared DB).
	engine, engineErr := eng.Open("")

	commands := BuildCommands(engine)

	// Build status bar from real data.
	statusBar := buildStatusBar(engine)

	m := Model{
		keys:       DefaultKeyMap(),
		header:     components.NewHeader("skrptiq", "v0.1.0-prototype"),
		statusBar:  statusBar,
		engine:     engine,
		commands:   commands,
		mode:       ModeCommand,
		activeView: viewREPL,
		repl:       repl.NewWithPrompt(repl.DefaultPromptConfig(), commands),
	}

	// Set the prompt via enterMode so it's always consistent.
	enterMode(&m, ModeCommand)

	// Store startup messages to print once program is running.
	if engineErr != nil {
		m.startupMsg = theme.ErrorText.Render("Engine: " + engineErr.Error())
	} else if engine != nil {
		m.startupMsg = theme.Title.Render("skrptiq") + " " + theme.Faint.Render("v0.1.0-prototype") + "\n" +
			theme.Faint.Render("  Profile: ") + statusBar.Profile + "\n" +
			theme.Faint.Render("  Workspace: ") + statusBar.Workspace + "\n" +
			theme.Faint.Render("  Database: ") + engine.DB.Path()
	}

	return m
}

func buildStatusBar(engine *eng.App) components.StatusBar {
	sb := components.NewStatusBar()

	if engine == nil {
		return sb
	}

	// Set profile from DB.
	if p, _ := engine.ActiveProfile("voice"); p != nil {
		sb.Profile = p.Name
	}

	// Detect workspace.
	if cwd, err := os.Getwd(); err == nil {
		home, _ := os.UserHomeDir()
		if home != "" && strings.HasPrefix(cwd, home) {
			sb.Workspace = "~" + cwd[len(home):]
		} else {
			sb.Workspace = filepath.Base(cwd)
		}
	}

	// Set MCP servers from DB.
	if servers, err := engine.MCPServers(); err == nil {
		sb.MCP = nil
		for _, s := range servers {
			sb.MCP = append(sb.MCP, components.MCPStatus{
				Name:      s.Name,
				Connected: s.Status == "connected",
			})
		}
	}

	return sb
}

// Printer is a shared reference to the tea.Program for scrollback output.
// It's a pointer so it can be wired after program creation but before Run.
type Printer struct {
	Program *tea.Program
}

// Println prints to terminal scrollback.
func (p *Printer) Println(text string) {
	if p != nil && p.Program != nil {
		p.Program.Println(text)
	}
}

// NewWithPrinter creates a new app model with a shared printer.
func NewWithPrinter(printer *Printer) Model {
	m := New()
	m.printer = printer
	m.repl.SetPrinter(func(text string) {
		printer.Println(text)
	})
	return m
}

// PrintOutput prints a line to the terminal scrollback.
func (m *Model) PrintOutput(text string) {
	if m.printer != nil {
		m.printer.Println(text)
	}
}

func (m Model) Init() tea.Cmd {
	return m.repl.Init()
}

// enterMode switches the app to a new mode, updating the prompt visuals.
func enterMode(m *Model, mode AppMode) {
	m.mode = mode

	// Clear bare completer when changing mode.
	m.repl.SetBareCompleter(nil)

	cfg := m.repl.Prompt()
	// All modes use the same clean style — the emoji is the mode signal.
	cfg.Style = lipgloss.NewStyle().Bold(true)

	switch mode {
	case ModeCommand:
		cfg.Symbol = "⚡ "
		profileName := "default"
		if m.engine != nil {
			if p, _ := m.engine.ActiveProfile("voice"); p != nil {
				profileName = p.Name
			}
		}
		cfg.ContextLeft = "command · " + profileName
		cfg.ContextRight = "Profile: " + m.statusBar.Profile +
			"  Workspace: " + m.statusBar.Workspace

	case ModeChat:
		cfg.Symbol = "💬 "
		provider := m.chatProvider
		if provider == "" {
			provider = "not connected"
		}
		cfg.ContextLeft = "chat · " + provider
		cfg.ContextRight = "/exit to return"

	case ModeRun:
		cfg.Symbol = "▶ "
		workflow := m.runWorkflow
		if workflow == "" {
			workflow = "select a workflow"
		}
		cfg.ContextLeft = "run · " + workflow
		cfg.ContextRight = "/exit or esc to cancel"
	}

	m.repl.SetPrompt(cfg)
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
		// Print startup message on first render.
		if m.startupMsg != "" {
			m.PrintOutput(m.startupMsg)
			m.startupMsg = ""
		}
		return m, nil

	case tea.KeyMsg:
		// Ctrl+C always quits — safety net.
		if key.Matches(msg, m.keys.ForceQuit) {
			return m, tea.Quit
		}
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
		// Escape cancels active streams, overlay views, or exits mode.
		if key.Matches(msg, m.keys.Back) {
			// Cancel any active stream first.
			if m.cancelStream != nil {
				m.cancelStream()
				m.cancelStream = nil
				m.streamCh = nil
				m.repl.SetActivity("")
				m.repl.AddOutput(theme.Faint.Render("Cancelled."))
				if m.executionID != "" {
					m.engine.StopExecution(m.executionID)
					m.executionID = ""
				}
				enterMode(&m, ModeCommand)
				return m, nil
			}
			if m.activeView != viewREPL {
				m.repl.AddOutput(theme.Faint.Render("Cancelled."))
				m.repl.SetActivity("")
				m.activeView = viewREPL
				resizeView(&m)
				return m, nil
			}
			if m.mode != ModeCommand {
				m.repl.AddOutput(theme.Faint.Render("Exited " + m.mode.ModeLabel() + " mode."))
				enterMode(&m, ModeCommand)
				return m, nil
			}
		}

	case clearExitHintMsg:
		m.exitHint = false
		return m, nil

	// Streaming LLM output.
	case StreamChunkMsg:
		m.streamBuf += msg.Text
		// Update the last history entry with accumulated output.
		if len(m.repl.History()) > 0 {
			m.repl.UpdateLastOutput(m.streamBuf)
		}
		// Read next chunk.
		return m, readStream(m.streamCh)

	case StreamDoneMsg:
		m.repl.SetActivity("")
		m.streamCh = nil
		m.cancelStream = nil
		if msg.InputTokens > 0 || msg.OutputTokens > 0 {
			m.repl.AddOutput(theme.Faint.Render(
				fmt.Sprintf("  %s · %d in / %d out tokens",
					msg.Provider, msg.InputTokens, msg.OutputTokens)))
		}
		return m, nil

	case StreamErrorMsg:
		m.repl.SetActivity("")
		m.streamCh = nil
		m.cancelStream = nil
		m.repl.AddOutput(theme.ErrorText.Render("Error: " + msg.Err.Error()))
		return m, nil

	// Workflow execution progress.
	case ProgressEventMsg:
		return handleProgressEvent(m, msg)

	case ExecutionDoneMsg:
		m.repl.SetActivity("")
		m.streamCh = nil
		m.cancelStream = nil
		if msg.Status == "completed" {
			m.repl.AddOutput(theme.SuccessText.Render("Workflow completed."))
		} else {
			m.repl.AddOutput(theme.ErrorText.Render("Workflow failed: " + msg.Error))
		}
		enterMode(&m, ModeCommand)
		return m, nil

	// REPL submitted a command.
	case repl.SubmitMsg:
		return handleCommand(m, msg.Input)

	// Progress completed.
	case progress.DoneMsg:
		m.repl.AddOutput(msg.Summary)
		m.repl.SetActivity("")
		m.activeView = viewREPL
		resizeView(&m)
		return m, nil

	// Tree dismissed.
	case tree.DismissMsg:
		m.repl.SetActivity("")
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
	// Without alt screen, only render the input area.
	// Output goes to terminal scrollback via Println.
	return m.repl.View()
}

func handleCommand(m Model, input string) (Model, tea.Cmd) {
	raw := strings.TrimSpace(input)

	// Slash commands — starts with "/".
	if strings.HasPrefix(raw, "/") {
		stripped := raw[1:]
		cmd := strings.ToLower(stripped)
		args := ""
		if idx := strings.Index(stripped, " "); idx > 0 {
			cmd = strings.ToLower(stripped[:idx])
			args = strings.TrimSpace(stripped[idx+1:])
		}

		// Try slash command handlers.
		if result, teaCmd, handled := handleSlashCommand(&m, cmd, args); handled {
			return result, teaCmd
		}

		// Prototype demo views.
		switch cmd {
		case "demo":
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

		case "tree":
			m.tree = tree.New("Blog Post Pipeline", mockTree())
			m.activeView = viewTree
			resizeView(&m)
			return m, m.tree.Init()

		case "gate":
			m.gate = gate.New("Review draft before continuing", mockGateContent())
			m.activeView = viewGate
			resizeView(&m)
			return m, m.gate.Init()

		case "diff":
			m.diff = diff.New("README.md", mockDiffHunks())
			m.activeView = viewDiff
			resizeView(&m)
			return m, m.diff.Init()
		}

		// Deferred execution commands.
		switch cmd {
		case "resume", "stop":
			m.repl.AddOutput(theme.Faint.Render("/" + cmd + " — requires engine execution wiring."))
			return m, nil
		}

		m.repl.AddOutput(theme.ErrorText.Render("Unknown command: /" + cmd) + " — type /help for available commands.")
		return m, nil
	}

	// Bare text — behaviour depends on current mode.
	switch m.mode {
	case ModeChat:
		return handleChatInput(m, raw)
	case ModeRun:
		if m.runWorkflow == "" {
			// No workflow selected — treat input as workflow name.
			m.runWorkflow = raw
			enterMode(&m, ModeRun)
			if m.engine != nil {
				node, err := m.engine.FindNodeByTitle(raw)
				if err != nil || node == nil || node.Type != "workflow" {
					m.repl.AddOutput(theme.ErrorText.Render("Workflow not found: " + raw))
					m.runWorkflow = ""
					enterMode(&m, ModeRun)
					return m, nil
				}
				cmd := handleEnterRunExec(m.engine, &m, node)
				return m, cmd
			}
			return m, nil
		}
		if len(m.pendingInputs) > 0 {
			// Collecting workflow inputs — store value and prompt for next.
			currentVar := m.pendingInputs[0]
			m.collectedInputs[currentVar] = raw
			m.pendingInputs = m.pendingInputs[1:]

			if len(m.pendingInputs) > 0 {
				promptForInput(&m)
				return m, nil
			}
			// All inputs collected — start execution.
			m.repl.AddOutput(theme.Faint.Render("All inputs collected. Starting execution..."))
			cmd := startExecution(m.engine, &m, m.pendingNode, m.collectedInputs)
			m.pendingNode = nil
			return m, cmd
		}
		return handleRunInput(m, raw)
	default:
		// Command mode — bare text prompts the user to enter chat mode.
		m.repl.AddOutput(theme.Faint.Render("Type /chat to enter chat mode, or use / commands."))
		return m, nil
	}
}

// handleChatInput processes natural language input in chat mode.
func handleChatInput(m Model, input string) (Model, tea.Cmd) {
	if m.engine == nil {
		m.repl.AddOutput(theme.ErrorText.Render("No engine connection."))
		return m, nil
	}

	m.repl.SetActivity("Thinking...")
	m.repl.AddOutput("") // Empty entry for streaming output
	m.streamBuf = ""

	// Create cancellable context.
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelStream = cancel

	// Create channel for streaming messages.
	ch := make(chan tea.Msg, 64)
	m.streamCh = ch

	messages := []llm.Message{{Role: "user", Content: input}}

	// Start LLM call in goroutine.
	go func() {
		defer close(ch)

		resp, err := m.engine.Chat(ctx, messages, llm.Options{}, func(chunk string) {
			ch <- StreamChunkMsg{Text: chunk}
		})
		if err != nil {
			ch <- StreamErrorMsg{Err: err}
			return
		}

		provider := resp.Provider
		model := resp.Model
		inputTokens := 0
		outputTokens := 0
		if resp.Usage != nil {
			inputTokens = resp.Usage.InputTokens
			outputTokens = resp.Usage.OutputTokens
		}
		ch <- StreamDoneMsg{
			FullOutput:   resp.Content,
			Provider:     provider + "/" + model,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		}
	}()

	// Start reading from the channel.
	return m, readStream(ch)
}

// promptForInput shows the prompt for the next required input variable.
func promptForInput(m *Model) {
	if len(m.pendingInputs) == 0 {
		return
	}
	varName := m.pendingInputs[0]
	label := varName
	desc := ""
	example := ""
	if meta, ok := m.inputMeta[varName]; ok {
		if meta.Label != "" {
			label = meta.Label
		}
		desc = meta.Description
		example = meta.Example
	}

	prompt := theme.Bold.Render(label) + ":"
	if desc != "" {
		prompt += " " + theme.Faint.Render(desc)
	}
	if example != "" {
		prompt += "\n  " + theme.Faint.Render("Example: "+example)
	}
	m.repl.AddOutput(prompt)
}

// startExecution kicks off workflow execution with collected inputs.
func startExecution(engine *eng.App, m *Model, node *storage.Node, inputs map[string]string) tea.Cmd {
	m.repl.SetActivity("Starting workflow...")
	m.streamBuf = ""

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelStream = cancel

	ch := make(chan tea.Msg, 64)
	m.streamCh = ch

	workflowID := node.ID

	go func() {
		defer close(ch)
		onProgress := func(evt exec.ProgressEvent) {
			ch <- ProgressEventMsg{evt}
		}
		_, err := engine.RunWorkflow(ctx, workflowID, inputs, onProgress)
		if err != nil {
			ch <- ExecutionDoneMsg{Status: "failed", Error: err.Error()}
			return
		}
		ch <- ExecutionDoneMsg{Status: "completed"}
	}()

	return readStream(ch)
}

// handleRunInput processes input during an active workflow run (gate responses).
func handleRunInput(m Model, input string) (Model, tea.Cmd) {
	if m.executionID == "" || m.engine == nil {
		m.repl.AddOutput(theme.Faint.Render("No active execution to respond to."))
		return m, nil
	}

	m.repl.SetActivity("Resuming...")
	m.streamBuf = ""

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelStream = cancel

	ch := make(chan tea.Msg, 64)
	m.streamCh = ch

	execID := m.executionID
	engine := m.engine

	go func() {
		defer close(ch)

		onProgress := func(evt exec.ProgressEvent) {
			ch <- ProgressEventMsg{evt}
		}

		_, err := engine.ResumeExecution(ctx, execID, input, onProgress)
		if err != nil {
			ch <- ExecutionDoneMsg{ExecutionID: execID, Status: "failed", Error: err.Error()}
			return
		}
		ch <- ExecutionDoneMsg{ExecutionID: execID, Status: "completed"}
	}()

	return m, readStream(ch)
}

// handleProgressEvent processes a workflow execution progress event.
func handleProgressEvent(m Model, msg ProgressEventMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case "execution-started":
		m.executionID = msg.ExecutionID
		m.repl.SetActivity("Running workflow...")

	case "step-started":
		m.repl.SetActivity("Step " + fmt.Sprintf("%d", msg.Position) + ": " + msg.NodeTitle)
		m.repl.AddOutput(theme.Faint.Render("  ◌ ") + msg.NodeTitle)

	case "step-chunk":
		m.streamBuf += msg.Chunk

	case "step-completed":
		status := theme.SuccessText.Render("  ✓ ") + msg.NodeTitle
		if msg.Provider != "" {
			status += theme.Faint.Render(" (" + msg.Provider + ")")
		}
		if msg.TokenUsage != nil {
			status += theme.Faint.Render(fmt.Sprintf(" %d tokens", msg.TokenUsage.Total))
		}
		m.repl.AddOutput(status)
		m.streamBuf = ""

	case "step-failed":
		m.repl.AddOutput(theme.ErrorText.Render("  ✗ " + msg.NodeTitle + ": " + msg.Error))

	case "step-awaiting-input":
		m.repl.SetActivity("")
		m.repl.AddOutput(theme.WarningText.Render("⏸ Gate: ") + msg.NodeTitle)
		if msg.GateInstructions != "" {
			m.repl.AddOutput(msg.GateInstructions)
		}
		m.repl.AddOutput(theme.Faint.Render("Type your response and press enter to continue."))
		// Don't read next from channel — wait for user input via handleRunInput.
		return m, nil

	case "execution-paused":
		// Already handled by step-awaiting-input.
		return m, nil

	case "execution-completed":
		return m, nil // Handled by ExecutionDoneMsg.

	case "execution-failed":
		return m, nil // Handled by ExecutionDoneMsg.
	}

	// Continue reading from the stream.
	if m.streamCh != nil {
		return m, readStream(m.streamCh)
	}
	return m, nil
}

func helpText() string {
	return `Available commands:

  Modes
  /chat                  Enter chat mode (talk to your AI team)
  /run <name>            Enter run mode (execute a workflow)
  /exit                  Return to command mode
  esc                    Cancel current mode or overlay

  Browse & search
  /list [type]           List nodes (workflows, skills, prompts...)
  /search <query>        Search nodes by title
  /show <name>           Show node content and metadata

  Execution
  /run <name>            Execute a workflow
  /runs list             List recent executions
  /runs status           Show active executions
  /resume                Resume a paused execution
  /stop                  Cancel the running workflow

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
  /clear                 Clear session history
  /help                  This message

  Type / to see all commands with autocomplete.`
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
