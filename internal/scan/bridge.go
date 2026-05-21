package scan

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/skrptiq/engine/manifest"
	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/engine/storage"
)

// bridgeResult holds the output of bridgeToHydration — everything the
// scanner needs to feed HydratePackage and attribute issues to files.
type bridgeResult struct {
	Input     storage.HydrationInput
	Issues    []ScanIssue
	NodeFiles map[string]string      // nodeID → relative file path
	EdgeFiles map[string]edgeFileCtx // edgeID → file context
}

// bridgeToHydration converts a parse.Package into a storage.HydrationInput
// plus any scan-level issues (cross-package edges, unresolved targets).
//
// The bridge is the sole point where parse.Package types are translated
// into storage types. No other scan code touches parse types directly
// beyond calling ReadPackage.
func bridgeToHydration(pkg parse.Package, absPath string) bridgeResult {
	manifestRaw := pkg.Manifest.Raw
	if manifestRaw == nil {
		manifestRaw = make(map[string]any)
	}
	// Namespace fallback — same logic the old scanner used.
	if manifestRaw["name"] == nil || manifestRaw["name"] == "" {
		manifestRaw["name"] = filepath.Base(absPath)
	}

	nodeIDSet := make(map[string]bool, len(pkg.Nodes))
	nodeFiles := make(map[string]string, len(pkg.Nodes))
	nodes := make([]storage.NodeInput, 0, len(pkg.Nodes))

	for _, nf := range pkg.Nodes {
		rel := relPath(absPath, nf.FilePath)
		nodeIDSet[nf.ID] = true
		nodeFiles[nf.ID] = rel

		var desc, content, metadata, fileSlug, fileHash *string
		if nf.Description != "" {
			d := nf.Description
			desc = &d
		}
		if nf.Content != "" {
			c := nf.Content
			content = &c
		}
		if len(nf.Metadata) > 0 {
			mj := marshalJSON(nf.Metadata)
			metadata = &mj
		}
		slug := nf.ID
		fileSlug = &slug
		if nf.FileHash != "" {
			h := nf.FileHash
			fileHash = &h
		}

		nodes = append(nodes, storage.NodeInput{
			ID:          nf.ID,
			Type:        nf.Type,
			Title:       nf.Title,
			Description: desc,
			Content:     content,
			Metadata:    metadata,
			FileSlug:    fileSlug,
			FileHash:    fileHash,
		})
	}

	// Build edges, handling cross-package targets (GH#580).
	var issues []ScanIssue
	edges := make([]storage.EdgeInput, 0)
	edgeFiles := make(map[string]edgeFileCtx)

	for _, nf := range pkg.Nodes {
		rel := relPath(absPath, nf.FilePath)
		for _, conn := range nf.Connections {
			edgeID := fmt.Sprintf("%s--%s--%s", nf.ID, conn.Type, conn.Target)

			// Cross-package edge: target contains "/" (namespace/slug).
			// Legal at runtime but unresolvable in a single-package scan.
			if strings.Contains(conn.Target, "/") {
				issues = append(issues, ScanIssue{
					File:     rel,
					NodeSlug: nf.ID,
					ValidationIssue: storage.ValidationIssue{
						Code:     "scan.cross_package_edge",
						Severity: storage.SeverityWarning,
						Message:  fmt.Sprintf("cross-package edge target %q — will be resolved at runtime", conn.Target),
						Field:    "connections",
					},
				})
				continue
			}

			if !nodeIDSet[conn.Target] {
				issues = append(issues, ScanIssue{
					File:     rel,
					NodeSlug: nf.ID,
					ValidationIssue: makeIssue("scan.edge_target_unresolved", "error",
						fmt.Sprintf("connection target %q not found", conn.Target),
						"§6.4", "connections"),
				})
				continue
			}

			edges = append(edges, storage.EdgeInput{
				ID:       edgeID,
				SourceID: nf.ID,
				TargetID: conn.Target,
				Type:     conn.Type,
				Position: conn.Position,
			})
			edgeFiles[edgeID] = edgeFileCtx{file: rel, sourceSlug: nf.ID}
		}
	}

	return bridgeResult{
		Input: storage.HydrationInput{
			HubImportID: "scan-import",
			Manifest:    manifestRaw,
			Nodes:       nodes,
			Edges:       edges,
		},
		Issues:    issues,
		NodeFiles: nodeFiles,
		EdgeFiles: edgeFiles,
	}
}

// parseIssuesToScanIssues converts engine manifest.ParseIssue records
// into the CLI's ScanIssue type, mapping severity strings along the way.
func parseIssuesToScanIssues(issues []manifest.ParseIssue) []ScanIssue {
	out := make([]ScanIssue, 0, len(issues))
	for _, pi := range issues {
		out = append(out, ScanIssue{
			File: pi.File,
			ValidationIssue: storage.ValidationIssue{
				Code:     string(pi.Code),
				Severity: parseToStorageSeverity(pi.Severity),
				Message:  pi.Message,
			},
		})
	}
	return out
}

// parseToStorageSeverity maps manifest.Severity ("warn") to
// storage.ValidationSeverity ("warning"). The vocabulary difference is
// a known drift surface (W-CLI-1); Wave 4 F8 consolidates it.
func parseToStorageSeverity(s manifest.Severity) storage.ValidationSeverity {
	switch s {
	case manifest.SeverityError:
		return storage.SeverityError
	case manifest.SeverityWarn:
		return storage.SeverityWarning
	case manifest.SeverityInfo:
		return storage.SeverityInfo
	default:
		return storage.SeverityWarning
	}
}

// relPath returns the path of full relative to base.
func relPath(base, full string) string {
	rel, err := filepath.Rel(base, full)
	if err != nil {
		return full
	}
	return rel
}

// marshalJSON marshals a value to a JSON string.
func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
