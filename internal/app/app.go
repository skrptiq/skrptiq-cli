package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/chzyer/readline"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/llm"
	"github.com/skrptiq/engine/storage"

	eng "github.com/skrptiq/skrptiq-cli/internal/engine"
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

// App is the main application.
type App struct {
	engine   *eng.App
	rl       *readline.Instance
	commands []Command
	mode     AppMode

	// Mode state.
	chatProvider string
	runWorkflow  string

	// Execution state.
	executionID  string
	cancelStream context.CancelFunc

	// Input collection state.
	pendingInputs   []string
	collectedInputs map[string]string
	inputMeta       map[string]inputMetaInfo
	pendingNode     *storage.Node
}

type inputMetaInfo struct {
	Label       string
	Description string
	Example     string
}

// New creates a new App.
func New() (*App, error) {
	engine, engineErr := eng.Open("")

	a := &App{
		engine: engine,
	}

	a.commands = BuildCommands(engine)

	// Build readline completer from command registry.
	completer := a.buildCompleter()

	// Initial prompt — will be updated by updatePrompt after first resize.
	prompt := "⚡ › "

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt,
		HistoryFile:     historyPath(),
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return nil, fmt.Errorf("readline: %w", err)
	}
	a.rl = rl

	// Print startup banner.
	a.printBanner(engine, engineErr)

	return a, nil
}

// Close cleans up resources.
func (a *App) Close() {
	if a.rl != nil {
		a.rl.Close()
	}
	if a.engine != nil {
		a.engine.Close()
	}
}

// Print prints a line to the terminal (persists in scrollback).
func (a *App) Print(text string) {
	fmt.Println(text)
}

// printInputFrame prints the top separator before the prompt.
func (a *App) printInputFrame() {
	w := a.termWidth()
	sep := theme.Faint.Render(strings.Repeat("─", w))
	fmt.Println(sep)
}

// printStatusFooter prints the bottom separator + status bar after the user submits.
func (a *App) printStatusFooter() {
	w := a.termWidth()
	sep := theme.Faint.Render(strings.Repeat("─", w))

	var parts []string
	parts = append(parts, a.mode.Label())
	if a.engine != nil {
		if p, _ := a.engine.ActiveProfile("voice"); p != nil {
			parts = append(parts, p.Name)
		}
	}
	switch a.mode {
	case ModeChat:
		if a.chatProvider != "" {
			parts = append(parts, a.chatProvider)
		}
	case ModeRun:
		if a.runWorkflow != "" {
			parts = append(parts, a.runWorkflow)
		}
	}
	statusText := " " + strings.Join(parts, " · ")
	if len(statusText) < w {
		statusText += strings.Repeat(" ", w-len(statusText))
	}
	bar := lipgloss.NewStyle().
		Background(lipgloss.Color("#1F2937")).
		Foreground(lipgloss.Color("#9CA3AF")).
		Render(statusText)

	fmt.Println(sep)
	fmt.Println(bar)
}

func (a *App) termWidth() int {
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w < 20 {
		return 60
	}
	return w
}

func (a *App) printBanner(engine *eng.App, engineErr error) {
	w := a.termWidth()

	// Clear screen and move cursor to top.
	fmt.Print("\033[2J\033[H")

	// Get terminal height to push content to bottom.
	_, rows, err := term.GetSize(os.Stdout.Fd())
	if err != nil || rows < 10 {
		rows = 24
	}

	bannerLines := 14
	padding := rows - bannerLines
	if padding < 0 {
		padding = 0
	}
	for i := 0; i < padding; i++ {
		fmt.Println()
	}

	sep := theme.Faint.Render(strings.Repeat("─", w))

	// Banner block — scrolls off as user works.
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
			} else {
				workspace = filepath.Base(cwd)
			}
		}

		profile := "default"
		if p, _ := engine.ActiveProfile("voice"); p != nil {
			profile = p.Name
		}

		mcpStatus := ""
		if servers, err := engine.MCPServers(); err == nil && len(servers) > 0 {
			var parts []string
			for _, s := range servers {
				indicator := theme.ErrorText.Render("●")
				if s.Status == "connected" {
					indicator = theme.SuccessText.Render("●")
				}
				parts = append(parts, s.Name+" "+indicator)
			}
			mcpStatus = strings.Join(parts, "  ")
		}

		labelStyle := lipgloss.NewStyle().Foreground(theme.Muted).Width(14)
		fmt.Println("  " + labelStyle.Render("Profile:") + profile)
		fmt.Println("  " + labelStyle.Render("Workspace:") + workspace)
		if mcpStatus != "" {
			fmt.Println("  " + labelStyle.Render("MCP:") + mcpStatus)
		}
	}

	fmt.Println()
	fmt.Println("  " + theme.Faint.Render("Type naturally to chat, or "+theme.ActionKey.Render("/")+" for commands. "+theme.ActionKey.Render("/help")+" for the full list."))
	fmt.Println()
	a.printInputFrame()
}

// Run is the main input loop.
func (a *App) Run() {
	var lastEOF time.Time

	for {
		line, err := a.rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				// Ctrl+C — cancel active stream or ignore.
				if a.cancelStream != nil {
					a.cancelStream()
					a.cancelStream = nil
					a.Print(theme.Faint.Render("Cancelled."))
					if a.executionID != "" && a.engine != nil {
						a.engine.StopExecution(a.executionID)
						a.executionID = ""
					}
					a.setMode(ModeCommand)
					continue
				}
				fmt.Println()
				continue
			}
			if err == io.EOF {
				// Double Ctrl+D to exit — must press twice within 500ms.
				now := time.Now()
				if now.Sub(lastEOF) < 500*time.Millisecond {
					fmt.Println()
					a.Print(theme.Faint.Render("Goodbye."))
					return
				}
				lastEOF = now
				a.Print(theme.Faint.Render("Press Ctrl+D again to exit."))
				continue
			}
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Bottom separator + status bar (below the input the user just typed).
		a.printStatusFooter()

		// Handle the command — output goes between footer and next frame.
		a.handleInput(line)

		// Top separator for the next prompt.
		a.printInputFrame()
	}
}

func (a *App) handleInput(input string) {
	// Bare "/" shows the command list with descriptions.
	if input == "/" {
		a.listCommands()
		return
	}

	// Slash commands work in any mode.
	if strings.HasPrefix(input, "/") {
		stripped := input[1:]
		cmd := strings.ToLower(stripped)
		args := ""
		if idx := strings.Index(stripped, " "); idx > 0 {
			cmd = strings.ToLower(stripped[:idx])
			args = strings.TrimSpace(stripped[idx+1:])
		}

		if a.handleSlashCommand(cmd, args) {
			return
		}

		// Demo commands.
		switch cmd {
		case "demo", "tree", "gate", "diff":
			a.Print(theme.Faint.Render("/" + cmd + " — prototype TUI views not available in readline mode."))
			return
		case "resume", "stop":
			a.Print(theme.Faint.Render("/" + cmd + " — requires engine execution wiring."))
			return
		}

		a.Print(theme.ErrorText.Render("Unknown command: /" + cmd) + " — type /help for available commands.")
		return
	}

	// Bare text — depends on mode.
	switch a.mode {
	case ModeChat:
		a.handleChatInput(input)
	case ModeRun:
		if a.runWorkflow == "" {
			a.handleRunWorkflowSelect(input)
		} else if len(a.pendingInputs) > 0 {
			a.handleInputCollection(input)
		} else {
			a.handleRunInput(input)
		}
	default:
		a.Print(theme.Faint.Render("Type /chat to enter chat mode, or use / commands."))
	}
}

func (a *App) setMode(mode AppMode) {
	a.mode = mode
	a.updatePrompt()
}

func (a *App) updatePrompt() {
	prompt := a.mode.Symbol() + " › "
	if a.rl != nil {
		a.rl.SetPrompt(prompt)
	}
}


// handleChatInput sends input to the LLM.
func (a *App) handleChatInput(input string) {
	if a.engine == nil {
		a.Print(theme.ErrorText.Render("No engine connection."))
		return
	}

	a.Print(theme.Faint.Render("Thinking..."))

	ctx, cancel := context.WithCancel(context.Background())
	a.cancelStream = cancel
	defer func() { a.cancelStream = nil }()

	messages := []llm.Message{{Role: "user", Content: input}}
	var output strings.Builder

	resp, err := a.engine.Chat(ctx, messages, llm.Options{}, func(chunk string) {
		fmt.Print(chunk)
		output.WriteString(chunk)
	})
	fmt.Println() // newline after streaming

	if err != nil {
		a.Print(theme.ErrorText.Render("Error: " + err.Error()))
		return
	}

	if resp.Usage != nil {
		a.Print(theme.Faint.Render(fmt.Sprintf("  %s/%s · %d in / %d out tokens",
			resp.Provider, resp.Model, resp.Usage.InputTokens, resp.Usage.OutputTokens)))
	}
}

// handleRunWorkflowSelect treats input as a workflow name in run mode.
func (a *App) handleRunWorkflowSelect(input string) {
	if a.engine == nil {
		a.Print(theme.ErrorText.Render("No engine connection."))
		return
	}

	node, err := a.engine.FindNodeByTitle(input)
	if err != nil || node == nil || node.Type != "workflow" {
		a.Print(theme.ErrorText.Render("Workflow not found: " + input))
		return
	}

	a.runWorkflow = node.Title
	a.setMode(ModeRun)
	a.startExecution(node)
}

// handleInputCollection collects workflow input variables.
func (a *App) handleInputCollection(input string) {
	currentVar := a.pendingInputs[0]
	a.collectedInputs[currentVar] = input
	a.pendingInputs = a.pendingInputs[1:]

	if len(a.pendingInputs) > 0 {
		a.promptForInput()
		return
	}

	a.Print(theme.Faint.Render("All inputs collected. Starting execution..."))
	a.startExecutionWithInputs(a.pendingNode, a.collectedInputs)
	a.pendingNode = nil
}

func (a *App) promptForInput() {
	if len(a.pendingInputs) == 0 {
		return
	}
	varName := a.pendingInputs[0]
	label := varName
	desc := ""
	example := ""
	if meta, ok := a.inputMeta[varName]; ok {
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
	a.Print(prompt)
}

// handleRunInput processes gate responses during execution.
func (a *App) handleRunInput(input string) {
	if a.executionID == "" || a.engine == nil {
		a.Print(theme.Faint.Render("No active execution to respond to."))
		return
	}

	a.Print(theme.Faint.Render("Resuming..."))

	ctx, cancel := context.WithCancel(context.Background())
	a.cancelStream = cancel
	defer func() { a.cancelStream = nil }()

	execID := a.executionID

	onProgress := func(evt exec.ProgressEvent) {
		a.handleProgressEvent(evt)
	}

	_, err := a.engine.ResumeExecution(ctx, execID, input, onProgress)
	if err != nil {
		a.Print(theme.ErrorText.Render("Execution failed: " + err.Error()))
	} else {
		a.Print(theme.SuccessText.Render("Workflow completed."))
	}
	a.setMode(ModeCommand)
}

func (a *App) startExecution(node *storage.Node) {
	plan, err := a.engine.BuildPlan(node.ID)
	if err != nil {
		a.Print(theme.ErrorText.Render("Plan error: " + err.Error()))
		return
	}

	a.Print(theme.Title.Render("Run Mode") + " — " + plan.WorkflowTitle)

	// Show steps.
	var b strings.Builder
	for _, pg := range plan.PositionGroups {
		for _, step := range pg.Steps {
			gate := ""
			if step.IsGate {
				gate = theme.WarningText.Render(" (gate)")
			}
			b.WriteString(fmt.Sprintf("  %d. %s%s\n", step.Position, step.NodeTitle, gate))
		}
	}
	a.Print(b.String())

	// Check for required inputs.
	if len(plan.InputVariables) > 0 {
		a.pendingInputs = plan.InputVariables
		a.collectedInputs = make(map[string]string)
		a.pendingNode = node

		a.inputMeta = make(map[string]inputMetaInfo)
		for varName, meta := range plan.InputMeta {
			a.inputMeta[varName] = inputMetaInfo{
				Label:       meta.Label,
				Description: meta.Description,
				Example:     meta.Example,
			}
		}

		a.Print(theme.Faint.Render(fmt.Sprintf(
			"%d input(s) required. Enter each value and press enter.", len(plan.InputVariables))))
		a.promptForInput()
		return
	}

	a.startExecutionWithInputs(node, map[string]string{})
}

func (a *App) startExecutionWithInputs(node *storage.Node, inputs map[string]string) {
	a.Print(theme.Faint.Render("Starting execution..."))

	ctx, cancel := context.WithCancel(context.Background())
	a.cancelStream = cancel
	defer func() { a.cancelStream = nil }()

	onProgress := func(evt exec.ProgressEvent) {
		a.handleProgressEvent(evt)
	}

	_, err := a.engine.RunWorkflow(ctx, node.ID, inputs, onProgress)
	if err != nil {
		a.Print(theme.ErrorText.Render("Execution failed: " + err.Error()))
		a.setMode(ModeCommand)
		return
	}

	// Check if paused at gate (executionID will be set by progress handler).
	if a.executionID != "" {
		a.Print(theme.Faint.Render("Workflow paused at gate. Type your response."))
	} else {
		a.Print(theme.SuccessText.Render("Workflow completed."))
		a.setMode(ModeCommand)
	}
}

func (a *App) handleProgressEvent(evt exec.ProgressEvent) {
	switch evt.Type {
	case "execution-started":
		a.executionID = evt.ExecutionID

	case "step-started":
		a.Print(theme.Faint.Render("  ◌ ") + evt.NodeTitle)

	case "step-chunk":
		fmt.Print(evt.Chunk)

	case "step-completed":
		status := theme.SuccessText.Render("  ✓ ") + evt.NodeTitle
		if evt.Provider != "" {
			status += theme.Faint.Render(" (" + evt.Provider + ")")
		}
		if evt.TokenUsage != nil {
			status += theme.Faint.Render(fmt.Sprintf(" %d tokens", evt.TokenUsage.Total))
		}
		a.Print(status)

	case "step-failed":
		a.Print(theme.ErrorText.Render("  ✗ " + evt.NodeTitle + ": " + evt.Error))

	case "step-awaiting-input":
		a.Print(theme.WarningText.Render("⏸ Gate: ") + evt.NodeTitle)
		if evt.GateInstructions != "" {
			a.Print(evt.GateInstructions)
		}
		a.Print(theme.Faint.Render("Type your response and press enter to continue."))
	}
}

func (a *App) buildCompleter() *readline.PrefixCompleter {
	var items []readline.PrefixCompleterInterface

	for _, cmd := range a.commands {
		if cmd.HasSubcommands() {
			var subItems []readline.PrefixCompleterInterface
			for _, sub := range cmd.Subcommands {
				subItems = append(subItems, readline.PcItem(sub.Name))
			}
			items = append(items, readline.PcItem(cmd.Name, subItems...))
		} else {
			items = append(items, readline.PcItem(cmd.Name))
		}
	}

	return readline.NewPrefixCompleter(items...)
}

// listCommands prints all available commands with descriptions.
func (a *App) listCommands() {
	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB")).
		Background(lipgloss.Color("#374151")).
		Padding(0, 1)

	descStyle := lipgloss.NewStyle().Foreground(theme.Muted)

	for _, cmd := range a.commands {
		desc := cmd.Description
		if cmd.HasSubcommands() {
			var subs []string
			for _, sub := range cmd.Subcommands {
				subs = append(subs, sub.Name)
			}
			desc += " (" + strings.Join(subs, ", ") + ")"
		}
		fmt.Println(nameStyle.Render(cmd.Name) + " " + descStyle.Render(desc))
	}
}

func historyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".skrptiq")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "cli_history")
}

// lipgloss is used by handlers.go in this package.
var _ = lipgloss.NewStyle
