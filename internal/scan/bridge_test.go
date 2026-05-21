package scan

import (
	"testing"

	"github.com/skrptiq/engine/manifest"
	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/engine/storage"
)

func TestParseToStorageSeverity(t *testing.T) {
	tests := []struct {
		in   manifest.Severity
		want storage.ValidationSeverity
	}{
		{manifest.SeverityError, storage.SeverityError},
		{manifest.SeverityWarn, storage.SeverityWarning},
		{manifest.SeverityInfo, storage.SeverityInfo},
		{manifest.Severity("unknown"), storage.SeverityWarning},
	}
	for _, tt := range tests {
		got := parseToStorageSeverity(tt.in)
		if got != tt.want {
			t.Errorf("parseToStorageSeverity(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestBridgeToHydration_BasicNodes(t *testing.T) {
	pkg := parse.Package{
		Manifest: manifest.Manifest{
			Name: "test-pkg",
			Raw:  map[string]any{"name": "test-pkg"},
		},
		Nodes: []parse.NodeFile{
			{
				ID:          "example-skill",
				Type:        "skill",
				Title:       "Example Skill",
				Description: "A test skill",
				Content:     "Do the thing.",
				Metadata:    map[string]any{"inputs": map[string]any{"topic": "string"}},
				FilePath:    "/tmp/test-pkg/skills/example-skill.md",
				FileHash:    "abc123",
			},
		},
		RootDir: "/tmp/test-pkg",
	}

	result := bridgeToHydration(pkg, "/tmp/test-pkg")

	if len(result.Input.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(result.Input.Nodes))
	}
	n := result.Input.Nodes[0]
	if n.ID != "example-skill" {
		t.Errorf("node ID = %q, want %q", n.ID, "example-skill")
	}
	if n.Title != "Example Skill" {
		t.Errorf("node Title = %q, want %q", n.Title, "Example Skill")
	}
	if n.Description == nil || *n.Description != "A test skill" {
		t.Errorf("node Description = %v, want %q", n.Description, "A test skill")
	}
	if n.Content == nil || *n.Content != "Do the thing." {
		t.Errorf("node Content = %v, want %q", n.Content, "Do the thing.")
	}
	if n.Metadata == nil || *n.Metadata == "" {
		t.Error("expected non-nil Metadata JSON")
	}
	if n.FileSlug == nil || *n.FileSlug != "example-skill" {
		t.Errorf("node FileSlug = %v, want %q", n.FileSlug, "example-skill")
	}
	if n.FileHash == nil || *n.FileHash != "abc123" {
		t.Errorf("node FileHash = %v, want %q", n.FileHash, "abc123")
	}
	if result.NodeFiles["example-skill"] != "skills/example-skill.md" {
		t.Errorf("nodeFiles = %q, want %q", result.NodeFiles["example-skill"], "skills/example-skill.md")
	}
	if len(result.Issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(result.Issues))
	}
}

func TestBridgeToHydration_EmptyMetadata(t *testing.T) {
	pkg := parse.Package{
		Manifest: manifest.Manifest{Raw: map[string]any{"name": "test"}},
		Nodes: []parse.NodeFile{
			{
				ID:       "bare-node",
				Type:     "prompt",
				Title:    "Bare",
				FilePath: "/tmp/test/prompts/bare-node.md",
			},
		},
		RootDir: "/tmp/test",
	}

	result := bridgeToHydration(pkg, "/tmp/test")
	n := result.Input.Nodes[0]
	if n.Metadata != nil {
		t.Errorf("expected nil Metadata for empty map, got %q", *n.Metadata)
	}
	if n.Description != nil {
		t.Errorf("expected nil Description for empty string, got %q", *n.Description)
	}
}

func TestBridgeToHydration_CrossPackageEdge(t *testing.T) {
	pkg := parse.Package{
		Manifest: manifest.Manifest{Raw: map[string]any{"name": "test"}},
		Nodes: []parse.NodeFile{
			{
				ID:       "my-skill",
				Type:     "skill",
				Title:    "My Skill",
				FilePath: "/tmp/test/skills/my-skill.md",
				Connections: []parse.Connection{
					{Target: "shared-content/brand-guide", Type: "uses"},
				},
			},
		},
		RootDir: "/tmp/test",
	}

	result := bridgeToHydration(pkg, "/tmp/test")

	// Cross-package edge should NOT produce an EdgeInput.
	if len(result.Input.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(result.Input.Edges))
	}
	// Should produce a warning-severity issue.
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}
	issue := result.Issues[0]
	if issue.Severity != storage.SeverityWarning {
		t.Errorf("issue severity = %q, want %q", issue.Severity, storage.SeverityWarning)
	}
	if issue.Code != "scan.cross_package_edge" {
		t.Errorf("issue code = %q, want %q", issue.Code, "scan.cross_package_edge")
	}
}

func TestBridgeToHydration_UnresolvedLocalEdge(t *testing.T) {
	pkg := parse.Package{
		Manifest: manifest.Manifest{Raw: map[string]any{"name": "test"}},
		Nodes: []parse.NodeFile{
			{
				ID:       "my-skill",
				Type:     "skill",
				Title:    "My Skill",
				FilePath: "/tmp/test/skills/my-skill.md",
				Connections: []parse.Connection{
					{Target: "nonexistent-node", Type: "uses"},
				},
			},
		},
		RootDir: "/tmp/test",
	}

	result := bridgeToHydration(pkg, "/tmp/test")

	if len(result.Input.Edges) != 0 {
		t.Errorf("expected 0 edges (unresolved), got %d", len(result.Input.Edges))
	}
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}
	issue := result.Issues[0]
	if issue.Severity != storage.SeverityError {
		t.Errorf("issue severity = %q, want %q", issue.Severity, storage.SeverityError)
	}
	if issue.Code != "scan.edge_target_unresolved" {
		t.Errorf("issue code = %q, want %q", issue.Code, "scan.edge_target_unresolved")
	}
}

func TestBridgeToHydration_ResolvedLocalEdge(t *testing.T) {
	pkg := parse.Package{
		Manifest: manifest.Manifest{Raw: map[string]any{"name": "test"}},
		Nodes: []parse.NodeFile{
			{
				ID:       "skill-a",
				Type:     "skill",
				Title:    "Skill A",
				FilePath: "/tmp/test/skills/skill-a.md",
				Connections: []parse.Connection{
					{Target: "prompt-b", Type: "uses"},
				},
			},
			{
				ID:       "prompt-b",
				Type:     "prompt",
				Title:    "Prompt B",
				FilePath: "/tmp/test/prompts/prompt-b.md",
			},
		},
		RootDir: "/tmp/test",
	}

	result := bridgeToHydration(pkg, "/tmp/test")

	if len(result.Input.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(result.Input.Edges))
	}
	e := result.Input.Edges[0]
	if e.SourceID != "skill-a" || e.TargetID != "prompt-b" {
		t.Errorf("edge = %s→%s, want skill-a→prompt-b", e.SourceID, e.TargetID)
	}
	if len(result.Issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(result.Issues))
	}
}

func TestBridgeToHydration_NamespaceFallback(t *testing.T) {
	pkg := parse.Package{
		Manifest: manifest.Manifest{Raw: map[string]any{}},
		RootDir:  "/tmp/my-cool-skrpt",
	}

	result := bridgeToHydration(pkg, "/tmp/my-cool-skrpt")
	name, ok := result.Input.Manifest["name"]
	if !ok || name != "my-cool-skrpt" {
		t.Errorf("manifest name = %v, want %q", name, "my-cool-skrpt")
	}
}

func TestParseIssuesToScanIssues(t *testing.T) {
	issues := []manifest.ParseIssue{
		{
			Severity: manifest.SeverityWarn,
			Code:     manifest.CodeManifestVersionMissing,
			Message:  "manifest.version is missing",
			File:     "skrptiq.yaml",
		},
		{
			Severity: manifest.SeverityError,
			Code:     manifest.CodeManifestNameMissing,
			Message:  "manifest.name is required",
			File:     "skrptiq.yaml",
		},
	}

	scanIssues := parseIssuesToScanIssues(issues)

	if len(scanIssues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(scanIssues))
	}
	// First issue: "warn" → "warning"
	if scanIssues[0].Severity != storage.SeverityWarning {
		t.Errorf("issue[0] severity = %q, want %q", scanIssues[0].Severity, storage.SeverityWarning)
	}
	if scanIssues[0].Code != string(manifest.CodeManifestVersionMissing) {
		t.Errorf("issue[0] code = %q, want %q", scanIssues[0].Code, manifest.CodeManifestVersionMissing)
	}
	// Second issue: "error" → "error"
	if scanIssues[1].Severity != storage.SeverityError {
		t.Errorf("issue[1] severity = %q, want %q", scanIssues[1].Severity, storage.SeverityError)
	}
}
