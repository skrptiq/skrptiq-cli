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
		{Name: "/runs", Description: "Execution history", Subcommands: []Subcommand{
			{Name: "list", Description: "List recent executions"},
		}},
		{Name: "/hub", Description: "Hub operations", Subcommands: []Subcommand{
			{Name: "list", Description: "List imported skrpts"},
			{Name: "search", Description: "Search community skrpts"},
		}},
		{Name: "/profile", Description: "Voice profiles", Subcommands: []Subcommand{
			{Name: "list", Description: "List profiles"},
			{Name: "use", Description: "Switch profile", ArgProvider: func(partial string) []Completion {
				return []Completion{
					{Value: "Ben's Voice", Description: "voice (active)"},
					{Value: "Formal", Description: "voice"},
				}
			}},
		}},
	}
}

func TestNewAutocomplete(t *testing.T) {
	a := NewAutocomplete(testCommands())
	if a.Visible() {
		t.Error("should not be visible initially")
	}
}

func TestShowOnlyTopLevel(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	a.Show("/")
	if !a.Visible() {
		t.Error("should be visible after Show")
	}
	// Should only show top-level commands, not subcommands.
	if len(a.items) != 5 {
		t.Errorf("expected 5 top-level commands, got %d", len(a.items))
		for _, item := range a.items {
			t.Logf("  %s", item.Value)
		}
	}
	// Should NOT contain "list" or "search" as standalone items.
	for _, item := range a.items {
		if item.Value == "list" || item.Value == "search" {
			t.Errorf("subcommand %q should not appear at top level", item.Value)
		}
	}
}

func TestFilterTopLevel(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	a.Show("/hu")
	if len(a.items) != 1 {
		t.Errorf("expected 1 match for '/hu', got %d", len(a.items))
	}
	if len(a.items) > 0 && a.items[0].Value != "/hub" {
		t.Errorf("expected '/hub', got %q", a.items[0].Value)
	}
}

func TestSubcommandCompletion(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	cmd := a.FindCommand("/hub")
	if cmd == nil {
		t.Fatal("expected to find /hub")
	}

	a.ShowSubcommands(cmd, "")
	if !a.Visible() {
		t.Error("should be visible after ShowSubcommands")
	}
	if len(a.items) != 2 {
		t.Errorf("expected 2 subcommands for /hub, got %d", len(a.items))
	}

	// Filter subcommands.
	a.ShowSubcommands(cmd, "l")
	if len(a.items) != 1 {
		t.Errorf("expected 1 match for 'l', got %d", len(a.items))
	}
	if len(a.items) > 0 && a.items[0].Value != "list" {
		t.Errorf("expected 'list', got %q", a.items[0].Value)
	}
}

func TestSelectTopLevelWithSubs(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)
	a.Show("/hu")

	var cmd tea.Cmd
	a, cmd, _ = a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd == nil {
		t.Fatal("expected command from select")
	}
	msg := cmd()
	sel, ok := msg.(AutocompleteSelectMsg)
	if !ok {
		t.Fatalf("expected AutocompleteSelectMsg, got %T", msg)
	}
	if sel.FullText != "/hub" {
		t.Errorf("expected '/hub', got %q", sel.FullText)
	}
	if !sel.NeedsMore {
		t.Error("expected NeedsMore for command with subcommands")
	}
}

func TestSelectTopLevelWithArgs(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)
	a.Show("/ru")

	// First item should be /run.
	var cmd tea.Cmd
	a, cmd, _ = a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd == nil {
		t.Fatal("expected command from select")
	}
	msg := cmd()
	sel := msg.(AutocompleteSelectMsg)
	if sel.FullText != "/run" {
		t.Errorf("expected '/run', got %q", sel.FullText)
	}
	if !sel.NeedsMore {
		t.Error("expected NeedsMore for command with ArgProvider")
	}
}

func TestSelectTopLevelNoMore(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)
	a.Show("/he")

	var cmd tea.Cmd
	a, cmd, _ = a.Update(tea.KeyMsg{Type: tea.KeyTab})
	msg := cmd()
	sel := msg.(AutocompleteSelectMsg)
	if sel.NeedsMore {
		t.Error("expected no NeedsMore for /help")
	}
}

func TestSelectSubcommand(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	cmd := a.FindCommand("/hub")
	a.ShowSubcommands(cmd, "")

	// Select "list".
	var teaCmd tea.Cmd
	a, teaCmd, _ = a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if teaCmd == nil {
		t.Fatal("expected command from subcommand select")
	}
	msg := teaCmd()
	sel := msg.(AutocompleteSelectMsg)
	if sel.FullText != "/hub list" {
		t.Errorf("expected '/hub list', got %q", sel.FullText)
	}
	if sel.NeedsMore {
		t.Error("expected no NeedsMore for /hub list (no ArgProvider)")
	}
}

func TestSelectSubcommandWithArgs(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	cmd := a.FindCommand("/profile")
	a.ShowSubcommands(cmd, "")

	// Move to "use" (index 1).
	a, _, _ = a.Update(tea.KeyMsg{Type: tea.KeyDown})

	var teaCmd tea.Cmd
	a, teaCmd, _ = a.Update(tea.KeyMsg{Type: tea.KeyTab})
	msg := teaCmd()
	sel := msg.(AutocompleteSelectMsg)
	if sel.FullText != "/profile use" {
		t.Errorf("expected '/profile use', got %q", sel.FullText)
	}
	if !sel.NeedsMore {
		t.Error("expected NeedsMore for /profile use (has ArgProvider)")
	}
}

func TestArgCompletion(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	cmd := a.FindCommand("/run")
	a.ShowArgs(cmd, nil, "")
	if len(a.items) != 3 {
		t.Errorf("expected 3 workflow completions, got %d", len(a.items))
	}

	a.ShowArgs(cmd, nil, "blog")
	if len(a.items) != 1 {
		t.Errorf("expected 1 match for 'blog', got %d", len(a.items))
	}
}

func TestArgViaSubcommand(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	cmd := a.FindCommand("/profile")
	sub := a.FindSubcommand(cmd, "use")
	if sub == nil {
		t.Fatal("expected to find 'use' subcommand")
	}

	a.ShowArgs(cmd, sub, "")
	if len(a.items) != 2 {
		t.Errorf("expected 2 profile completions, got %d", len(a.items))
	}
}

func TestArgSelection(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	cmd := a.FindCommand("/run")
	a.ShowArgs(cmd, nil, "")

	var teaCmd tea.Cmd
	a, teaCmd, _ = a.Update(tea.KeyMsg{Type: tea.KeyTab})
	msg := teaCmd()
	sel := msg.(AutocompleteSelectMsg)
	if sel.FullText != "/run Blog Post Pipeline" {
		t.Errorf("expected '/run Blog Post Pipeline', got %q", sel.FullText)
	}
	if sel.NeedsMore {
		t.Error("expected no NeedsMore for arg selection")
	}
}

func TestDismiss(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)
	a.Show("/")

	var cmd tea.Cmd
	a, cmd, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
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
	if !strings.Contains(view, "/hub") {
		t.Error("expected '/hub' in view")
	}
	// Should NOT contain subcommands.
	if strings.Contains(view, "list") {
		t.Error("should not show subcommand 'list' at top level")
	}
}

func TestEmptyFilter(t *testing.T) {
	a := NewAutocomplete(testCommands())
	a.SetWidth(80)

	a.Show("/zzz")
	if len(a.items) != 0 {
		t.Errorf("expected 0 matches for '/zzz', got %d", len(a.items))
	}
}

func TestFindCommand(t *testing.T) {
	a := NewAutocomplete(testCommands())

	cmd := a.FindCommand("/hub")
	if cmd == nil || cmd.Name != "/hub" {
		t.Error("expected to find /hub")
	}

	if a.FindCommand("/nonexistent") != nil {
		t.Error("expected nil for nonexistent command")
	}
}

func TestFindSubcommand(t *testing.T) {
	a := NewAutocomplete(testCommands())

	cmd := a.FindCommand("/hub")
	sub := a.FindSubcommand(cmd, "list")
	if sub == nil || sub.Name != "list" {
		t.Error("expected to find 'list' subcommand")
	}

	if a.FindSubcommand(cmd, "nonexistent") != nil {
		t.Error("expected nil for nonexistent subcommand")
	}
}
