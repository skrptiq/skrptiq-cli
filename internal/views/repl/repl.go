package repl

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	keys     KeyMap
	input    textinput.Model
	viewport viewport.Model
	history  []string
	width    int
	height   int
	ready    bool
}

// New creates a new REPL view model.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.Prompt = theme.Prompt.Render("> ")
	ti.Focus()
	ti.CharLimit = 500

	return Model{
		keys:    DefaultKeyMap(),
		input:   ti,
		history: []string{},
	}
}

// SetSize updates the REPL dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Viewport gets all height except the input line (1 line + 1 padding).
	vpHeight := height - 2
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

	m.input.Width = width - 4 // account for prompt
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
		if key.Matches(msg, m.keys.Submit) && m.input.Value() != "" {
			input := m.input.Value()
			m.history = append(m.history, theme.Prompt.Render("> ")+input)
			m.input.SetValue("")
			if m.ready {
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
			}
			return m, func() tea.Msg { return SubmitMsg{Input: input} }
		}

	case OutputMsg:
		m.AddOutput(msg.Text)
		return m, nil
	}

	// Update text input.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport.
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return "Initialising..."
	}

	return fmt.Sprintf("%s\n%s", m.viewport.View(), m.input.View())
}

func (m Model) renderHistory() string {
	if len(m.history) == 0 {
		welcome := lipgloss.NewStyle().
			Foreground(theme.Muted).
			Render("Welcome to skrptiq. Type help for available commands.")
		return welcome
	}
	return strings.Join(m.history, "\n")
}
