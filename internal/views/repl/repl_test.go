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

func TestCommandHistory(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	// Submit two commands.
	for _, r := range "first" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	for _, r := range "second" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if len(m.cmdHistory) != 2 {
		t.Fatalf("expected 2 command history entries, got %d", len(m.cmdHistory))
	}

	// Press up — should show "second".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.input.Value() != "second" {
		t.Errorf("expected 'second', got %q", m.input.Value())
	}

	// Press up again — should show "first".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.input.Value() != "first" {
		t.Errorf("expected 'first', got %q", m.input.Value())
	}

	// Press down — back to "second".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.input.Value() != "second" {
		t.Errorf("expected 'second', got %q", m.input.Value())
	}

	// Press down again — back to empty (current input).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.input.Value() != "" {
		t.Errorf("expected empty input, got %q", m.input.Value())
	}
}

func TestCommandHistoryPreservesInput(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	// Submit a command.
	for _, r := range "old" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Type partial input.
	for _, r := range "partial" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press up — should save "partial" and show "old".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.input.Value() != "old" {
		t.Errorf("expected 'old', got %q", m.input.Value())
	}

	// Press down — should restore "partial".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.input.Value() != "partial" {
		t.Errorf("expected 'partial', got %q", m.input.Value())
	}
}

func TestActivityIndicator(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	m.SetActivity("Processing workflow")
	if m.Activity() != "Processing workflow" {
		t.Errorf("expected activity text, got %q", m.Activity())
	}

	view := m.View()
	if !strings.Contains(view, "Processing workflow") {
		t.Error("expected activity text in view")
	}

	m.SetActivity("")
	view = m.View()
	if strings.Contains(view, "Processing") {
		t.Error("expected no activity text after clearing")
	}
}
