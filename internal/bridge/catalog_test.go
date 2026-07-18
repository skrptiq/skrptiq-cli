package bridge

import (
	"encoding/json"
	"os"
	"testing"
)

func TestLoadCatalog_EmbeddedIsValid(t *testing.T) {
	tools, err := loadCatalog()
	if err != nil {
		t.Fatalf("loadCatalog: %v", err)
	}
	if len(tools) == 0 {
		t.Fatal("embedded catalog is empty")
	}
	for _, tool := range tools {
		if tool.Name == "" {
			t.Errorf("catalog tool with empty name: %+v", tool)
		}
		if tool.Description == "" {
			t.Errorf("catalog tool %q has no description", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("catalog tool %q has no inputSchema", tool.Name)
		}
	}
}

func TestCatalog_DestructiveToolsAnnotated(t *testing.T) {
	// The engine's gate (mcp/gate.go) reads destructiveHint to auto-gate writes;
	// if the SoT marks a tool destructive, our vendored JSON must carry it.
	tools, _ := loadCatalog()
	names := catalogToolNames(tools)
	for _, want := range []string{"click", "fill", "navigate"} {
		if _, ok := names[want]; !ok {
			t.Errorf("expected write tool %q in catalog", want)
		}
	}
	for _, tool := range tools {
		if tool.Name == "click" || tool.Name == "fill" || tool.Name == "navigate" {
			if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
				t.Errorf("write tool %q must carry destructiveHint:true (engine gate depends on it)", tool.Name)
			}
		}
	}
}

// The provenance file must agree with the vendored catalog — a mismatch means a
// hand-edit slipped in without a regen from the SoT.
func TestCatalog_ProvenanceMatches(t *testing.T) {
	raw, err := os.ReadFile("catalog.provenance.json")
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}
	var prov struct {
		ToolCount int      `json:"toolCount"`
		ToolNames []string `json:"toolNames"`
	}
	if err := json.Unmarshal(raw, &prov); err != nil {
		t.Fatalf("parse provenance: %v", err)
	}
	tools, _ := loadCatalog()
	if prov.ToolCount != len(tools) {
		t.Errorf("provenance toolCount %d != catalog %d — regen after editing", prov.ToolCount, len(tools))
	}
	names := catalogToolNames(tools)
	for _, n := range prov.ToolNames {
		if _, ok := names[n]; !ok {
			t.Errorf("provenance lists %q but catalog does not — stale provenance", n)
		}
	}
}
