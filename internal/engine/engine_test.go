package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultDBPath(t *testing.T) {
	path := DefaultDBPath()
	if path == "" {
		t.Fatal("DefaultDBPath returned empty string")
	}

	// Should be an absolute path.
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %q", path)
	}

	// Should end with skrptiq.db.
	if !strings.HasSuffix(path, "skrptiq.db") {
		t.Errorf("expected path ending in skrptiq.db, got %q", path)
	}
}

func TestDefaultDBPathChecksAppSupportFirst(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	appSupportDB := filepath.Join(home, "Library", "Application Support", "skrptiq", "data", "skrptiq.db")
	if _, err := os.Stat(appSupportDB); err == nil {
		// App support DB exists — DefaultDBPath should return it.
		path := DefaultDBPath()
		if path != appSupportDB {
			t.Errorf("expected %q, got %q", appSupportDB, path)
		}
	}
}

func TestOpenInvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/path/to/db.sqlite")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestOpenDirCreateErrorIsActionable(t *testing.T) {
	// A path where the parent directory cannot be created should give
	// actionable guidance including the directory and a mkdir suggestion.
	_, err := Open("/nonexistent/path/to/db.sqlite")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
	msg := err.Error()
	if !strings.Contains(msg, "cannot create data directory") {
		t.Errorf("error should describe the failure, got: %s", msg)
	}
	if !strings.Contains(msg, "mkdir -p") {
		t.Errorf("error should suggest mkdir, got: %s", msg)
	}
}

func TestOpenDBErrorIsActionable(t *testing.T) {
	// A path where the directory exists but the file is not a valid DB
	// should give guidance about common causes.
	tmp := t.TempDir()
	badDB := filepath.Join(tmp, "bad.db")
	// Write garbage to make it an invalid SQLite file.
	os.WriteFile(badDB, []byte("not a database"), 0644)
	_, err := Open(badDB)
	if err == nil {
		// Some SQLite drivers accept any file; skip if no error.
		t.Skip("storage.Open accepted invalid file")
	}
	msg := err.Error()
	if !strings.Contains(msg, "--db-path") {
		t.Errorf("error should suggest --db-path flag, got: %s", msg)
	}
	if !strings.Contains(msg, "Common causes") {
		t.Errorf("error should list common causes, got: %s", msg)
	}
}

func TestOpenTempDB(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	app, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	if app.DB == nil {
		t.Fatal("DB is nil")
	}
	if app.Hub == nil {
		t.Fatal("Hub client is nil")
	}
}

func TestNodesByTypeEmpty(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	nodes, err := app.NodesByType("workflow")
	if err != nil {
		t.Fatalf("NodesByType failed: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestWorkflowsEmpty(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	wf, err := app.Workflows()
	if err != nil {
		t.Fatalf("Workflows failed: %v", err)
	}
	if len(wf) != 0 {
		t.Errorf("expected 0 workflows, got %d", len(wf))
	}
}

func TestProfilesEmpty(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	profiles, err := app.Profiles()
	if err != nil {
		t.Fatalf("Profiles failed: %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(profiles))
	}
}

func TestActiveProfileNone(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	p, err := app.ActiveProfile("voice")
	if err != nil {
		t.Fatalf("ActiveProfile failed: %v", err)
	}
	if p != nil {
		t.Error("expected nil profile when none configured")
	}
}

func TestTagsEmpty(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	tags, err := app.Tags()
	if err != nil {
		t.Fatalf("Tags failed: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(tags))
	}
}

func TestSettingDefault(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	val := app.Setting("nonexistent")
	if val != "" {
		t.Errorf("expected empty string for nonexistent setting, got %q", val)
	}
}

func TestFindNodeByTitleNotFound(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	node, err := app.FindNodeByTitle("Nonexistent")
	if err != nil {
		t.Fatalf("FindNodeByTitle failed: %v", err)
	}
	if node != nil {
		t.Error("expected nil for nonexistent node")
	}
}

func TestSearchNodesEmpty(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	results, err := app.SearchNodes("test")
	if err != nil {
		t.Fatalf("SearchNodes failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFindProfileByNameNotFound(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	p, err := app.FindProfileByName("Nonexistent")
	if err != nil {
		t.Fatalf("FindProfileByName failed: %v", err)
	}
	if p != nil {
		t.Error("expected nil for nonexistent profile")
	}
}

func TestListExecutionsEmpty(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	runs, err := app.ListExecutions(10)
	if err != nil {
		t.Fatalf("ListExecutions failed: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(runs))
	}
}

func TestFindRunByPrefixNotFound(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	id, err := app.FindRunByPrefix("nonexist")
	if err != nil {
		t.Fatalf("FindRunByPrefix failed: %v", err)
	}
	if id != nil {
		t.Error("expected nil for nonexistent run")
	}
}

func TestHubImportsEmpty(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	imports, err := app.HubImports()
	if err != nil {
		t.Fatalf("HubImports failed: %v", err)
	}
	if len(imports) != 0 {
		t.Errorf("expected 0 imports, got %d", len(imports))
	}
}

func TestMCPServersEmpty(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	servers, err := app.MCPServers()
	if err != nil {
		t.Fatalf("MCPServers failed: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
}

func TestProvidersEmpty(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	providers, err := app.Providers()
	if err != nil {
		t.Fatalf("Providers failed: %v", err)
	}
	if len(providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(providers))
	}
}

func TestConnectionsEmpty(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	conns, err := app.Connections()
	if err != nil {
		t.Fatalf("Connections failed: %v", err)
	}
	if len(conns) != 0 {
		t.Errorf("expected 0 connections, got %d", len(conns))
	}
}

func TestNodeCRUDAndSearch(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	// Create nodes.
	desc := "A test workflow"
	content := "Workflow content"
	app.DB.CreateNode("wf1", "workflow", "Blog Post Pipeline", &desc, &content, nil, nil, nil)
	app.DB.CreateNode("sk1", "skill", "Language Polish", nil, nil, nil, nil, nil)
	app.DB.CreateNode("sk2", "skill", "SEO Optimisation", nil, nil, nil, nil, nil)

	// Test NodesByType.
	workflows, _ := app.NodesByType("workflow")
	if len(workflows) != 1 {
		t.Errorf("expected 1 workflow, got %d", len(workflows))
	}

	skills, _ := app.NodesByType("skill")
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}

	// Test Workflows convenience.
	wf, _ := app.Workflows()
	if len(wf) != 1 {
		t.Errorf("expected 1 workflow, got %d", len(wf))
	}

	// Test FindNodeByTitle.
	node, _ := app.FindNodeByTitle("Blog Post Pipeline")
	if node == nil {
		t.Fatal("expected to find Blog Post Pipeline")
	}
	if node.Type != "workflow" {
		t.Errorf("expected type workflow, got %s", node.Type)
	}

	// Case-insensitive.
	node, _ = app.FindNodeByTitle("blog post pipeline")
	if node == nil {
		t.Fatal("expected case-insensitive match")
	}

	// Test SearchNodes.
	results, _ := app.SearchNodes("polish")
	if len(results) != 1 {
		t.Errorf("expected 1 search result, got %d", len(results))
	}
	if results[0].Title != "Language Polish" {
		t.Errorf("expected Language Polish, got %s", results[0].Title)
	}

	// Search across types.
	results, _ = app.SearchNodes("o") // matches SEO Optimisation, Blog Post Pipeline
	if len(results) < 2 {
		t.Errorf("expected at least 2 results, got %d", len(results))
	}
}

func TestProfileCRUD(t *testing.T) {
	tmp := t.TempDir()
	app, err := Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer app.Close()

	app.DB.CreateProfile("p1", "voice", "Ben's Voice", nil, "content", nil)
	app.DB.CreateProfile("p2", "voice", "Formal", nil, "content", nil)

	profiles, _ := app.Profiles()
	if len(profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(profiles))
	}

	// Find by name.
	p, _ := app.FindProfileByName("Ben's Voice")
	if p == nil {
		t.Fatal("expected to find Ben's Voice")
	}

	// Case-insensitive.
	p, _ = app.FindProfileByName("ben's voice")
	if p == nil {
		t.Fatal("expected case-insensitive match")
	}

	// Set active.
	app.DB.SetActiveProfile("p1", "voice")
	active, _ := app.ActiveProfile("voice")
	if active == nil {
		t.Fatal("expected active profile")
	}
	if active.Name != "Ben's Voice" {
		t.Errorf("expected Ben's Voice, got %s", active.Name)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		start, end string
		expected   string
	}{
		{"2026-04-26T10:00:00Z", "2026-04-26T10:00:00Z", "0ms"},
		{"2026-04-26T10:00:00Z", "2026-04-26T10:00:00.500Z", "500ms"},
		{"2026-04-26T10:00:00Z", "2026-04-26T10:00:03Z", "3.0s"},
		{"2026-04-26T10:00:00Z", "2026-04-26T10:01:30Z", "90.0s"},
		{"invalid", "2026-04-26T10:00:00Z", ""},
	}

	for _, tt := range tests {
		result := formatDuration(tt.start, tt.end)
		if result != tt.expected {
			t.Errorf("formatDuration(%q, %q) = %q, want %q", tt.start, tt.end, result, tt.expected)
		}
	}
}
