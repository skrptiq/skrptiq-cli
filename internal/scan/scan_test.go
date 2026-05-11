package scan

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"os"
)

func testdataPath(name string) string {
	return filepath.Join("testdata", name)
}

// --- Parse tests ---

func TestParseFile_ValidSkill(t *testing.T) {
	f, err := ParseFile(filepath.Join(testdataPath("valid-package"), "skills", "example-skill.md"), "skills")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if f.Frontmatter.Type != "skill" {
		t.Errorf("expected type skill, got %q", f.Frontmatter.Type)
	}
	if f.Frontmatter.ID != "example-skill" {
		t.Errorf("expected id example-skill, got %q", f.Frontmatter.ID)
	}
	if f.Frontmatter.Title != "Example Skill" {
		t.Errorf("expected title Example Skill, got %q", f.Frontmatter.Title)
	}
	if len(f.Frontmatter.Connections) != 1 {
		t.Errorf("expected 1 connection, got %d", len(f.Frontmatter.Connections))
	}
	if !strings.Contains(f.Body, "valid node") {
		t.Errorf("expected body to contain 'valid node', got %q", f.Body)
	}
}

func TestParseFile_EmptyIDDerivesFromFilename(t *testing.T) {
	// Create a temp file without an id field.
	tmp := t.TempDir()
	content := "---\ntype: skill\ntitle: \"No ID\"\n---\nBody text."
	os.WriteFile(filepath.Join(tmp, "derived-slug.md"), []byte(content), 0644)

	f, err := ParseFile(filepath.Join(tmp, "derived-slug.md"), "skills")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if f.Frontmatter.ID != "derived-slug" {
		t.Errorf("expected id derived-slug, got %q", f.Frontmatter.ID)
	}
}

func TestParseFile_NoFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "no-fm.md"), []byte("Just text, no frontmatter."), 0644)
	_, err := ParseFile(filepath.Join(tmp, "no-fm.md"), "skills")
	if err == nil {
		t.Error("expected error for file without frontmatter")
	}
}

func TestParseFile_Connections(t *testing.T) {
	f, err := ParseFile(filepath.Join(testdataPath("valid-package"), "workflows", "example-workflow.md"), "workflows")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if len(f.Frontmatter.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(f.Frontmatter.Connections))
	}
	conn := f.Frontmatter.Connections[0]
	if conn.Target != "example-skill" {
		t.Errorf("expected target example-skill, got %q", conn.Target)
	}
	if conn.Type != "uses" {
		t.Errorf("expected type uses, got %q", conn.Type)
	}
	if conn.Position == nil || *conn.Position != 0 {
		t.Error("expected position 0")
	}
}

func TestParseDirectory_MissingManifest(t *testing.T) {
	tmp := t.TempDir()
	_, _, err := ParseDirectory(tmp)
	if err == nil {
		t.Error("expected error for directory without skrptiq.yaml")
	}
}

func TestParseDirectory_ValidPackage(t *testing.T) {
	files, issues, err := ParseDirectory(testdataPath("valid-package"))
	if err != nil {
		t.Fatalf("ParseDirectory failed: %v", err)
	}
	if len(files) != 4 {
		t.Errorf("expected 4 files, got %d", len(files))
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 parse issues, got %d", len(issues))
		for _, i := range issues {
			t.Logf("  %s: %s", i.Code, i.Message)
		}
	}
}

// --- Scan integration tests ---

func TestScan_ValidPackage(t *testing.T) {
	var buf bytes.Buffer
	code := RunTo(testdataPath("valid-package"), true, &buf)
	if code != 0 {
		t.Errorf("expected exit code 0 for valid package, got %d", code)
		t.Logf("output: %s", buf.String())
	}
}

func TestScan_InvalidSlug(t *testing.T) {
	// The invalid package has Bad_Slug which should trigger node.invalid_file_slug.
	result := runScan(t, testdataPath("invalid-package"))
	if result.ErrorCount == 0 {
		t.Error("expected at least 1 error for invalid package")
	}
	// Check for the specific issue code.
	found := false
	for _, issue := range result.Issues {
		if issue.Code == "node.invalid_file_slug" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected node.invalid_file_slug issue")
		for _, i := range result.Issues {
			t.Logf("  %s: %s (%s)", i.Code, i.Message, i.File)
		}
	}
}

func TestScan_EmptyPromptBody(t *testing.T) {
	result := runScan(t, testdataPath("invalid-package"))
	found := false
	for _, issue := range result.Issues {
		if issue.Code == "prompt.empty_body" {
			found = true
			if issue.Severity != "warning" {
				t.Errorf("expected warning severity, got %q", issue.Severity)
			}
			break
		}
	}
	if !found {
		t.Error("expected prompt.empty_body issue")
	}
}

func TestScan_BrokenEdgeTarget(t *testing.T) {
	result := runScan(t, testdataPath("invalid-package"))
	found := false
	for _, issue := range result.Issues {
		if issue.Code == "scan.edge_target_unresolved" {
			found = true
			if issue.Severity != "error" {
				t.Errorf("expected error severity for unresolved target, got %q", issue.Severity)
			}
			break
		}
	}
	if !found {
		t.Error("expected scan.edge_target_unresolved issue")
		for _, i := range result.Issues {
			t.Logf("  %s: %s (%s)", i.Code, i.Message, i.File)
		}
	}
}

func TestScan_ExitCodes(t *testing.T) {
	var buf bytes.Buffer
	validCode := RunTo(testdataPath("valid-package"), true, &buf)
	if validCode != 0 {
		t.Errorf("valid package: expected exit 0, got %d", validCode)
	}

	buf.Reset()
	invalidCode := RunTo(testdataPath("invalid-package"), true, &buf)
	if invalidCode != 2 {
		t.Errorf("invalid package: expected exit 2, got %d", invalidCode)
	}
}

func TestScan_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	RunTo(testdataPath("valid-package"), true, &buf)

	var result ScanResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON output not parseable: %v\nOutput: %s", err, buf.String())
	}
	if result.NodeCount == 0 {
		t.Error("expected nodeCount > 0 in JSON output")
	}
}

// runScan is a test helper that captures the scan result via JSON.
func runScan(t *testing.T, path string) ScanResult {
	t.Helper()
	var buf bytes.Buffer
	RunTo(path, true, &buf)

	var result ScanResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON output not parseable: %v\nOutput: %s", err, buf.String())
	}
	return result
}
