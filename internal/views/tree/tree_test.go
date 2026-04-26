package tree

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func mockTree() *Node {
	return &Node{
		Name:     "Pipeline",
		Status:   NodeRunning,
		Expanded: true,
		Children: []*Node{
			{Name: "Step 1", Status: NodeDone, Detail: "complete"},
			{
				Name:     "Step 2",
				Status:   NodeWarning,
				Expanded: false,
				Children: []*Node{
					{Name: "Sub 2a", Status: NodeWarning},
					{Name: "Sub 2b", Status: NodeDone},
				},
			},
			{Name: "Step 3", Status: NodePending},
		},
	}
}

func TestNew(t *testing.T) {
	m := New("Test Pipeline", mockTree())
	m.SetSize(80, 24)

	if len(m.visible) != 3 {
		t.Errorf("expected 3 visible nodes (children not expanded), got %d", len(m.visible))
	}
}

func TestExpandCollapse(t *testing.T) {
	m := New("Test Pipeline", mockTree())
	m.SetSize(80, 24)

	// Move cursor to Step 2 (index 1).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Fatalf("expected cursor at 1, got %d", m.cursor)
	}

	// Expand Step 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.visible) != 5 {
		t.Errorf("expected 5 visible nodes after expand, got %d", len(m.visible))
	}

	// Collapse Step 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.visible) != 3 {
		t.Errorf("expected 3 visible nodes after collapse, got %d", len(m.visible))
	}
}

func TestCursorBounds(t *testing.T) {
	m := New("Test Pipeline", mockTree())
	m.SetSize(80, 24)

	// Try to go above first node.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("cursor should stay at 0, got %d", m.cursor)
	}

	// Go to last node.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 2 {
		t.Fatalf("expected cursor at 2, got %d", m.cursor)
	}

	// Try to go past last node.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("cursor should stay at 2, got %d", m.cursor)
	}
}

func TestDismiss(t *testing.T) {
	m := New("Test Pipeline", mockTree())
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected dismiss command")
	}
	msg := cmd()
	if _, ok := msg.(DismissMsg); !ok {
		t.Errorf("expected DismissMsg, got %T", msg)
	}
}

func TestViewContainsNodes(t *testing.T) {
	m := New("Test Pipeline", mockTree())
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "Test Pipeline") {
		t.Error("expected title in view")
	}
	if !strings.Contains(view, "Step 1") {
		t.Error("expected 'Step 1' in view")
	}
}
