// Package prompt provides an inline bubbletea input area with separators
// and status bar. Output goes to terminal scrollback via tea.Println().
package prompt

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"os"
)

// SubmitMsg is sent when the user presses enter.
type SubmitMsg struct {
	Text string
}

// Model is the inline prompt — separator + textarea + separator + status.
type Model struct {
	textarea textarea.Model
	width    int
	status   string
	symbol   string
}

// New creates a new prompt model.
func New(symbol, status string) Model {
	w := termWidth()

	ta := textarea.New()
	ta.Placeholder = "Ask a question or type / for commands..."
	ta.Prompt = symbol + " › "
	ta.ShowLineNumbers = false
	ta.CharLimit = 2000
	ta.MaxHeight = 8
	ta.SetHeight(1)
	ta.SetWidth(w - 2)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.Focus()
	ta.KeyMap.InsertNewline.SetEnabled(false)

	return Model{
		textarea: ta,
		width:    w,
		status:   status,
		symbol:   symbol,
	}
}

// SetStatus updates the status bar text.
func (m *Model) SetStatus(s string) {
	m.status = s
}

// SetSymbol updates the prompt symbol.
func (m *Model) SetSymbol(s string) {
	m.symbol = s
	m.textarea.Prompt = s + " › "
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.textarea.SetWidth(msg.Width - 2)
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, func() tea.Msg { return CtrlCMsg{} }
		case tea.KeyCtrlD:
			return m, func() tea.Msg { return CtrlDMsg{} }
		case tea.KeyEnter:
			text := strings.TrimSpace(m.textarea.Value())
			if text != "" {
				m.textarea.Reset()
				return m, func() tea.Msg { return SubmitMsg{Text: text} }
			}
			return m, nil
		case tea.KeyEscape:
			return m, func() tea.Msg { return EscMsg{} }
		}
	}

	// Pass everything else to textarea.
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563")).
		Render(strings.Repeat("─", m.width))

	statusBar := lipgloss.NewStyle().
		Background(lipgloss.Color("#1F2937")).
		Foreground(lipgloss.Color("#9CA3AF")).
		Width(m.width).
		Render(" " + m.status)

	return sep + "\n" + m.textarea.View() + "\n" + sep + "\n" + statusBar
}

// CtrlCMsg is sent on Ctrl+C.
type CtrlCMsg struct{}

// CtrlDMsg is sent on Ctrl+D.
type CtrlDMsg struct{}

// EscMsg is sent on Escape.
type EscMsg struct{}

func termWidth() int {
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w < 20 {
		return 60
	}
	return w
}
