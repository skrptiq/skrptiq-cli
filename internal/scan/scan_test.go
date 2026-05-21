package scan

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
)

func testdataPath(name string) string {
	return filepath.Join("testdata", name)
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

// GH#524: template-flagged packages may use {{UPPERCASE}} fill-in-the-
// blank markers in prompts. The scanner must propagate `template: true`
// from skrptiq.yaml into the temp DB's `skrpt_manifests` row AND set
// `Namespace` on every node — without both, the engine's
// IsTemplatePackage check returns false and the warning still fires.
// This test would have caught the original ship-and-forget bug.
func TestScan_TemplatePackage_SuppressesUnknownNamespaceWarnings(t *testing.T) {
	result := runScan(t, testdataPath("template-package"))
	if result.WarnCount > 0 || result.ErrorCount > 0 {
		t.Errorf("expected 0 issues for template package, got %d errors + %d warnings",
			result.ErrorCount, result.WarnCount)
		for _, issue := range result.Issues {
			t.Logf("  %s: %s", issue.Code, issue.Message)
		}
	}
	for _, issue := range result.Issues {
		if issue.Code == "prompt.unknown_namespace" {
			t.Errorf("template package should not emit unknown_namespace: %s", issue.Message)
		}
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

// GH#580: cross-package edges (target contains "/") should produce a
// warning, not an error. They're legal at runtime but unresolvable in
// a single-package scan.
func TestScan_CrossPackageEdge_WarningNotError(t *testing.T) {
	result := runScan(t, testdataPath("cross-package-edge"))
	// Should have a warning for the cross-package edge.
	found := false
	for _, issue := range result.Issues {
		if issue.Code == "scan.cross_package_edge" {
			found = true
			if issue.Severity != "warning" {
				t.Errorf("expected warning severity, got %q", issue.Severity)
			}
			break
		}
	}
	if !found {
		t.Error("expected scan.cross_package_edge issue")
		for _, i := range result.Issues {
			t.Logf("  %s: %s (%s)", i.Code, i.Message, i.File)
		}
	}
	// Must NOT have an error for the cross-package target.
	for _, issue := range result.Issues {
		if issue.Code == "scan.edge_target_unresolved" {
			t.Errorf("cross-package edge should not produce edge_target_unresolved: %s", issue.Message)
		}
	}
	// Exit code should be 1 (warning), not 2 (error).
	var buf bytes.Buffer
	code := RunTo(testdataPath("cross-package-edge"), true, &buf)
	if code != 1 {
		t.Errorf("expected exit code 1 (warning) for cross-package edge, got %d", code)
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
