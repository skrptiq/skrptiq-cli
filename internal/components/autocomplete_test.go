package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testCommands() []Command {
	return []Command{
		{Name: "/help", Description: "List all commands"},
		{Name: "/run", Description: "Execute a workflow", ArgProvider: func(partial string) []Completion {
			items := []Completion{
				{Value: "Blog Post Pipeline", Description: "workflow"},
				{Value: "Code Review", Description: "workflow"},
				{Value: "Content Polish", Description: "workflow"},
			}
			if partial == "" {
				return items
			}
			var filtered []Completion
			for _, item := range items {
				if strings.Contains(strings.ToLower(item.Value), strings.ToLower(partial)) {
					filtered = append(filtered, item)
				}
			}
			return filtered
		}},
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
	if len(a.items) != len(testCommands()) {
		t.Errorf("expected all commands with '/' filter, got %d", len(a.items))
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
	if len(a.items) != 2 {
		t.Errorf("expected 2 matches for '/ru' (run, runs), got %d", len(a.items))
	}

	a.SetFilter("/run")
	if len(a.items) != 2 {
		t.Errorf("expected 2 matches for '/run' (run, runs), got %d", len(a.items))
	}

	a.SetFilter("/runs")
	if len(a.items) != 1 {
		t.Errorf("expected 1 match for '/runs', got %d", len(a.items))
	}
}

func TestFilterHub(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	a.Show("/hub")
	if len(a.items) != 2 {
		t.Errorf("expected 2 matches for '/hub', got %d", len(a.items))
	}
}

func TestCursorNavigation(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)
	a.Show("/")

	a, _, _ = a.Update(tea.KeyMsg{Type: tea.KeyDown})
	if a.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", a.cursor)
	}

	a, _, _ = a.Update(tea.KeyMsg{Type: tea.KeyUp})
	if a.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", a.cursor)
	}

	a, _, _ = a.Update(tea.KeyMsg{Type: tea.KeyUp})
	if a.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", a.cursor)
	}
}

func TestSelectCommandWithoutArgs(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)
	a.Show("/h")

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
	if sel.FullText != "/help" {
		t.Errorf("expected '/help', got %q", sel.FullText)
	}
	if !sel.IsCommand {
		t.Error("expected IsCommand to be true")
	}
	if sel.HasArgs {
		t.Error("expected HasArgs to be false for /help")
	}
}

func TestSelectCommandWithArgs(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)
	a.Show("/ru")

	// Move to /run (first match).
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
	if sel.FullText != "/run" {
		t.Errorf("expected '/run', got %q", sel.FullText)
	}
	if !sel.HasArgs {
		t.Error("expected HasArgs to be true for /run")
	}
}

func TestArgCompletion(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	cmd := a.FindCommand("/run")
	if cmd == nil {
		t.Fatal("expected to find /run command")
	}

	a.ShowArgs(cmd, "")
	if !a.Visible() {
		t.Error("should be visible after ShowArgs")
	}
	if len(a.items) != 3 {
		t.Errorf("expected 3 workflow completions, got %d", len(a.items))
	}

	// Filter args.
	a.ShowArgs(cmd, "blog")
	if len(a.items) != 1 {
		t.Errorf("expected 1 match for 'blog', got %d", len(a.items))
	}
	if a.items[0].Value != "Blog Post Pipeline" {
		t.Errorf("expected 'Blog Post Pipeline', got %q", a.items[0].Value)
	}
}

func TestArgSelection(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	cmd := a.FindCommand("/run")
	a.ShowArgs(cmd, "")

	// Select first arg.
	var teaCmd tea.Cmd
	a, teaCmd, _ = a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if teaCmd == nil {
		t.Fatal("expected command from arg select")
	}
	msg := teaCmd()
	sel, ok := msg.(AutocompleteSelectMsg)
	if !ok {
		t.Fatalf("expected AutocompleteSelectMsg, got %T", msg)
	}
	if sel.FullText != "/run Blog Post Pipeline" {
		t.Errorf("expected '/run Blog Post Pipeline', got %q", sel.FullText)
	}
	if sel.IsCommand {
		t.Error("expected IsCommand to be false for arg selection")
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
}

func TestKeysConsumed(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	_, _, consumed := a.Update(tea.KeyMsg{Type: tea.KeyDown})
	if consumed {
		t.Error("keys should not be consumed when hidden")
	}

	a.Show("/")
	_, _, consumed = a.Update(tea.KeyMsg{Type: tea.KeyDown})
	if !consumed {
		t.Error("keys should be consumed when visible")
	}
}

func TestViewRenders(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

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
	if len(a.items) != 0 {
		t.Errorf("expected 0 matches for '/zzz', got %d", len(a.items))
	}
	if a.View() != "" {
		t.Error("expected empty view with no matches")
	}
}

func TestFindCommand(t *testing.T) {
	a := NewAutocomplete(testCommands())

	cmd := a.FindCommand("/run")
	if cmd == nil {
		t.Fatal("expected to find /run")
	}
	if cmd.Name != "/run" {
		t.Errorf("expected '/run', got %q", cmd.Name)
	}

	cmd = a.FindCommand("/nonexistent")
	if cmd != nil {
		t.Error("expected nil for nonexistent command")
	}
}
