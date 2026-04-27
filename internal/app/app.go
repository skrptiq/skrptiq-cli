package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/llm"
	"github.com/skrptiq/engine/storage"

	eng "github.com/skrptiq/skrptiq-cli/internal/engine"
	"github.com/skrptiq/skrptiq-cli/internal/prompt"
	"github.com/skrptiq/skrptiq-cli/internal/theme"
)

// AppMode represents the current interaction mode.
type AppMode int

const (
	ModeCommand AppMode = iota
	ModeChat
	ModeRun
)

func (m AppMode) Label() string {
	switch m {
	case ModeChat:
		return "chat"
	case ModeRun:
		return "run"
	default:
		return "command"
	}
}

func (m AppMode) Symbol() string {
	switch m {
	case ModeChat:
		return "💬"
	case ModeRun:
		return "▶"
	default:
		return "⚡"
	}
}

type inputMetaInfo struct {
	Label       string
	Description string
	Example     string
}

// Model is the top-level bubbletea model.
type Model struct {
	prompt   prompt.Model
	program  *tea.Program
	engine   *eng.App
	commands []Command
	mode     AppMode

	chatProvider string
	runWorkflow  string

	executionID  string
	cancelStream context.CancelFunc

	pendingInputs   []string
	collectedInputs map[string]string
	inputMeta       map[string]inputMetaInfo
	pendingNode     *storage.Node

	pendingOutput []string
	lastEOF       time.Time
	ready         bool
}

// New creates a new Model.
func New() Model {
	engine, engineErr := eng.Open("")

	m := Model{
		engine: engine,
		mode:   ModeCommand,
	}
	m.commands = BuildCommands(engine)
	m.prompt = prompt.New(m.mode.Symbol(), m.statusText())

	// Print banner before bubbletea starts.
	printBanner(engine, engineErr)

	return m
}

// SetProgram stores the program reference for Println.
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// Print queues text for printing to scrollback above the managed region.
// The actual printing happens via a tea.Cmd after Update returns.
func (m *Model) Print(text string) {
	m.pendingOutput = append(m.pendingOutput, text)
}

// flushOutput returns a tea.Cmd that prints all queued output via Program.Println.
// This runs in a goroutine outside the renderer lock, avoiding deadlock.
func (m *Model) flushOutput() tea.Cmd {
	if len(m.pendingOutput) == 0 {
		return nil
	}
	lines := make([]string, len(m.pendingOutput))
	copy(lines, m.pendingOutput)
	m.pendingOutput = nil
	p := m.program

	return func() tea.Msg {
		for _, line := range lines {
			p.Println(line)
		}
		return flushDoneMsg{}
	}
}

type flushDoneMsg struct{}

func (m Model) Init() tea.Cmd {
	return m.prompt.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ready = true

	case prompt.SubmitMsg:
		m.handleInput(msg.Text)
		return m, m.flushOutput()

	case prompt.CtrlCMsg:
		if m.cancelStream != nil {
			m.cancelStream()
			m.cancelStream = nil
			m.Print(theme.Faint.Render("Cancelled."))
			if m.executionID != "" && m.engine != nil {
				m.engine.StopExecution(m.executionID)
				m.executionID = ""
			}
			m.setMode(ModeCommand)
		}
		return m, m.flushOutput()

	case prompt.CtrlDMsg:
		now := time.Now()
		if now.Sub(m.lastEOF) < 500*time.Millisecond {
			m.Print(theme.Faint.Render("Goodbye."))
			return m, tea.Sequence(m.flushOutput(), tea.Quit)
		}
		m.lastEOF = now
		m.Print(theme.Faint.Render("Press Ctrl+D again to exit."))
		return m, m.flushOutput()

	case prompt.EscMsg:
		if m.cancelStream != nil {
			m.cancelStream()
			m.cancelStream = nil
			m.Print(theme.Faint.Render("Cancelled."))
		}
		if m.mode != ModeCommand {
			m.Print(theme.Faint.Render("Exited " + m.mode.Label() + " mode."))
			m.setMode(ModeCommand)
		}
		return m, m.flushOutput()
	}

	// Pass to prompt model.
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	return m.prompt.View()
}

func (m *Model) setMode(mode AppMode) {
	m.mode = mode
	m.prompt.SetSymbol(m.mode.Symbol())
	m.prompt.SetStatus(m.statusText())
}

func (m Model) statusText() string {
	var parts []string
	parts = append(parts, m.mode.Label())
	if m.engine != nil {
		if p, _ := m.engine.ActiveProfile("voice"); p != nil {
			parts = append(parts, p.Name)
		}
	}
	switch m.mode {
	case ModeChat:
		if m.chatProvider != "" {
			parts = append(parts, m.chatProvider)
		}
	case ModeRun:
		if m.runWorkflow != "" {
			parts = append(parts, m.runWorkflow)
		}
	}
	return strings.Join(parts, " · ")
}

func (m *Model) handleInput(input string) {
	if input == "/" {
		m.listCommands()
		return
	}

	if strings.HasPrefix(input, "/") {
		stripped := input[1:]
		cmd := strings.ToLower(stripped)
		args := ""
		if idx := strings.Index(stripped, " "); idx > 0 {
			cmd = strings.ToLower(stripped[:idx])
			args = strings.TrimSpace(stripped[idx+1:])
		}

		if m.handleSlashCommand(cmd, args) {
			return
		}

		switch cmd {
		case "demo", "tree", "gate", "diff":
			m.Print(theme.Faint.Render("/" + cmd + " — prototype TUI views not available."))
			return
		case "resume", "stop":
			m.Print(theme.Faint.Render("/" + cmd + " — requires engine execution wiring."))
			return
		}

		m.Print(theme.ErrorText.Render("Unknown command: /"+cmd) + " — type /help for available commands.")
		return
	}

	switch m.mode {
	case ModeChat:
		m.handleChatInput(input)
	case ModeRun:
		if m.runWorkflow == "" {
			m.handleRunWorkflowSelect(input)
		} else if len(m.pendingInputs) > 0 {
			m.handleInputCollection(input)
		} else {
			m.handleRunInput(input)
		}
	default:
		m.Print(theme.Faint.Render("Use /chat for chat mode or / for commands."))
	}
}

func (m *Model) listCommands() {
	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB")).
		Background(lipgloss.Color("#374151")).
		Padding(0, 1)
	descStyle := lipgloss.NewStyle().Foreground(theme.Muted)

	for _, cmd := range m.commands {
		desc := cmd.Description
		if cmd.HasSubcommands() {
			var subs []string
			for _, sub := range cmd.Subcommands {
				subs = append(subs, sub.Name)
			}
			desc += " (" + strings.Join(subs, ", ") + ")"
		}
		m.Print(nameStyle.Render(cmd.Name) + " " + descStyle.Render(desc))
	}
}

func (m *Model) handleChatInput(input string) {
	if m.engine == nil {
		m.Print(theme.ErrorText.Render("No engine connection."))
		return
	}
	m.Print(theme.Faint.Render("Thinking..."))
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelStream = cancel
	defer func() { m.cancelStream = nil }()

	messages := []llm.Message{{Role: "user", Content: input}}
	resp, err := m.engine.Chat(ctx, messages, llm.Options{}, func(chunk string) {
		fmt.Print(chunk) // stream directly
	})
	fmt.Println()
	if err != nil {
		m.Print(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}
	if resp.Usage != nil {
		m.Print(theme.Faint.Render(fmt.Sprintf("  %s/%s · %d in / %d out tokens",
			resp.Provider, resp.Model, resp.Usage.InputTokens, resp.Usage.OutputTokens)))
	}
}

func (m *Model) handleRunWorkflowSelect(input string) {
	if m.engine == nil { m.Print(theme.ErrorText.Render("No engine connection.")); return }
	node, err := m.engine.FindNodeByTitle(input)
	if err != nil || node == nil || node.Type != "workflow" {
		m.Print(theme.ErrorText.Render("Workflow not found: " + input)); return
	}
	m.runWorkflow = node.Title
	m.setMode(ModeRun)
	m.startExecution(node)
}

func (m *Model) handleInputCollection(input string) {
	currentVar := m.pendingInputs[0]
	m.collectedInputs[currentVar] = input
	m.pendingInputs = m.pendingInputs[1:]
	if len(m.pendingInputs) > 0 { m.promptForInput(); return }
	m.Print(theme.Faint.Render("All inputs collected. Starting execution..."))
	m.startExecutionWithInputs(m.pendingNode, m.collectedInputs)
	m.pendingNode = nil
}

func (m *Model) promptForInput() {
	if len(m.pendingInputs) == 0 { return }
	varName := m.pendingInputs[0]
	label, desc, example := varName, "", ""
	if meta, ok := m.inputMeta[varName]; ok {
		if meta.Label != "" { label = meta.Label }
		desc = meta.Description
		example = meta.Example
	}
	p := theme.Bold.Render(label) + ":"
	if desc != "" { p += " " + theme.Faint.Render(desc) }
	if example != "" { p += "\n  " + theme.Faint.Render("Example: "+example) }
	m.Print(p)
}

func (m *Model) handleRunInput(input string) {
	if m.executionID == "" || m.engine == nil {
		m.Print(theme.Faint.Render("No active execution.")); return
	}
	m.Print(theme.Faint.Render("Resuming..."))
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelStream = cancel
	defer func() { m.cancelStream = nil }()
	_, err := m.engine.ResumeExecution(ctx, m.executionID, input, func(evt exec.ProgressEvent) {
		m.handleProgressEvent(evt)
	})
	if err != nil {
		m.Print(theme.ErrorText.Render("Execution failed: " + err.Error()))
	} else {
		m.Print(theme.SuccessText.Render("Workflow completed."))
	}
	m.setMode(ModeCommand)
}

func (m *Model) startExecution(node *storage.Node) {
	plan, err := m.engine.BuildPlan(node.ID)
	if err != nil { m.Print(theme.ErrorText.Render("Plan error: " + err.Error())); return }
	m.Print(theme.Title.Render("Run Mode") + " — " + plan.WorkflowTitle)
	var b strings.Builder
	for _, pg := range plan.PositionGroups {
		for _, step := range pg.Steps {
			gate := ""
			if step.IsGate { gate = theme.WarningText.Render(" (gate)") }
			b.WriteString(fmt.Sprintf("  %d. %s%s\n", step.Position, step.NodeTitle, gate))
		}
	}
	m.Print(b.String())
	if len(plan.InputVariables) > 0 {
		m.pendingInputs = plan.InputVariables
		m.collectedInputs = make(map[string]string)
		m.pendingNode = node
		m.inputMeta = make(map[string]inputMetaInfo)
		for varName, meta := range plan.InputMeta {
			m.inputMeta[varName] = inputMetaInfo{Label: meta.Label, Description: meta.Description, Example: meta.Example}
		}
		m.Print(theme.Faint.Render(fmt.Sprintf("%d input(s) required.", len(plan.InputVariables))))
		m.promptForInput()
		return
	}
	m.startExecutionWithInputs(node, map[string]string{})
}

func (m *Model) startExecutionWithInputs(node *storage.Node, inputs map[string]string) {
	m.Print(theme.Faint.Render("Starting execution..."))
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelStream = cancel
	defer func() { m.cancelStream = nil }()
	_, err := m.engine.RunWorkflow(ctx, node.ID, inputs, func(evt exec.ProgressEvent) {
		m.handleProgressEvent(evt)
	})
	if err != nil {
		m.Print(theme.ErrorText.Render("Execution failed: " + err.Error()))
		m.setMode(ModeCommand)
		return
	}
	if m.executionID != "" {
		m.Print(theme.Faint.Render("Workflow paused at gate. Type your response."))
	} else {
		m.Print(theme.SuccessText.Render("Workflow completed."))
		m.setMode(ModeCommand)
	}
}

func (m *Model) handleProgressEvent(evt exec.ProgressEvent) {
	switch evt.Type {
	case "execution-started":
		m.executionID = evt.ExecutionID
	case "step-started":
		m.Print(theme.Faint.Render("  ◌ ") + evt.NodeTitle)
	case "step-chunk":
		fmt.Print(evt.Chunk)
	case "step-completed":
		s := theme.SuccessText.Render("  ✓ ") + evt.NodeTitle
		if evt.Provider != "" { s += theme.Faint.Render(" (" + evt.Provider + ")") }
		if evt.TokenUsage != nil { s += theme.Faint.Render(fmt.Sprintf(" %d tokens", evt.TokenUsage.Total)) }
		m.Print(s)
	case "step-failed":
		m.Print(theme.ErrorText.Render("  ✗ " + evt.NodeTitle + ": " + evt.Error))
	case "step-awaiting-input":
		m.Print(theme.WarningText.Render("⏸ Gate: ") + evt.NodeTitle)
		if evt.GateInstructions != "" { m.Print(evt.GateInstructions) }
		m.Print(theme.Faint.Render("Type your response and press enter."))
	}
}

func printBanner(engine *eng.App, engineErr error) {
	w, _, _ := term.GetSize(os.Stdout.Fd())
	if w < 20 { w = 60 }
	_, rows, _ := term.GetSize(os.Stdout.Fd())
	if rows < 10 { rows = 24 }

	fmt.Print("\033[2J\033[H")
	bannerLines := 12
	for i := 0; i < rows-bannerLines; i++ { fmt.Println() }

	sep := theme.Faint.Render(strings.Repeat("─", w))
	fmt.Println(sep)
	fmt.Println()
	fmt.Println("  " + theme.Title.Render("skrptiq") + "  " + theme.Faint.Render("v0.1.0-prototype"))
	fmt.Println("  " + theme.Faint.Render("Interactive terminal for personalised AI agents"))
	fmt.Println()

	if engineErr != nil {
		fmt.Println("  " + theme.ErrorText.Render("Engine: "+engineErr.Error()))
	} else if engine != nil {
		workspace := "~"
		if cwd, err := os.Getwd(); err == nil {
			home, _ := os.UserHomeDir()
			if home != "" && strings.HasPrefix(cwd, home) {
				workspace = "~" + cwd[len(home):]
			} else { workspace = filepath.Base(cwd) }
		}
		profile := "default"
		if p, _ := engine.ActiveProfile("voice"); p != nil { profile = p.Name }
		labelStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(14)
		fmt.Println("  " + labelStyle.Render("Profile:") + profile)
		fmt.Println("  " + labelStyle.Render("Workspace:") + workspace)
	}
	fmt.Println()
	fmt.Println("  " + theme.Faint.Render("Type naturally to chat, or / for commands. /help for the full list."))
	fmt.Println()
}

// lipgloss is used by handlers.go.
var _ = lipgloss.NewStyle
