package progress

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
)

func TestNew(t *testing.T) {
	steps := []string{"Step 1", "Step 2", "Step 3"}
	m := New(steps)

	if len(m.steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(m.steps))
	}
	if m.done {
		t.Error("expected not done initially")
	}
	if m.current != -1 {
		t.Errorf("expected current -1, got %d", m.current)
	}
}

func TestTickAdvancesSteps(t *testing.T) {
	m := New([]string{"Step 1", "Step 2"})
	m.SetSize(80, 24)

	// First tick: starts step 0.
	m, _ = m.Update(TickMsg{})
	if m.current != 0 {
		t.Errorf("expected current 0, got %d", m.current)
	}
	if m.steps[0].Status != StepRunning {
		t.Errorf("expected step 0 running, got %d", m.steps[0].Status)
	}

	// Second tick: completes step 0, starts step 1.
	m, _ = m.Update(TickMsg{})
	if m.steps[0].Status != StepDone {
		t.Errorf("expected step 0 done, got %d", m.steps[0].Status)
	}
	if m.steps[1].Status != StepRunning {
		t.Errorf("expected step 1 running, got %d", m.steps[1].Status)
	}

	// Third tick: completes step 1, all done.
	m, cmd := m.Update(TickMsg{})
	if !m.done {
		t.Error("expected done after all steps complete")
	}
	if cmd == nil {
		t.Fatal("expected DoneMsg command")
	}
	msg := cmd()
	if _, ok := msg.(DoneMsg); !ok {
		t.Errorf("expected DoneMsg, got %T", msg)
	}
}

func TestViewRendersSteps(t *testing.T) {
	m := New([]string{"Drafting Agent", "Review Agent"})
	m.SetSize(80, 24)

	// Advance to first step.
	m, _ = m.Update(TickMsg{})

	view := m.View()
	if !strings.Contains(view, "Drafting Agent") {
		t.Error("expected 'Drafting Agent' in view")
	}
	if !strings.Contains(view, "Review Agent") {
		t.Error("expected 'Review Agent' in view")
	}
}

func TestSpinnerUpdates(t *testing.T) {
	m := New([]string{"Step 1"})
	m.SetSize(80, 24)

	// Spinner tick should not error.
	m, _ = m.Update(spinner.TickMsg{})
}

func TestDoneAfterAllSteps(t *testing.T) {
	m := New([]string{"Only step"})
	m.SetSize(80, 24)

	if m.Done() {
		t.Error("should not be done initially")
	}

	m, _ = m.Update(TickMsg{}) // start step 0
	m, _ = m.Update(TickMsg{}) // complete step 0

	if !m.Done() {
		t.Error("should be done after all steps")
	}
	if m.Summary() == "" {
		t.Error("expected non-empty summary")
	}
}
