package repl

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNew(t *testing.T) {
	m := New()
	if len(m.history) != 0 {
		t.Errorf("expected empty history, got %d entries", len(m.history))
	}
}

func TestSubmit(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	// Type "hello" into the input.
	for _, r := range "hello" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press enter.
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected a command after submit")
	}

	msg := cmd()
	submit, ok := msg.(SubmitMsg)
	if !ok {
		t.Fatalf("expected SubmitMsg, got %T", msg)
	}
	if submit.Input != "hello" {
		t.Errorf("expected input 'hello', got %q", submit.Input)
	}

	// Input should be cleared after submit.
	if m.input.Value() != "" {
		t.Errorf("expected empty input after submit, got %q", m.input.Value())
	}

	// History should contain the command.
	if len(m.history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(m.history))
	}
}

func TestAddOutput(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	m.AddOutput("test output")

	if len(m.history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(m.history))
	}
	if m.history[0] != "test output" {
		t.Errorf("expected 'test output', got %q", m.history[0])
	}
}

func TestViewShowsWelcome(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "Welcome") {
		t.Error("expected welcome message in empty REPL view")
	}
}

func TestEmptySubmitIgnored(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	// Press enter with no input.
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		// The text input update may return a blink command — check it doesn't produce SubmitMsg.
		msg := cmd()
		if _, ok := msg.(SubmitMsg); ok {
			t.Error("expected no SubmitMsg for empty input")
		}
	}

	if len(m.history) != 0 {
		t.Errorf("expected no history entries for empty submit, got %d", len(m.history))
	}
}
