// Package prompt provides an inline bubbletea input area with separators
// and status bar. Output goes to terminal scrollback via tea.Println().
package prompt

import (
	"fmt"
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
		case tea.KeyTab, tea.KeyDown:
			if len(m.tabMatches) > 0 {
				// Move selection down (or select first if none selected).
				m.tabIndex++
				if m.tabIndex >= len(m.tabMatches) {
					m.tabIndex = 0
				}
				m.selectMatch()
				return m, nil
			} else if msg.Type == tea.KeyTab && m.tabComplete != nil {
				// First tab with no list — trigger completion.
				m.tabMatches = m.tabComplete(m.textarea.Value())
				if len(m.tabMatches) > 0 {
					m.tabIndex = 0
					m.tabOriginal = m.textarea.Value()
					m.selectMatch()
				}
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
		}

		// Any other key clears completion state.
		if len(m.tabMatches) > 0 {
			m.tabMatches = nil
			m.tabIndex = 0
		}
	}

	// Track value before textarea processes the key.
	prevVal := m.textarea.Value()

	// Pass everything else to textarea.
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)

	// Auto-show completions ONLY when the value actually changed (user typed something).
	newVal := m.textarea.Value()
	if m.tabComplete != nil && newVal != prevVal {
		if strings.HasPrefix(newVal, "/") {
			m.tabMatches = m.tabComplete(newVal)
			m.tabIndex = -1
		} else {
			m.tabMatches = nil
			m.tabIndex = -1
		}
	}

	return m, cmd
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

	// Show completion matches as a vertical list, max 5 visible.
	completionView := ""
	if len(m.tabMatches) > 0 {
		const maxVisible = 5
		matchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
		selectedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB")).
			Background(lipgloss.Color("#374151")).
			Width(m.width)

		// Calculate visible window around the selected item.
		idx := m.tabIndex
		if idx < 0 { idx = 0 }
		start := 0
		if idx >= maxVisible {
			start = idx - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(m.tabMatches) {
			end = len(m.tabMatches)
			start = end - maxVisible
			if start < 0 {
				start = 0
			}
		}

		var lines []string
		for i := start; i < end; i++ {
			if i == m.tabIndex {
				lines = append(lines, selectedStyle.Render(" › "+m.tabMatches[i]))
			} else {
				lines = append(lines, matchStyle.Render("   "+m.tabMatches[i]))
			}
		}
		if len(m.tabMatches) > maxVisible {
			lines = append(lines, matchStyle.Render(fmt.Sprintf("   %d/%d", m.tabIndex+1, len(m.tabMatches))))
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
