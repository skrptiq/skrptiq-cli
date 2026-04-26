package tree

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/skrptiq/skrptiq-cli/internal/theme"
)

// NodeStatus represents the state of a tree node.
type NodeStatus int

const (
	NodePending NodeStatus = iota
	NodeRunning
	NodeDone
	NodeWarning
	NodeFailed
)

// Node represents a node in the execution tree.
type Node struct {
	Name     string
	Status   NodeStatus
	Detail   string
	Children []*Node
	Expanded bool
}

// DismissMsg signals the tree view should be dismissed.
type DismissMsg struct{}

// KeyMap defines tree-specific key bindings.
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Toggle   key.Binding
	Dismiss  key.Binding
}

// DefaultKeyMap returns default tree key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter/space", "expand/collapse"),
		),
		Dismiss: key.NewBinding(
			key.WithKeys("esc", "q"),
			key.WithHelp("esc/q", "dismiss"),
		),
	}
}

// Model is the execution tree view.
type Model struct {
	keys     KeyMap
	root     *Node
	title    string
	cursor   int
	visible  []*flatNode // flattened visible nodes for cursor navigation
	viewport viewport.Model
	width    int
	height   int
	ready    bool
}

type flatNode struct {
	node   *Node
	depth  int
	isLast bool
	prefix string
}

// New creates a new tree view.
func New(title string, root *Node) Model {
	return Model{
		keys:  DefaultKeyMap(),
		root:  root,
		title: title,
	}
}

// SetSize updates the tree view dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	vpHeight := height - 3 // title + help line + padding
	if vpHeight < 1 {
		vpHeight = 1
	}

	if !m.ready {
		m.viewport = viewport.New(width, vpHeight)
		m.ready = true
	} else {
		m.viewport.Width = width
		m.viewport.Height = vpHeight
	}

	m.rebuildVisible()
	m.updateViewport()
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
				m.updateViewport()
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.visible)-1 {
				m.cursor++
				m.updateViewport()
			}
		case key.Matches(msg, m.keys.Toggle):
			if m.cursor < len(m.visible) {
				node := m.visible[m.cursor].node
				if len(node.Children) > 0 {
					node.Expanded = !node.Expanded
					m.rebuildVisible()
					m.updateViewport()
				}
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

	title := theme.Title.Render(m.title)
	help := theme.Faint.Render("↑/↓ navigate  enter expand/collapse  esc dismiss")

	return title + "\n\n" + m.viewport.View() + "\n" + help
}

func (m *Model) rebuildVisible() {
	m.visible = nil
	if m.root == nil {
		return
	}
	m.flattenChildren(m.root.Children, 0, "")

	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) flattenChildren(nodes []*Node, depth int, parentPrefix string) {
	for i, child := range nodes {
		isLast := i == len(nodes)-1

		var connector string
		if isLast {
			connector = "└─ "
		} else {
			connector = "├─ "
		}

		prefix := parentPrefix + connector

		var childPrefix string
		if isLast {
			childPrefix = parentPrefix + "   "
		} else {
			childPrefix = parentPrefix + "│  "
		}

		m.visible = append(m.visible, &flatNode{
			node:   child,
			depth:  depth,
			isLast: isLast,
			prefix: prefix,
		})

		if child.Expanded && len(child.Children) > 0 {
			m.flattenChildren(child.Children, depth+1, childPrefix)
		}
	}
}

func (m *Model) updateViewport() {
	var lines []string
	for i, fn := range m.visible {
		line := m.renderNode(fn, i == m.cursor)
		lines = append(lines, line)
	}
	m.viewport.SetContent(strings.Join(lines, "\n"))
}

func (m Model) renderNode(fn *flatNode, selected bool) string {
	node := fn.node

	icon := statusIcon(node.Status)

	expandHint := ""
	if len(node.Children) > 0 {
		if node.Expanded {
			expandHint = "▼ "
		} else {
			expandHint = fmt.Sprintf("▶ (%d) ", len(node.Children))
		}
	}

	name := node.Name
	if selected {
		name = lipgloss.NewStyle().Bold(true).Underline(true).Render(name)
	}

	detail := ""
	if node.Detail != "" {
		detail = " " + theme.Faint.Render(node.Detail)
	}

	prefix := theme.TreeBranch.Render(fn.prefix)
	return fmt.Sprintf("%s%s%s%s%s", prefix, expandHint, icon, name, detail)
}

func statusIcon(status NodeStatus) string {
	switch status {
	case NodePending:
		return theme.Faint.Render("○ ")
	case NodeRunning:
		return theme.Subtitle.Render("◌ ")
	case NodeDone:
		return theme.SuccessText.Render("✓ ")
	case NodeWarning:
		return theme.WarningText.Render("⚠ ")
	case NodeFailed:
		return theme.ErrorText.Render("✗ ")
	default:
		return "  "
	}
}
