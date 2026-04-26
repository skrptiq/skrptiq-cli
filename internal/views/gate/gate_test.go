package gate

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNew(t *testing.T) {
	m := New("Review draft", "Draft content here")
	m.SetSize(80, 24)

	if m.title != "Review draft" {
		t.Errorf("expected title 'Review draft', got %q", m.title)
	}
	if m.content != "Draft content here" {
		t.Errorf("expected content 'Draft content here', got %q", m.content)
	}
}

func TestApprove(t *testing.T) {
	m := New("Review draft", "Content")
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected command from approve")
	}
	msg := cmd()
	result, ok := msg.(ResultMsg)
	if !ok {
		t.Fatalf("expected ResultMsg, got %T", msg)
	}
	if result.Action != ActionApprove {
		t.Errorf("expected ActionApprove, got %d", result.Action)
	}
}

func TestCancel(t *testing.T) {
	m := New("Review draft", "Content")
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd == nil {
		t.Fatal("expected command from cancel")
	}
	msg := cmd()
	if _, ok := msg.(CancelMsg); !ok {
		t.Errorf("expected CancelMsg, got %T", msg)
	}
}

func TestToggleView(t *testing.T) {
	m := New("Review draft", "Content")
	m.SetSize(80, 24)

	if m.viewing {
		t.Error("should not be in full view initially")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if !m.viewing {
		t.Error("should be in full view after pressing v")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if m.viewing {
		t.Error("should exit full view after pressing v again")
	}
}

func TestViewRendersActions(t *testing.T) {
	m := New("Review draft", "Content")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "approve") {
		t.Error("expected 'approve' action in view")
	}
	if !strings.Contains(view, "cancel") {
		t.Error("expected 'cancel' action in view")
	}
	if !strings.Contains(view, "Gate") {
		t.Error("expected 'Gate' title in view")
	}
}
