// Package prompt provides an inline bubbletea input area with separators
// and status bar. Output goes to terminal scrollback via tea.Println().
package prompt

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
)

// TabCompleteFunc returns completions for the current input.
type TabCompleteFunc func(input string) []string

// SubmitMsg is sent when the user presses enter.
type SubmitMsg struct {
	Text string
}

// CtrlCMsg is sent on Ctrl+C.
type CtrlCMsg struct{}

// CtrlDMsg is sent on Ctrl+D.
type CtrlDMsg struct{}

// EscMsg is sent on Escape.
type EscMsg struct{}

// Model is the inline prompt — separator + textarea + separator + status.
type Model struct {
	textarea    textarea.Model
	width       int
	status      string
	symbol      string
	tabComplete TabCompleteFunc
	tabMatches  []string
	tabIndex    int
	tabOriginal string
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

// SetTabComplete sets the tab completion function.
func (m *Model) SetTabComplete(fn TabCompleteFunc) {
	m.tabComplete = fn
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
				m.tabMatches = nil
				return m, func() tea.Msg { return SubmitMsg{Text: text} }
			}
			return m, nil
		case tea.KeyEscape:
			m.tabMatches = nil
			return m, func() tea.Msg { return EscMsg{} }
		case tea.KeyTab:
			if m.tabComplete != nil {
				m.handleTab()
				return m, nil
			}
		case tea.KeyUp:
			if len(m.tabMatches) > 0 {
				m.tabIndex--
				if m.tabIndex < 0 {
					m.tabIndex = len(m.tabMatches) - 1
				}
				m.selectMatch()
				return m, nil
			}
		case tea.KeyDown:
			if len(m.tabMatches) > 0 {
				m.tabIndex++
				if m.tabIndex >= len(m.tabMatches) {
					m.tabIndex = 0
				}
				m.selectMatch()
				return m, nil
			}
		}

		// Any non-navigation key clears completion state.
		if msg.Type != tea.KeyTab && msg.Type != tea.KeyUp && msg.Type != tea.KeyDown {
			m.tabMatches = nil
			m.tabIndex = 0
		}
	}

	// Pass everything else to textarea.
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)

	// Auto-show completions when input starts with /.
	if m.tabComplete != nil {
		val := m.textarea.Value()
		if strings.HasPrefix(val, "/") {
			m.tabMatches = m.tabComplete(val)
			m.tabIndex = -1 // no selection yet, just showing
		} else {
			m.tabMatches = nil
		}
	}

	return m, cmd
}

func (m *Model) handleTab() {
	if len(m.tabMatches) == 0 {
		if m.tabComplete != nil {
			m.tabMatches = m.tabComplete(m.textarea.Value())
			m.tabIndex = 0
			m.tabOriginal = m.textarea.Value()
		}
	} else {
		// Cycle through matches.
		m.tabIndex = (m.tabIndex + 1) % len(m.tabMatches)
	}

	if len(m.tabMatches) == 0 {
		return
	}
	if m.tabIndex < 0 {
		m.tabIndex = 0
	}

	m.selectMatch()
}

func (m *Model) selectMatch() {
	if m.tabIndex < 0 || m.tabIndex >= len(m.tabMatches) {
		return
	}
	m.textarea.Reset()
	m.textarea.SetValue(m.tabMatches[m.tabIndex])
	m.textarea.SetCursor(len(m.tabMatches[m.tabIndex]))
}

func (m Model) View() string {
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
	sep := sepStyle.Render(strings.Repeat("─", m.width))

	statusBar := lipgloss.NewStyle().
		Background(lipgloss.Color("#1F2937")).
		Foreground(lipgloss.Color("#9CA3AF")).
		Width(m.width).
		Render(" " + m.status)

	// Show completion matches as a vertical list.
	completionView := ""
	if len(m.tabMatches) > 0 {
		matchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
		selectedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB")).
			Background(lipgloss.Color("#374151")).
			Width(m.width)

		var lines []string
		for i, match := range m.tabMatches {
			if i == m.tabIndex {
				lines = append(lines, selectedStyle.Render(" › "+match))
			} else {
				lines = append(lines, matchStyle.Render("   "+match))
			}
		}
		completionView = strings.Join(lines, "\n") + "\n"
	}

	return sep + "\n" + m.textarea.View() + "\n" + sep + "\n" + completionView + statusBar
}

func termWidth() int {
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w < 20 {
		return 60
	}
	return w
}
