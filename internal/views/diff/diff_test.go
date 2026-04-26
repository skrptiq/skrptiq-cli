package diff

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func mockHunks() []Hunk {
	return []Hunk{
		{
			Header: "@@ -1,3 +1,4 @@",
			Lines: []DiffLine{
				{Type: LineContext, Content: "line 1"},
				{Type: LineRemove, Content: "old line 2"},
				{Type: LineAdd, Content: "new line 2"},
				{Type: LineAdd, Content: "new line 3"},
				{Type: LineContext, Content: "line 4"},
			},
		},
	}
}

func TestNew(t *testing.T) {
	m := New("test.go", mockHunks())
	m.SetSize(80, 24)

	if m.file != "test.go" {
		t.Errorf("expected file 'test.go', got %q", m.file)
	}
	if len(m.hunks) != 1 {
		t.Errorf("expected 1 hunk, got %d", len(m.hunks))
	}
}

func TestAccept(t *testing.T) {
	m := New("test.go", mockHunks())
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected command from accept")
	}
	msg := cmd()
	result, ok := msg.(ResultMsg)
	if !ok {
		t.Fatalf("expected ResultMsg, got %T", msg)
	}
	if result.Action != ActionAccept {
		t.Errorf("expected ActionAccept, got %d", result.Action)
	}
	if result.File != "test.go" {
		t.Errorf("expected file 'test.go', got %q", result.File)
	}
}

func TestReject(t *testing.T) {
	m := New("test.go", mockHunks())
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected command from reject")
	}
	msg := cmd()
	result, ok := msg.(ResultMsg)
	if !ok {
		t.Fatalf("expected ResultMsg, got %T", msg)
	}
	if result.Action != ActionReject {
		t.Errorf("expected ActionReject, got %d", result.Action)
	}
}

func TestDismiss(t *testing.T) {
	m := New("test.go", mockHunks())
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected command from dismiss")
	}
	msg := cmd()
	if _, ok := msg.(DismissMsg); !ok {
		t.Errorf("expected DismissMsg, got %T", msg)
	}
}

func TestViewRendersDiff(t *testing.T) {
	m := New("test.go", mockHunks())
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "test.go") {
		t.Error("expected file name in view")
	}
	if !strings.Contains(view, "accept") {
		t.Error("expected 'accept' action in view")
	}
	if !strings.Contains(view, "reject") {
		t.Error("expected 'reject' action in view")
	}
}
