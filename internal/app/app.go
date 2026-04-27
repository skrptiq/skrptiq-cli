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
	"github.com/lmorg/readline/v4"

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
		mode:   ModeCommand,
	}

	a.commands = BuildCommands(engine)

	rl := readline.NewInstance()
	rl.SetPrompt(a.mode.Symbol() + " › ")

	// Hint text — shows the status bar BELOW the prompt.
	rl.HintText = func(line []rune, pos int) []rune {
		return []rune(a.hintText())
	}
	rl.HintFormatting = "\033[2m" // dim

	// Tab completion.
	rl.TabCompleter = a.tabCompleter

	// History.
	rl.History = new(readline.ExampleHistory)

	a.rl = rl

	// Print startup banner.
	a.printBanner(engine, engineErr)

	return a, nil
}

// Close cleans up resources.
func (a *App) Close() {
	if a.engine != nil {
		a.engine.Close()
	}
}

// Print prints a line to the terminal (persists in scrollback).
func (a *App) Print(text string) {
	fmt.Println(text)
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

	// Clear screen and push content to bottom.
	fmt.Print("\033[2J\033[H")

	_, rows, err := term.GetSize(os.Stdout.Fd())
	if err != nil || rows < 10 {
		rows = 24
	}

	bannerLines := 12
	padding := rows - bannerLines
	if padding < 0 {
		padding = 0
	}
	for i := 0; i < padding; i++ {
		fmt.Println()
	}

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
	fmt.Println(sep)
}

// hintText returns the status text shown below the prompt.
func (a *App) hintText() string {
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

	return strings.Join(parts, " · ")
}

// tabCompleter provides tab completion for commands.
func (a *App) tabCompleter(line []rune, pos int, _ readline.DelayedTabContext) *readline.TabCompleterReturnT {
	input := string(line[:pos])

	if !strings.HasPrefix(input, "/") {
		return nil
	}

	var items []string
	parts := strings.SplitN(input, " ", 2)
	cmdName := parts[0]

	if len(parts) == 1 {
		// Stage 1: match command names.
		for _, cmd := range a.commands {
			if strings.HasPrefix(strings.ToLower(cmd.Name), strings.ToLower(input)) {
				items = append(items, cmd.Name)
			}
		}
	} else {
		// Stage 2: match subcommands.
		subInput := ""
		if len(parts) > 1 {
			subInput = parts[1]
		}
		for _, cmd := range a.commands {
			if strings.EqualFold(cmd.Name, cmdName) {
				for _, sub := range cmd.Subcommands {
					if subInput == "" || strings.HasPrefix(strings.ToLower(sub.Name), strings.ToLower(subInput)) {
						items = append(items, cmd.Name+" "+sub.Name)
					}
				}
				break
			}
		}
	}

	if len(items) == 0 {
		return nil
	}

	return &readline.TabCompleterReturnT{
		Prefix:      input,
		Suggestions: items,
	}
}

// Run is the main input loop.
func (a *App) Run() {
	var lastEOF time.Time

	for {
		line, err := a.rl.Readline()
		if err != nil {
			if err == readline.ErrCtrlC {
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

		a.handleInput(line)
	}
}

func (a *App) handleInput(input string) {
	// Bare "/" shows the command list.
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

		switch cmd {
		case "demo", "tree", "gate", "diff":
			a.Print(theme.Faint.Render("/" + cmd + " — prototype TUI views not available in readline mode."))
			return
		case "resume", "stop":
			a.Print(theme.Faint.Render("/" + cmd + " — requires engine execution wiring."))
			return
		}

		a.Print(theme.ErrorText.Render("Unknown command: /"+cmd) + " — type /help for available commands.")
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
	if a.rl != nil {
		a.rl.SetPrompt(a.mode.Symbol() + " › ")
	}
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

	resp, err := a.engine.Chat(ctx, messages, llm.Options{}, func(chunk string) {
		fmt.Print(chunk)
	})
	fmt.Println()

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
