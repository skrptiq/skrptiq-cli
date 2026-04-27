package app

import (
	"strings"
	"testing"
)

// --- Type tests ---

func TestCommandHasSubcommands(t *testing.T) {
	cmd := Command{Name: "/hub", Subcommands: []Subcommand{{Name: "list"}}}
	if !cmd.HasSubcommands() {
		t.Error("expected HasSubcommands to be true")
	}

	cmd2 := Command{Name: "/help"}
	if cmd2.HasSubcommands() {
		t.Error("expected HasSubcommands to be false")
	}
}

func TestAppModeLabel(t *testing.T) {
	tests := []struct {
		mode     AppMode
		expected string
	}{
		{ModeCommand, "command"},
		{ModeChat, "chat"},
		{ModeRun, "run"},
	}
	for _, tt := range tests {
		if tt.mode.Label() != tt.expected {
			t.Errorf("mode %d: expected %q, got %q", tt.mode, tt.expected, tt.mode.Label())
		}
	}
}

func TestAppModeSymbol(t *testing.T) {
	if ModeCommand.Symbol() != "⚡" {
		t.Errorf("expected ⚡ for command mode, got %q", ModeCommand.Symbol())
	}
	if ModeChat.Symbol() != "💬" {
		t.Errorf("expected 💬 for chat mode, got %q", ModeChat.Symbol())
	}
	if ModeRun.Symbol() != "▶" {
		t.Errorf("expected ▶ for run mode, got %q", ModeRun.Symbol())
	}
}

// --- splitFirst tests ---

func TestSplitFirst(t *testing.T) {
	tests := []struct {
		input       string
		expectedCmd string
		expectedArg string
	}{
		{"", "", ""},
		{"list", "list", ""},
		{"list workflows", "list", "workflows"},
		{"show My Node Name", "show", "My Node Name"},
		{"  set  key value  ", "set", "key value"},
	}
	for _, tt := range tests {
		cmd, arg := splitFirst(tt.input)
		if cmd != tt.expectedCmd || arg != tt.expectedArg {
			t.Errorf("splitFirst(%q) = (%q, %q), want (%q, %q)",
				tt.input, cmd, arg, tt.expectedCmd, tt.expectedArg)
		}
	}
}

// --- helpText tests ---

func TestHelpTextContainsAllCommands(t *testing.T) {
	help := helpText()

	required := []string{
		"/chat", "/run", "/exit", "/command",
		"/list", "/search", "/show",
		"/hub", "/runs", "/profile",
		"/mcp", "/workspace", "/tags",
		"/settings",
		"/clear", "/help",
	}

	for _, cmd := range required {
		if !strings.Contains(help, cmd) {
			t.Errorf("help text missing command: %s", cmd)
		}
	}
}

// --- Command routing tests ---

// testModel creates a minimal Model for testing command handlers.
// Engine is nil — handlers should handle that gracefully.
func testModel() *Model {
	m := &Model{
		mode:     ModeCommand,
		commands: BuildCommands(nil),
	}
	return m
}

func TestHandleSlashCommandRouting(t *testing.T) {
	m := testModel()

	// Commands that should be handled.
	handled := []string{
		"help", "chat", "command", "exit", "run",
		"clear", "list", "show", "search",
		"hub", "runs", "profile", "mcp", "providers",
		"workspace", "tags", "tag", "untag",
		"config", "settings",
	}

	for _, cmd := range handled {
		if !m.handleSlashCommand(cmd, "") {
			t.Errorf("expected %q to be handled", cmd)
		}
	}

	// Commands that should NOT be handled (unknown).
	unhandled := []string{
		"foobar", "invalid", "xyz",
	}

	for _, cmd := range unhandled {
		if m.handleSlashCommand(cmd, "") {
			t.Errorf("expected %q to NOT be handled", cmd)
		}
	}
}

func TestHandleSlashCommandModeSwitch(t *testing.T) {
	m := testModel()

	// Enter chat mode.
	m.handleSlashCommand("chat", "")
	if m.mode != ModeChat {
		t.Errorf("expected ModeChat, got %d", m.mode)
	}

	// Enter run mode.
	m.handleSlashCommand("run", "")
	if m.mode != ModeRun {
		t.Errorf("expected ModeRun, got %d", m.mode)
	}

	// Exit to command mode.
	m.handleSlashCommand("exit", "")
	if m.mode != ModeCommand {
		t.Errorf("expected ModeCommand, got %d", m.mode)
	}

	// /command also returns to command mode.
	m.handleSlashCommand("chat", "")
	m.handleSlashCommand("command", "")
	if m.mode != ModeCommand {
		t.Errorf("expected ModeCommand from /command, got %d", m.mode)
	}
}

func TestHandleSlashCommandRunWithWorkflow(t *testing.T) {
	m := testModel()

	m.handleSlashCommand("run", "Blog Post Pipeline")
	if m.mode != ModeRun {
		t.Errorf("expected ModeRun, got %d", m.mode)
	}
	if m.runWorkflow != "Blog Post Pipeline" {
		t.Errorf("expected runWorkflow to be set, got %q", m.runWorkflow)
	}
}

// --- Handler nil engine tests ---
// Verify handlers don't panic when engine is nil.

func TestHandlersWithNilEngine(t *testing.T) {
	m := testModel()

	// These should all print an error message but not panic.
	m.handleSlashCommand("list", "")
	m.handleSlashCommand("show", "test")
	m.handleSlashCommand("search", "test")
	m.handleSlashCommand("hub", "list")
	m.handleSlashCommand("runs", "list")
	m.handleSlashCommand("profile", "list")
	m.handleSlashCommand("mcp", "list")
	m.handleSlashCommand("providers", "list")
	m.handleSlashCommand("tags", "list")
	m.handleSlashCommand("tag", "node tag")
	m.handleSlashCommand("untag", "node tag")
	m.handleSlashCommand("config", "show")
	m.handleSlashCommand("settings", "connections")
}

// --- Usage block tests ---

func TestUsageBlockContainsSubcommands(t *testing.T) {
	block := usageBlock("/hub", []string{"list", "search", "import"})

	if !strings.Contains(block, "/hub") {
		t.Error("usage block missing command name")
	}
	if !strings.Contains(block, "list") {
		t.Error("usage block missing subcommand 'list'")
	}
	if !strings.Contains(block, "search") {
		t.Error("usage block missing subcommand 'search'")
	}
}

// --- Status icon tests ---

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status string
		expect string // just check it's non-empty
	}{
		{"completed", "✓"},
		{"failed", "✗"},
		{"running", "◌"},
		{"paused", "⏸"},
		{"unknown", "○"},
	}

	for _, tt := range tests {
		icon := statusIcon(tt.status)
		if !strings.Contains(icon, tt.expect) {
			t.Errorf("statusIcon(%q) = %q, expected to contain %q", tt.status, icon, tt.expect)
		}
	}
}

// --- BuildCommands tests ---

func TestBuildCommandsWithNilEngine(t *testing.T) {
	commands := BuildCommands(nil)
	if len(commands) == 0 {
		t.Fatal("expected commands even with nil engine")
	}

	// Check key commands exist.
	names := make(map[string]bool)
	for _, c := range commands {
		names[c.Name] = true
	}

	required := []string{"/chat", "/run", "/command", "/exit", "/help", "/clear",
		"/list", "/search", "/show", "/hub", "/profile", "/mcp",
		"/workspace", "/tags", "/tag", "/untag", "/settings",
		"/runs", "/resume", "/stop"}

	for _, name := range required {
		if !names[name] {
			t.Errorf("missing command: %s", name)
		}
	}
}

func TestBuildCommandsSubcommands(t *testing.T) {
	commands := BuildCommands(nil)

	// Find /hub and check it has subcommands.
	for _, c := range commands {
		if c.Name == "/hub" {
			if !c.HasSubcommands() {
				t.Error("/hub should have subcommands")
			}
			subNames := make(map[string]bool)
			for _, s := range c.Subcommands {
				subNames[s.Name] = true
			}
			for _, expected := range []string{"list", "search", "import", "update"} {
				if !subNames[expected] {
					t.Errorf("/hub missing subcommand: %s", expected)
				}
			}
			return
		}
	}
	t.Error("/hub command not found")
}

func TestBuildCommandsRunHasArgProvider(t *testing.T) {
	commands := BuildCommands(nil)
	for _, c := range commands {
		if c.Name == "/run" {
			if c.ArgProvider == nil {
				t.Error("/run should have ArgProvider")
			}
			// With nil engine, ArgProvider should return nil.
			result := c.ArgProvider("")
			if result != nil {
				t.Errorf("expected nil from ArgProvider with nil engine, got %d results", len(result))
			}
			return
		}
	}
	t.Error("/run command not found")
}

// --- handleInput bare text tests ---

func TestHandleInputBareTextInCommandMode(t *testing.T) {
	m := testModel()
	m.mode = ModeCommand
	// Bare text in command mode should not panic.
	m.handleInput("some random text")
}

func TestHandleInputSlashInAnyMode(t *testing.T) {
	m := testModel()

	// Slash commands work in chat mode.
	m.mode = ModeChat
	m.handleInput("/help")
	// Should still be handled (help prints, no crash).

	// Slash commands work in run mode.
	m.mode = ModeRun
	m.handleInput("/exit")
	if m.mode != ModeCommand {
		t.Errorf("expected ModeCommand after /exit in run mode, got %d", m.mode)
	}
}

// --- noEngineMsg tests ---

func TestNoEngineMsg(t *testing.T) {
	msg := noEngineMsg()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

