package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/skrptiq/skrptiq-cli/internal/views/diff"
	"github.com/skrptiq/skrptiq-cli/internal/views/gate"
	"github.com/skrptiq/skrptiq-cli/internal/views/progress"
	"github.com/skrptiq/skrptiq-cli/internal/views/repl"
	"github.com/skrptiq/skrptiq-cli/internal/views/tree"
)

func setupModel() Model {
	m := New()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return updated.(Model)
}

func TestNewStartsAtREPL(t *testing.T) {
	m := New()
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view, got %d", m.activeView)
	}
}

func TestWindowResize(t *testing.T) {
	m := setupModel()
	if m.width != 80 || m.height != 24 {
		t.Errorf("expected 80x24, got %dx%d", m.width, m.height)
	}
	if !m.ready {
		t.Error("expected ready after resize")
	}
}

func TestHelpCommand(t *testing.T) {
	m := setupModel()
	m, _ = handleCommand(m, "help")
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view after help, got %d", m.activeView)
	}
}

func TestDemoCommand(t *testing.T) {
	m := setupModel()
	var cmd tea.Cmd
	m, cmd = handleCommand(m, "/demo")
	if m.activeView != viewProgress {
		t.Errorf("expected progress view, got %d", m.activeView)
	}
	if cmd == nil {
		t.Error("expected init command for progress view")
	}
}

func TestTreeCommand(t *testing.T) {
	m := setupModel()
	m, _ = handleCommand(m, "/tree")
	if m.activeView != viewTree {
		t.Errorf("expected tree view, got %d", m.activeView)
	}
}

func TestGateCommand(t *testing.T) {
	m := setupModel()
	m, _ = handleCommand(m, "/gate")
	if m.activeView != viewGate {
		t.Errorf("expected gate view, got %d", m.activeView)
	}
}

func TestDiffCommand(t *testing.T) {
	m := setupModel()
	m, _ = handleCommand(m, "/diff")
	if m.activeView != viewDiff {
		t.Errorf("expected diff view, got %d", m.activeView)
	}
}

func TestUnknownCommand(t *testing.T) {
	m := setupModel()
	m, _ = handleCommand(m, "/foobar")
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view after unknown command, got %d", m.activeView)
	}
}

func TestProgressDoneReturnsToREPL(t *testing.T) {
	m := setupModel()
	m.activeView = viewProgress
	updated, _ := m.Update(progress.DoneMsg{Summary: "Done"})
	m = updated.(Model)
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view after progress done, got %d", m.activeView)
	}
}

func TestTreeDismissReturnsToREPL(t *testing.T) {
	m := setupModel()
	m.activeView = viewTree
	updated, _ := m.Update(tree.DismissMsg{})
	m = updated.(Model)
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view after tree dismiss, got %d", m.activeView)
	}
}

func TestGateApproveReturnsToREPL(t *testing.T) {
	m := setupModel()
	m.activeView = viewGate
	updated, _ := m.Update(gate.ResultMsg{Action: gate.ActionApprove, Content: "content"})
	m = updated.(Model)
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view after gate approve, got %d", m.activeView)
	}
}

func TestGateCancelReturnsToREPL(t *testing.T) {
	m := setupModel()
	m.activeView = viewGate
	updated, _ := m.Update(gate.CancelMsg{})
	m = updated.(Model)
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view after gate cancel, got %d", m.activeView)
	}
}

func TestDiffAcceptReturnsToREPL(t *testing.T) {
	m := setupModel()
	m.activeView = viewDiff
	updated, _ := m.Update(diff.ResultMsg{Action: diff.ActionAccept, File: "test.go"})
	m = updated.(Model)
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view after diff accept, got %d", m.activeView)
	}
}

func TestDiffDismissReturnsToREPL(t *testing.T) {
	m := setupModel()
	m.activeView = viewDiff
	updated, _ := m.Update(diff.DismissMsg{})
	m = updated.(Model)
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view after diff dismiss, got %d", m.activeView)
	}
}

func TestSingleCtrlDShowsHint(t *testing.T) {
	m := setupModel()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(Model)
	if !m.exitHint {
		t.Error("expected exitHint to be true after first Ctrl+D")
	}
	if cmd == nil {
		t.Error("expected tick command for clearing hint")
	}
}

func TestDoubleCtrlDExits(t *testing.T) {
	m := setupModel()
	// First press.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(Model)
	// Second press immediately.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if cmd == nil {
		t.Fatal("expected quit command on double Ctrl+D")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

func TestExitHintClears(t *testing.T) {
	m := setupModel()
	// First press sets hint.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(Model)
	if !m.exitHint {
		t.Fatal("expected exitHint after first Ctrl+D")
	}
	// Clear hint message.
	updated, _ = m.Update(clearExitHintMsg{})
	m = updated.(Model)
	if m.exitHint {
		t.Error("expected exitHint to be cleared")
	}
}

func TestREPLSubmitRouting(t *testing.T) {
	m := setupModel()
	updated, _ := m.Update(repl.SubmitMsg{Input: "/tree"})
	m = updated.(Model)
	if m.activeView != viewTree {
		t.Errorf("expected tree view from REPL submit, got %d", m.activeView)
	}
}

func TestBareTextGoesToChat(t *testing.T) {
	m := setupModel()
	m, _ = handleCommand(m, "polish this README for me")
	// Should stay on REPL (chat mode, not switch views).
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view for chat input, got %d", m.activeView)
	}
	// Should have output (chat placeholder message).
	if len(m.repl.History()) == 0 {
		t.Error("expected chat response output")
	}
}

func TestSlashPrefixCommands(t *testing.T) {
	m := setupModel()
	m, _ = handleCommand(m, "/help")
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view after /help, got %d", m.activeView)
	}

	m = setupModel()
	m, _ = handleCommand(m, "/demo")
	if m.activeView != viewProgress {
		t.Errorf("expected progress view after /demo, got %d", m.activeView)
	}

	m = setupModel()
	m, _ = handleCommand(m, "/tree")
	if m.activeView != viewTree {
		t.Errorf("expected tree view after /tree, got %d", m.activeView)
	}

	m = setupModel()
	m, _ = handleCommand(m, "/gate")
	if m.activeView != viewGate {
		t.Errorf("expected gate view after /gate, got %d", m.activeView)
	}

	m = setupModel()
	m, _ = handleCommand(m, "/diff")
	if m.activeView != viewDiff {
		t.Errorf("expected diff view after /diff, got %d", m.activeView)
	}
}

func TestImplementedCommandShowsOutput(t *testing.T) {
	m := setupModel()
	m, _ = handleCommand(m, "/runs")
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view after /runs, got %d", m.activeView)
	}
	if len(m.repl.History()) == 0 {
		t.Error("expected output from /runs")
	}
}

func TestDeferredCommandShowsMessage(t *testing.T) {
	m := setupModel()
	m, _ = handleCommand(m, "/resume")
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view after deferred command, got %d", m.activeView)
	}
	if len(m.repl.History()) == 0 {
		t.Error("expected output message for deferred command")
	}
}

func TestEscapeCancelsOverlay(t *testing.T) {
	m := setupModel()
	// Enter tree view.
	m, _ = handleCommand(m, "/tree")
	if m.activeView != viewTree {
		t.Fatalf("expected tree view, got %d", m.activeView)
	}
	// Press escape.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view after escape, got %d", m.activeView)
	}
}

func TestEscapeDoesNothingInREPL(t *testing.T) {
	m := setupModel()
	// Escape in REPL should not quit or change view.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.activeView != viewREPL {
		t.Errorf("expected REPL view, got %d", m.activeView)
	}
}
