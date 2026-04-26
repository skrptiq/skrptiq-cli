package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testCommands() []Command {
	return []Command{
		{Name: "/help", Description: "List all commands"},
		{Name: "/run", Description: "Execute a workflow"},
		{Name: "/runs", Description: "List recent executions"},
		{Name: "/resume", Description: "Resume a paused execution"},
		{Name: "/hub", Description: "Hub status"},
		{Name: "/hub search", Description: "Search skrpts"},
	}
}

func TestNewAutocomplete(t *testing.T) {
	a := NewAutocomplete(testCommands())
	if a.Visible() {
		t.Error("should not be visible initially")
	}
}

func TestShowAndHide(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	a.Show("/")
	if !a.Visible() {
		t.Error("should be visible after Show")
	}
	if len(a.filtered) != len(testCommands()) {
		t.Errorf("expected all commands with '/' filter, got %d", len(a.filtered))
	}

	a.Hide()
	if a.Visible() {
		t.Error("should not be visible after Hide")
	}
}

func TestFilterByPrefix(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	a.Show("/ru")
	if len(a.filtered) != 2 {
		t.Errorf("expected 2 matches for '/ru' (run, runs), got %d", len(a.filtered))
	}

	a.SetFilter("/run")
	if len(a.filtered) != 2 {
		t.Errorf("expected 2 matches for '/run' (run, runs), got %d", len(a.filtered))
	}

	a.SetFilter("/runs")
	if len(a.filtered) != 1 {
		t.Errorf("expected 1 match for '/runs', got %d", len(a.filtered))
	}
}

func TestFilterHub(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	a.Show("/hub")
	if len(a.filtered) != 2 {
		t.Errorf("expected 2 matches for '/hub', got %d", len(a.filtered))
	}
}

func TestCursorNavigation(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)
	a.Show("/")

	// Move down.
	a, _, _ = a.Update(tea.KeyMsg{Type: tea.KeyDown})
	if a.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", a.cursor)
	}

	// Move up.
	a, _, _ = a.Update(tea.KeyMsg{Type: tea.KeyUp})
	if a.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", a.cursor)
	}

	// Can't go above 0.
	a, _, _ = a.Update(tea.KeyMsg{Type: tea.KeyUp})
	if a.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", a.cursor)
	}
}

func TestSelectCommand(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)
	a.Show("/")

	// Select first item.
	var cmd tea.Cmd
	a, cmd, _ = a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from select")
	}
	msg := cmd()
	sel, ok := msg.(AutocompleteSelectMsg)
	if !ok {
		t.Fatalf("expected AutocompleteSelectMsg, got %T", msg)
	}
	if sel.Command != "/help" {
		t.Errorf("expected '/help', got %q", sel.Command)
	}
	if a.Visible() {
		t.Error("should hide after selection")
	}
}

func TestDismiss(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)
	a.Show("/")

	var cmd tea.Cmd
	a, cmd, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected command from dismiss")
	}
	msg := cmd()
	if _, ok := msg.(AutocompleteDismissMsg); !ok {
		t.Errorf("expected AutocompleteDismissMsg, got %T", msg)
	}
	if a.Visible() {
		t.Error("should hide after dismiss")
	}
}

func TestKeysConsumed(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	// Keys should NOT be consumed when not visible.
	_, _, consumed := a.Update(tea.KeyMsg{Type: tea.KeyDown})
	if consumed {
		t.Error("keys should not be consumed when autocomplete is hidden")
	}

	// Keys SHOULD be consumed when visible.
	a.Show("/")
	_, _, consumed = a.Update(tea.KeyMsg{Type: tea.KeyDown})
	if !consumed {
		t.Error("keys should be consumed when autocomplete is visible")
	}
}

func TestViewRenders(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	// No output when hidden.
	if a.View() != "" {
		t.Error("expected empty view when hidden")
	}

	a.Show("/")
	view := a.View()
	if !strings.Contains(view, "/help") {
		t.Error("expected '/help' in view")
	}
	if !strings.Contains(view, "/run") {
		t.Error("expected '/run' in view")
	}
}

func TestEmptyFilter(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	a.Show("/zzz")
	if len(a.filtered) != 0 {
		t.Errorf("expected 0 matches for '/zzz', got %d", len(a.filtered))
	}
	// View should be empty when no matches.
	if a.View() != "" {
		t.Error("expected empty view with no matches")
	}
}

func TestTabSelect(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)
	a.Show("/h")

	// Tab should also select.
	var cmd tea.Cmd
	a, cmd, _ = a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd == nil {
		t.Fatal("expected command from tab select")
	}
	msg := cmd()
	sel, ok := msg.(AutocompleteSelectMsg)
	if !ok {
		t.Fatalf("expected AutocompleteSelectMsg, got %T", msg)
	}
	if sel.Command != "/help" {
		t.Errorf("expected '/help', got %q", sel.Command)
	}
}
