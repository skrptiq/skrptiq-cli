package repl

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
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
	Submit key.Binding
}

// DefaultKeyMap returns default REPL key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "submit"),
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
	history      []string
	width        int
	height       int
	ready        bool
	prevInput    string // tracks previous input value for change detection
}

// New creates a new REPL view model.
func New() Model {
	return NewWithPrompt(DefaultPromptConfig(), nil)
}

// NewWithPrompt creates a new REPL view model with a custom prompt and commands.
func NewWithPrompt(cfg PromptConfig, commands []components.Command) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a command or ask a question..."
	ti.Prompt = cfg.Style.Render(cfg.Symbol)
	ti.Focus()
	ti.CharLimit = 500

	return Model{
		keys:         DefaultKeyMap(),
		prompt:       cfg,
		input:        ti,
		autocomplete: components.NewAutocomplete(commands),
		history:      []string{},
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

// AddOutput appends output to the history.
func (m *Model) AddOutput(text string) {
	m.history = append(m.history, text)
	if m.ready {
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If autocomplete is visible, let it handle keys first.
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

		// Submit command.
		if key.Matches(msg, m.keys.Submit) && m.input.Value() != "" {
			input := m.input.Value()
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

	case components.AutocompleteSelectMsg:
		// Insert the selected command into the input.
		m.input.SetValue(msg.Command + " ")
		m.input.CursorEnd()
		m.prevInput = m.input.Value()
		return m, nil

	case components.AutocompleteDismissMsg:
		return m, nil

	case OutputMsg:
		m.AddOutput(msg.Text)
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
	if strings.HasPrefix(input, "/") && !strings.Contains(input, " ") {
		// Show autocomplete and filter by what's typed so far.
		if !m.autocomplete.Visible() {
			m.autocomplete.Show(input)
		} else {
			m.autocomplete.SetFilter(input)
		}
	} else {
		m.autocomplete.Hide()
	}
}

func (m Model) View() string {
	if !m.ready {
		return "Initialising..."
	}

	var b strings.Builder
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

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
			Render(fmt.Sprintf("Welcome to skrptiq. Type %s for available commands.",
				theme.ActionKey.Render("/help")))
		return welcome
	}
	return strings.Join(m.history, "\n")
}
