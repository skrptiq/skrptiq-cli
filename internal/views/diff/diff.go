package diff

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/skrptiq/skrptiq-cli/internal/theme"
)

// Action represents the user's diff decision.
type Action int

const (
	ActionAccept Action = iota
	ActionReject
)

// ResultMsg is sent when the user acts on the diff.
type ResultMsg struct {
	Action Action
	File   string
}

// DismissMsg signals the diff view should be dismissed.
type DismissMsg struct{}

// Hunk represents a diff hunk.
type Hunk struct {
	Header string
	Lines  []DiffLine
}

// DiffLine is a single line in a diff.
type DiffLine struct {
	Type    LineType
	Content string
}

// LineType identifies the kind of diff line.
type LineType int

const (
	LineContext LineType = iota
	LineAdd
	LineRemove
)

// KeyMap defines diff-specific key bindings.
type KeyMap struct {
	Accept  key.Binding
	Reject  key.Binding
	Dismiss key.Binding
}

// DefaultKeyMap returns default diff key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Accept: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "accept"),
		),
		Reject: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "reject"),
		),
		Dismiss: key.NewBinding(
			key.WithKeys("esc", "q"),
			key.WithHelp("esc/q", "dismiss"),
		),
	}
}

// Model is the diff review view.
type Model struct {
	keys     KeyMap
	file     string
	hunks    []Hunk
	viewport viewport.Model
	width    int
	height   int
	ready    bool
}

// New creates a new diff view.
func New(file string, hunks []Hunk) Model {
	return Model{
		keys:  DefaultKeyMap(),
		file:  file,
		hunks: hunks,
	}
}

// SetSize updates the diff view dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	vpHeight := height - 4 // header + border + actions
	if vpHeight < 1 {
		vpHeight = 1
	}

	if !m.ready {
		m.viewport = viewport.New(width, vpHeight)
		m.viewport.SetContent(m.renderDiff())
		m.ready = true
	} else {
		m.viewport.Width = width
		m.viewport.Height = vpHeight
		m.viewport.SetContent(m.renderDiff())
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Accept):
			return m, func() tea.Msg {
				return ResultMsg{Action: ActionAccept, File: m.file}
			}
		case key.Matches(msg, m.keys.Reject):
			return m, func() tea.Msg {
				return ResultMsg{Action: ActionReject, File: m.file}
			}
		case key.Matches(msg, m.keys.Dismiss):
			return m, func() tea.Msg { return DismissMsg{} }
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if !m.ready {
		return "Initialising..."
	}

	header := lipgloss.NewStyle().
		Foreground(theme.DiffHeader).
		Bold(true).
		Render("diff --skrptiq " + m.file)

	border := theme.Faint.Render(strings.Repeat("─", m.width))

	actions := fmt.Sprintf(
		"  %s %s   %s %s   %s %s",
		theme.ActionKey.Render("[A]"),
		theme.ActionLabel.Render("accept"),
		theme.ActionKey.Render("[R]"),
		theme.ActionLabel.Render("reject"),
		theme.ActionKey.Render("[Esc]"),
		theme.ActionLabel.Render("dismiss"),
	)

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s",
		header,
		border,
		m.viewport.View(),
		border,
		actions,
	)
}

func (m Model) renderDiff() string {
	var b strings.Builder

	addStyle := lipgloss.NewStyle().Foreground(theme.DiffAdd)
	removeStyle := lipgloss.NewStyle().Foreground(theme.DiffRemove)
	headerStyle := lipgloss.NewStyle().Foreground(theme.DiffHeader)

	for _, hunk := range m.hunks {
		if hunk.Header != "" {
			b.WriteString(headerStyle.Render(hunk.Header) + "\n")
		}

		for _, line := range hunk.Lines {
			switch line.Type {
			case LineAdd:
				b.WriteString(addStyle.Render("+ "+line.Content) + "\n")
			case LineRemove:
				b.WriteString(removeStyle.Render("- "+line.Content) + "\n")
			case LineContext:
				b.WriteString("  " + line.Content + "\n")
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
