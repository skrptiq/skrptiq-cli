package gate

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/skrptiq/skrptiq-cli/internal/theme"
)

// Action represents the user's gate decision.
type Action int

const (
	ActionApprove Action = iota
	ActionReject
	ActionEdit
)

// ResultMsg is sent when the user completes a gate action.
type ResultMsg struct {
	Action  Action
	Content string // edited content if Action == ActionEdit
}

// CancelMsg is sent when the user cancels the gate.
type CancelMsg struct{}

// KeyMap defines gate-specific key bindings.
type KeyMap struct {
	Approve key.Binding
	Edit    key.Binding
	Cancel  key.Binding
	View    key.Binding
}

// DefaultKeyMap returns default gate key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Approve: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "approve"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "cancel"),
		),
		View: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "view full"),
		),
	}
}

// Model is the gate approval view.
type Model struct {
	keys     KeyMap
	title    string
	content  string
	viewport viewport.Model
	width    int
	height   int
	ready    bool
	viewing  bool // expanded view mode
}

// New creates a new gate view.
func New(title, content string) Model {
	return Model{
		keys:    DefaultKeyMap(),
		title:   title,
		content: content,
	}
}

// SetSize updates the gate view dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	vpHeight := height - 5 // title + border + actions + padding
	if vpHeight < 1 {
		vpHeight = 1
	}

	if !m.ready {
		m.viewport = viewport.New(width-4, vpHeight)
		m.viewport.SetContent(m.content)
		m.ready = true
	} else {
		m.viewport.Width = width - 4
		m.viewport.Height = vpHeight
		m.viewport.SetContent(m.content)
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// editorFinishedMsg carries the result of editing.
type editorFinishedMsg struct {
	content string
	err     error
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Approve):
			return m, func() tea.Msg {
				return ResultMsg{Action: ActionApprove, Content: m.content}
			}
		case key.Matches(msg, m.keys.Cancel):
			return m, func() tea.Msg { return CancelMsg{} }
		case key.Matches(msg, m.keys.View):
			m.viewing = !m.viewing
			if m.viewing {
				m.viewport.SetContent(m.content)
			} else {
				m.viewport.SetContent(m.previewContent())
			}
			return m, nil
		case key.Matches(msg, m.keys.Edit):
			return m, m.openEditor()
		}

	case editorFinishedMsg:
		if msg.err != nil {
			return m, func() tea.Msg {
				return ResultMsg{Action: ActionReject, Content: m.content}
			}
		}
		m.content = msg.content
		m.viewport.SetContent(m.content)
		return m, func() tea.Msg {
			return ResultMsg{Action: ActionEdit, Content: msg.content}
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

	title := theme.Title.Render("⏸ Gate: " + m.title)

	border := theme.Faint.Render(strings.Repeat("─", m.width))

	actions := fmt.Sprintf(
		"  %s %s   %s %s   %s %s   %s %s",
		theme.ActionKey.Render("[V]"),
		theme.ActionLabel.Render("view full"),
		theme.ActionKey.Render("[A]"),
		theme.ActionLabel.Render("approve"),
		theme.ActionKey.Render("[E]"),
		theme.ActionLabel.Render("edit"),
		theme.ActionKey.Render("[C]"),
		theme.ActionLabel.Render("cancel"),
	)

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s",
		title,
		border,
		m.viewport.View(),
		border,
		actions,
	)
}

func (m Model) previewContent() string {
	lines := strings.Split(m.content, "\n")
	maxLines := 20
	if len(lines) <= maxLines {
		return m.content
	}
	preview := strings.Join(lines[:maxLines], "\n")
	remaining := len(lines) - maxLines
	preview += fmt.Sprintf("\n\n%s", theme.Faint.Render(fmt.Sprintf("... %d more lines (press v to view full)", remaining)))
	return preview
}

func (m Model) openEditor() tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	tmpFile, err := os.CreateTemp("", "skrptiq-gate-*.md")
	if err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
		}
	}

	if _, err := tmpFile.WriteString(m.content); err != nil {
		tmpFile.Close()
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
		}
	}
	tmpFile.Close()

	c := exec.Command(editor, tmpFile.Name())
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return editorFinishedMsg{err: err}
		}
		content, readErr := os.ReadFile(tmpFile.Name())
		os.Remove(tmpFile.Name())
		return editorFinishedMsg{content: string(content), err: readErr}
	})
}
