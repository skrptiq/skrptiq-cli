package bridge

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// The authoritative browser-tool catalogue the MCP-mode bridge serves to the
// engine via tools/list — answered even with no browser connected (the connected
// extension's announce.tools declares what it can actually serve).
//
// K-055/K-049 single-source rule: the catalogue is DATA owned by the extension
// SoT (skrptiq-extension companion/src/catalog.ts). We do NOT hand-maintain a Go
// copy. scripts/regen-catalog.mjs mechanically derives catalog.json from the SoT
// (provenance pinned in catalog.provenance.json); this file EMBEDS and loads that
// same JSON at runtime — so the runtime source and the conformance golden are one
// file, nothing to drift.

//go:embed catalog.json
var catalogJSON []byte

// ToolAnnotations mirrors the MCP risk hints the engine's gate (mcp/gate.go
// ResolveToolGate) reads: destructiveHint gates a tool by declared risk.
type ToolAnnotations struct {
	ReadOnlyHint    *bool `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool `json:"destructiveHint,omitempty"`
}

// CatalogTool is one entry served on tools/list.
type CatalogTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Annotations *ToolAnnotations       `json:"annotations,omitempty"`
}

// loadCatalog parses the embedded catalogue. It fails loud on a malformed or
// empty catalogue — a broken catalogue is a build/regen fault, never a silent
// empty tools/list.
func loadCatalog() ([]CatalogTool, error) {
	var tools []CatalogTool
	if err := json.Unmarshal(catalogJSON, &tools); err != nil {
		return nil, fmt.Errorf("parse embedded catalog.json: %w", err)
	}
	if len(tools) == 0 {
		return nil, fmt.Errorf("embedded catalog.json is empty — regen from the extension SoT")
	}
	return tools, nil
}

// catalogToolNames returns the set of catalogue tool names for O(1) gating.
func catalogToolNames(tools []CatalogTool) map[string]struct{} {
	set := make(map[string]struct{}, len(tools))
	for _, t := range tools {
		set[t.Name] = struct{}{}
	}
	return set
}
