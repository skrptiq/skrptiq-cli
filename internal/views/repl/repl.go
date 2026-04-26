package repl

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/skrptiq/skrptiq-cli/internal/components"
	"github.com/skrptiq/skrptiq-cli/internal/theme"
)

// SubmitMsg is sent when the user submits a command.
type SubmitMsg struct {
	Input string
}

// OutputMsg adds output text to the history.
type OutputMsg struct {
	Text string
}

// PromptConfig controls the appearance of the input prompt.
type PromptConfig struct {
	// Symbol is the prompt character(s) shown before input (e.g. ">", "$", "λ").
	Symbol string
	// Style is the lipgloss style applied to the prompt symbol.
	Style lipgloss.Style
	// ContextLeft is optional text shown to the left of the prompt
	// (e.g. profile name, workspace).
	ContextLeft string
	// ContextRight is optional text shown right-aligned on the input line
	// (e.g. mode indicator, model name).
	ContextRight string
}

// DefaultPromptConfig returns the default prompt configuration.
func DefaultPromptConfig() PromptConfig {
	return PromptConfig{
		Symbol: "> ",
		Style:  theme.Prompt,
	}
}

// KeyMap defines REPL-specific key bindings.
type KeyMap struct {
	Submit      key.Binding
	HistoryUp   key.Binding
	HistoryDown key.Binding
	ScrollUp    key.Binding
	ScrollDown  key.Binding
}

// DefaultKeyMap returns default REPL key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "submit"),
		),
		HistoryUp: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "previous command"),
		),
		HistoryDown: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "next command"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("pgup", "shift+up"),
			key.WithHelp("pgup", "scroll up"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("pgdown", "shift+down"),
			key.WithHelp("pgdown", "scroll down"),
		),
	}
}

// Model is the REPL view model.
type Model struct {
	keys         KeyMap
	prompt       PromptConfig
	input        textinput.Model
	viewport     viewport.Model
	autocomplete components.Autocomplete
	history      []string   // display history (rendered output)
	cmdHistory   []string   // command history (raw inputs for up/down)
	cmdHistIdx   int        // -1 = new input, 0..n = browsing history
	savedInput   string     // saved current input when browsing history
	width        int
	height       int
	ready        bool
	prevInput     string     // tracks previous input value for change detection
	activity      string     // activity indicator text (empty = idle)
	activitySpinner spinner.Model
	bareCompleter func(string) []components.Completion // optional completer for non-/ input
}

// New creates a new REPL view model.
func New() Model {
	return NewWithPrompt(DefaultPromptConfig(), nil)
}

// NewWithPrompt creates a new REPL view model with a custom prompt and commands.
func NewWithPrompt(cfg PromptConfig, commands []components.Command) Model {
	ti := textinput.New()
	ti.Placeholder = "Ask a question or type / for commands..."
	ti.Prompt = cfg.Style.Render(cfg.Symbol)
	ti.Focus()
	ti.CharLimit = 500

	s := components.NewSpinner()

	return Model{
		keys:            DefaultKeyMap(),
		prompt:          cfg,
		input:           ti,
		autocomplete:    components.NewAutocomplete(commands),
		history:         []string{},
		cmdHistory:      []string{},
		cmdHistIdx:      -1,
		activitySpinner: s,
	}
}

// Prompt returns the current prompt configuration.
func (m Model) Prompt() PromptConfig {
	return m.prompt
}

// History returns the current history entries.
func (m Model) History() []string {
	return m.history
}

// SetBareCompleter sets a completer for non-/ input (e.g. workflow names in run mode).
// Pass nil to clear.
func (m *Model) SetBareCompleter(fn func(string) []components.Completion) {
	m.bareCompleter = fn
}

// SetActivity sets the activity indicator text. Empty string clears it.
func (m *Model) SetActivity(text string) {
	m.activity = text
}

// Activity returns the current activity text.
func (m Model) Activity() string {
	return m.activity
}

// SetPrompt updates the prompt configuration.
func (m *Model) SetPrompt(cfg PromptConfig) {
	m.prompt = cfg
	m.input.Prompt = cfg.Style.Render(cfg.Symbol)
}

// SetSize updates the REPL dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.autocomplete.SetWidth(width)

	// Viewport gets all height except the prompt line(s).
	promptLines := 1
	if m.prompt.ContextLeft != "" || m.prompt.ContextRight != "" {
		promptLines = 2
	}
	vpHeight := height - promptLines - 1
	if vpHeight < 1 {
		vpHeight = 1
	}

	if !m.ready {
		m.viewport = viewport.New(width, vpHeight)
		m.viewport.SetContent(m.renderHistory())
		m.ready = true
	} else {
		m.viewport.Width = width
		m.viewport.Height = vpHeight
		m.viewport.SetContent(m.renderHistory())
	}

	m.input.Width = width - lipgloss.Width(m.input.Prompt) - 1
}

// UpdateLastOutput replaces the last history entry (for streaming updates).
func (m *Model) UpdateLastOutput(text string) {
	if len(m.history) > 0 {
		m.history[len(m.history)-1] = text
		if m.ready {
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
		}
	}
}

// AddOutput appends output to the history.
func (m *Model) AddOutput(text string) {
	m.history = append(m.history, text)
	if m.ready {
		// Auto-scroll only if already at or near the bottom.
		atBottom := m.viewport.AtBottom()
		m.viewport.SetContent(m.renderHistory())
		if atBottom {
			m.viewport.GotoBottom()
		}
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Scroll keys always go to the viewport — never consumed by autocomplete.
		if key.Matches(msg, m.keys.ScrollUp) || key.Matches(msg, m.keys.ScrollDown) {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

		// Enter always submits — autocomplete never blocks it.
		if key.Matches(msg, m.keys.Submit) && m.input.Value() != "" {
			input := m.input.Value()
			m.cmdHistory = append(m.cmdHistory, input)
			m.cmdHistIdx = -1
			m.savedInput = ""
			m.history = append(m.history, m.prompt.Style.Render(m.prompt.Symbol)+input)
			m.input.SetValue("")
			m.prevInput = ""
			m.autocomplete.Hide()
			if m.ready {
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
			}
			return m, func() tea.Msg { return SubmitMsg{Input: input} }
		}

		// Autocomplete handles navigation keys (up/down/tab/esc).
		if m.autocomplete.Visible() {
			var cmd tea.Cmd
			var consumed bool
			m.autocomplete, cmd, consumed = m.autocomplete.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			if consumed {
				return m, tea.Batch(cmds...)
			}
		}

		// History navigation (only when autocomplete is hidden).
		if !m.autocomplete.Visible() {
			if key.Matches(msg, m.keys.HistoryUp) {
				if len(m.cmdHistory) > 0 {
					if m.cmdHistIdx == -1 {
						// Save current input before browsing.
						m.savedInput = m.input.Value()
						m.cmdHistIdx = len(m.cmdHistory) - 1
					} else if m.cmdHistIdx > 0 {
						m.cmdHistIdx--
					}
					m.input.SetValue(m.cmdHistory[m.cmdHistIdx])
					m.input.CursorEnd()
					m.prevInput = m.input.Value()
				}
				return m, nil
			}
			if key.Matches(msg, m.keys.HistoryDown) {
				if m.cmdHistIdx >= 0 {
					m.cmdHistIdx++
					if m.cmdHistIdx >= len(m.cmdHistory) {
						// Back to current input.
						m.cmdHistIdx = -1
						m.input.SetValue(m.savedInput)
					} else {
						m.input.SetValue(m.cmdHistory[m.cmdHistIdx])
					}
					m.input.CursorEnd()
					m.prevInput = m.input.Value()
				}
				return m, nil
			}
		}

	case components.AutocompleteSelectMsg:
		if msg.NeedsMore {
			// More input needed — set text with trailing space and trigger next stage.
			m.input.SetValue(msg.FullText + " ")
			m.input.CursorEnd()
			m.prevInput = m.input.Value()
			triggerNextStage(&m.autocomplete, msg.FullText)
		} else if msg.IsArg {
			// Argument selected — set input text, user presses enter to confirm.
			m.input.SetValue(msg.FullText)
			m.input.CursorEnd()
			m.prevInput = m.input.Value()
		} else {
			// Command selected (no args) — auto-submit.
			input := msg.FullText
			m.cmdHistory = append(m.cmdHistory, input)
			m.cmdHistIdx = -1
			m.savedInput = ""
			m.history = append(m.history, m.prompt.Style.Render(m.prompt.Symbol)+input)
			m.input.SetValue("")
			m.prevInput = ""
			if m.ready {
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
			}
			return m, func() tea.Msg { return SubmitMsg{Input: input} }
		}
		return m, nil

	case components.AutocompleteDismissMsg:
		return m, nil

	case OutputMsg:
		m.AddOutput(msg.Text)
		return m, nil

	case spinner.TickMsg:
		if m.activity != "" {
			var cmd tea.Cmd
			m.activitySpinner, cmd = m.activitySpinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	// Update text input.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	// Detect input changes to drive autocomplete.
	currentInput := m.input.Value()
	if currentInput != m.prevInput {
		m.prevInput = currentInput
		m.updateAutocomplete(currentInput)
	}

	// Update viewport.
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) updateAutocomplete(input string) {
	if !strings.HasPrefix(input, "/") {
		// Check for bare text completer (e.g. workflow names in run mode).
		if m.bareCompleter != nil && input != "" {
			completions := m.bareCompleter(input)
			if len(completions) > 0 {
				m.autocomplete.ShowBare(completions)
				return
			}
		}
		m.autocomplete.Hide()
		return
	}

	parts := strings.SplitN(input, " ", 3)
	cmdName := parts[0] // e.g. "/hub"

	cmd := m.autocomplete.FindCommand(cmdName)

	switch len(parts) {
	case 1:
		// Just "/hub" or "/h" — stage 1, filter top-level commands.
		if !m.autocomplete.Visible() {
			m.autocomplete.Show(input)
		} else {
			m.autocomplete.SetFilter(input)
		}

	case 2:
		// "/hub list" or "/hub l" or "/run " — check what the command expects.
		subOrArg := parts[1]

		if cmd == nil {
			m.autocomplete.Hide()
			return
		}

		if cmd.HasSubcommands() {
			// Show subcommand completions filtered by what's typed.
			m.autocomplete.ShowSubcommands(cmd, subOrArg)
		} else if cmd.ArgProvider != nil {
			// Show argument completions.
			m.autocomplete.ShowArgs(cmd, nil, subOrArg)
		} else {
			m.autocomplete.Hide()
		}

	case 3:
		// "/profile use Ben" — stage 3, argument after subcommand.
		if cmd == nil {
			m.autocomplete.Hide()
			return
		}
		subName := parts[1]
		argPartial := parts[2]
		sub := m.autocomplete.FindSubcommand(cmd, subName)
		if sub != nil && sub.ArgProvider != nil {
			m.autocomplete.ShowArgs(cmd, sub, argPartial)
		} else {
			m.autocomplete.Hide()
		}

	default:
		m.autocomplete.Hide()
	}
}

// triggerNextStage activates the next autocomplete stage after a selection.
func triggerNextStage(ac *components.Autocomplete, fullText string) {
	parts := strings.SplitN(fullText, " ", 3)
	cmdName := parts[0]
	cmd := ac.FindCommand(cmdName)
	if cmd == nil {
		return
	}

	switch len(parts) {
	case 1:
		if cmd.HasSubcommands() {
			ac.ShowSubcommands(cmd, "")
		} else if cmd.ArgProvider != nil {
			ac.ShowArgs(cmd, nil, "")
		}

	case 2:
		sub := ac.FindSubcommand(cmd, parts[1])
		if sub != nil && sub.ArgProvider != nil {
			ac.ShowArgs(cmd, sub, "")
		}
	}
}

func (m Model) View() string {
	if !m.ready {
		return "Initialising..."
	}

	var b strings.Builder
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Show scroll indicator when not at the bottom.
	if !m.viewport.AtBottom() {
		scrollPct := int(m.viewport.ScrollPercent() * 100)
		b.WriteString(theme.Faint.Render(fmt.Sprintf("  ↑ scroll — %d%%  pgup/pgdown to navigate", scrollPct)) + "\n")
	}

	// Render activity indicator above the input if engine is active.
	if m.activity != "" {
		b.WriteString(m.activitySpinner.View() + " " + theme.Subtitle.Render(m.activity) + "\n")
	}

	// Render autocomplete popup above the input if visible.
	if m.autocomplete.Visible() {
		popup := m.autocomplete.View()
		if popup != "" {
			b.WriteString(popup + "\n")
		}
	}

	// Render context line above input if configured.
	if m.prompt.ContextLeft != "" || m.prompt.ContextRight != "" {
		left := theme.Faint.Render(m.prompt.ContextLeft)
		right := theme.Faint.Render(m.prompt.ContextRight)

		leftWidth := lipgloss.Width(left)
		rightWidth := lipgloss.Width(right)
		gap := m.width - leftWidth - rightWidth
		if gap < 1 {
			gap = 1
		}

		b.WriteString(left + strings.Repeat(" ", gap) + right + "\n")
	}

	b.WriteString(m.input.View())
	return b.String()
}

func (m Model) renderHistory() string {
	if len(m.history) == 0 {
		welcome := lipgloss.NewStyle().
			Foreground(theme.Muted).
			Render(fmt.Sprintf("Welcome to skrptiq. Type naturally to chat, or %s for commands.",
				theme.ActionKey.Render("/")))
		return welcome
	}
	return strings.Join(m.history, "\n")
}
