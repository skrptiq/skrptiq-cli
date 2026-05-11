package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/skrptiq/engine/storage"
)

// setupTestDB creates a temp DB with test data and returns the path.
func setupTestDB(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}

	// Insert test nodes.
	slugWf := "blog-post-pipeline"
	slugSk := "language-polish"
	slugPr := "polish-prompt"
	desc := "A test workflow"
	content := "Polish the text: {{input.content}}"
	db.CreateNode("wf1", "workflow", "Blog Post Pipeline", &desc, nil, nil, &slugWf, nil)
	db.CreateNode("sk1", "skill", "Language Polish", nil, nil, nil, &slugSk, nil)
	db.CreateNode("pr1", "prompt", "Polish Prompt", nil, &content, nil, &slugPr, nil)

	db.Close()
	return dbPath
}

// --- List tests ---

func TestList_Nodes(t *testing.T) {
	dbPath := setupTestDB(t)
	code := List([]string{"nodes"}, dbPath)
	if code != ExitOK {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestList_NodesJSON(t *testing.T) {
	dbPath := setupTestDB(t)
	// Redirect stdout to capture output.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := List([]string{"--json", "nodes"}, dbPath)

	w.Close()
	os.Stdout = old

	if code != ExitOK {
		t.Errorf("expected exit 0, got %d", code)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])
	if len(output) == 0 || output[0] != '[' {
		t.Errorf("expected JSON array, got: %s", output)
	}
}

func TestList_NodesQuiet(t *testing.T) {
	dbPath := setupTestDB(t)
	code := List([]string{"nodes", "-q"}, dbPath)
	if code != ExitOK {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestList_Workflows(t *testing.T) {
	dbPath := setupTestDB(t)
	code := List([]string{"workflows"}, dbPath)
	if code != ExitOK {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestList_Profiles(t *testing.T) {
	dbPath := setupTestDB(t)
	code := List([]string{"profiles"}, dbPath)
	if code != ExitOK {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestList_Runs(t *testing.T) {
	dbPath := setupTestDB(t)
	code := List([]string{"runs"}, dbPath)
	if code != ExitOK {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestList_InvalidType(t *testing.T) {
	dbPath := setupTestDB(t)
	code := List([]string{"bananas"}, dbPath)
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

// --- Show tests ---

func TestShow_Node(t *testing.T) {
	dbPath := setupTestDB(t)
	code := Show([]string{"node", "Blog Post Pipeline"}, dbPath)
	if code != ExitOK {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestShow_NodeJSON(t *testing.T) {
	dbPath := setupTestDB(t)
	code := Show([]string{"--json", "node", "Blog Post Pipeline"}, dbPath)
	if code != ExitOK {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestShow_NotFound(t *testing.T) {
	dbPath := setupTestDB(t)
	code := Show([]string{"node", "Nonexistent"}, dbPath)
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

func TestShow_MissingArgs(t *testing.T) {
	code := Show([]string{}, "")
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

// --- Run tests ---

func TestRun_MissingWorkflow(t *testing.T) {
	dbPath := setupTestDB(t)
	code := Run([]string{"Nonexistent Workflow"}, dbPath)
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

func TestRun_MissingArgs(t *testing.T) {
	code := Run([]string{}, "")
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

// --- parseInputs tests ---

func TestParseInputs_Valid(t *testing.T) {
	result, err := parseInputs([]string{"topic=AI trends", "tone=casual"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["topic"] != "AI trends" {
		t.Errorf("expected 'AI trends', got %q", result["topic"])
	}
	if result["tone"] != "casual" {
		t.Errorf("expected 'casual', got %q", result["tone"])
	}
}

func TestParseInputs_InvalidFormat(t *testing.T) {
	_, err := parseInputs([]string{"no-equals-sign"})
	if err == nil {
		t.Error("expected error for missing =")
	}
}

func TestParseInputs_Empty(t *testing.T) {
	result, err := parseInputs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

// --- Hub tests ---

func TestHub_MissingArgs(t *testing.T) {
	code := Hub([]string{}, "")
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

func TestHub_InvalidSubcommand(t *testing.T) {
	dbPath := setupTestDB(t)
	code := Hub([]string{"bananas"}, dbPath)
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

func TestHub_List(t *testing.T) {
	dbPath := setupTestDB(t)
	code := Hub([]string{"list"}, dbPath)
	if code != ExitOK {
		t.Errorf("expected exit 0, got %d", code)
	}
}
