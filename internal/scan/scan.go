package scan

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/skrptiq/engine/storage"
)

// ScanIssue wraps a ValidationIssue with file context.
type ScanIssue struct {
	File     string `json:"file"`
	NodeSlug string `json:"nodeSlug,omitempty"`
	storage.ValidationIssue
}

// ScanResult is the complete scan output.
type ScanResult struct {
	Path       string      `json:"path"`
	NodeCount  int         `json:"nodeCount"`
	EdgeCount  int         `json:"edgeCount"`
	Issues     []ScanIssue `json:"issues"`
	ErrorCount int         `json:"errorCount"`
	WarnCount  int         `json:"warnCount"`
	InfoCount  int         `json:"infoCount"`
}

// Run executes the scan, writing output to stdout, and returns the exit code (0 pass, 1 warnings, 2 errors).
func Run(scanPath string, jsonOutput bool) int {
	return RunTo(scanPath, jsonOutput, os.Stdout)
}

// RunTo executes the scan, writing output to w, and returns the exit code.
//
// GH#525: hydration goes through engine.HydratePackage, which is the
// single source of truth for "parsed inputs → populated engine DB". The
// scanner builds HydrationInput slices from parsed files + manifest,
// hands them to HydratePackage, and surfaces the returned issues
// alongside its own per-file context. Workflow-level validation runs
// after hydration since it needs the full DB state.
func RunTo(scanPath string, jsonOutput bool, w io.Writer) int {
	absPath, err := filepath.Abs(scanPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	// 1. Parse the skrpt directory.
	files, manifest, parseIssues, err := ParseDirectory(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	// 2. Open a temp DB.
	tmpDir, err := os.MkdirTemp("", "skrptiq-scan-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp directory: %v\n", err)
		return 2
	}
	defer os.RemoveAll(tmpDir)

	db, err := storage.Open(filepath.Join(tmpDir, "scan.db"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening temp database: %v\n", err)
		return 2
	}
	defer db.Close()

	// Manifest's name is the namespace HydratePackage will stamp.
	// Fall back to the directory name if the manifest doesn't carry one
	// (older catalogue skrpts) — same fallback the previous flow used.
	if _, ok := manifest["name"]; !ok || manifest["name"] == nil || manifest["name"] == "" {
		manifest["name"] = filepath.Base(absPath)
	}

	// 3. Build HydrationInput from the parsed files. Also keep a
	// nodeID → file map so issues HydratePackage returns can be
	// re-attached to their source file for the user-facing report.
	nodeFiles := make(map[string]string, len(files))
	nodeIDSet := make(map[string]bool, len(files))
	nodes := make([]storage.NodeInput, 0, len(files))
	for _, f := range files {
		id := f.Frontmatter.ID
		nodeFiles[id] = relPath(absPath, f.Path)
		nodeIDSet[id] = true

		var desc, content, metadata, fileSlug *string
		if f.Frontmatter.Description != "" {
			d := f.Frontmatter.Description
			desc = &d
		}
		if f.Body != "" {
			c := f.Body
			content = &c
		}
		if mj := BuildMetadataJSON(f.Frontmatter); mj != "" {
			metadata = &mj
		}
		slug := id
		fileSlug = &slug

		nodes = append(nodes, storage.NodeInput{
			ID:          id,
			Type:        f.Frontmatter.Type,
			Title:       f.Frontmatter.Title,
			Description: desc,
			Content:     content,
			Metadata:    metadata,
			FileSlug:    fileSlug,
		})
	}

	// 4. Build EdgeInput slice, pre-filtering edges whose target slug
	// isn't in the node set. Those get scan.edge_target_unresolved
	// without being passed to HydratePackage — HydratePackage assumes
	// FK targets resolve, and dangling references are a scan-specific
	// failure mode, not a hydration one.
	var allIssues []ScanIssue
	allIssues = append(allIssues, parseIssues...)

	edgeFiles := make(map[string]edgeFileCtx, len(files))
	edges := make([]storage.EdgeInput, 0)
	for _, f := range files {
		rel := relPath(absPath, f.Path)
		sourceSlug := f.Frontmatter.ID
		for _, conn := range f.Frontmatter.Connections {
			edgeID := fmt.Sprintf("%s--%s--%s", sourceSlug, conn.Type, conn.Target)
			if !nodeIDSet[conn.Target] {
				allIssues = append(allIssues, ScanIssue{
					File:     rel,
					NodeSlug: sourceSlug,
					ValidationIssue: makeIssue("scan.edge_target_unresolved", "error",
						fmt.Sprintf("connection target %q not found", conn.Target),
						"§6.4", "connections"),
				})
				continue
			}
			edges = append(edges, storage.EdgeInput{
				ID:       edgeID,
				SourceID: sourceSlug,
				TargetID: conn.Target,
				Type:     conn.Type,
				Position: conn.Position,
			})
			edgeFiles[edgeID] = edgeFileCtx{file: rel, sourceSlug: sourceSlug}
		}
	}

	// 5. Hydrate. The single owner of parsed-input → engine-DB.
	hydration, err := db.HydratePackage(storage.HydrationInput{
		HubImportID: "scan-import",
		Manifest:    manifest,
		Nodes:       nodes,
		Edges:       edges,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error hydrating package: %v\n", err)
		return 2
	}

	// 6. Convert hydration issues back into ScanIssue with file context.
	for nodeID, issues := range hydration.IssuesByNodeID {
		file := nodeFiles[nodeID]
		for _, issue := range issues {
			allIssues = append(allIssues, ScanIssue{
				File:            file,
				NodeSlug:        nodeID,
				ValidationIssue: issue,
			})
		}
	}
	for edgeID, issues := range hydration.IssuesByEdgeID {
		ctx := edgeFiles[edgeID]
		for _, issue := range issues {
			allIssues = append(allIssues, ScanIssue{
				File:            ctx.file,
				NodeSlug:        ctx.sourceSlug,
				ValidationIssue: issue,
			})
		}
	}

	// 7. Validate workflows. HydratePackage runs node + edge validation
	// but the workflow-context checks (GH#511 binding coverage, GH#512
	// step-ref resolution, etc.) need the full DB state and live in
	// ValidateWorkflow. Run them per workflow file so issues attribute
	// back to the right file.
	for _, f := range files {
		if f.Frontmatter.Type != "workflow" {
			continue
		}
		rel := relPath(absPath, f.Path)
		issues := db.ValidateWorkflow(f.Frontmatter.ID)
		for _, issue := range issues {
			allIssues = append(allIssues, ScanIssue{
				File:            rel,
				NodeSlug:        f.Frontmatter.ID,
				ValidationIssue: issue,
			})
		}
	}

	// 8. Build result and output.
	result := ScanResult{
		Path:      absPath,
		NodeCount: hydration.NodesInserted,
		EdgeCount: hydration.EdgesInserted,
		Issues:    allIssues,
	}
	for _, issue := range allIssues {
		switch issue.Severity {
		case storage.SeverityError:
			result.ErrorCount++
		case storage.SeverityWarning:
			result.WarnCount++
		case storage.SeverityInfo:
			result.InfoCount++
		}
	}

	if jsonOutput {
		OutputJSON(result, w)
	} else {
		OutputTable(result, w)
	}

	// 9. Exit code.
	if result.ErrorCount > 0 {
		return 2
	}
	if result.WarnCount > 0 {
		return 1
	}
	return 0
}

// edgeFileCtx tracks the originating file + source slug for a built
// EdgeInput so we can re-attach file context to any issue HydratePackage
// surfaces against the edge.
type edgeFileCtx struct {
	file       string
	sourceSlug string
}

// makeIssue creates a ValidationIssue.
func makeIssue(code string, severity string, message, contractRef, field string) storage.ValidationIssue {
	return storage.ValidationIssue{
		Code:        code,
		Severity:    storage.ValidationSeverity(severity),
		Message:     message,
		ContractRef: contractRef,
		Field:       field,
	}
}

// marshalJSON marshals a value to a JSON string.
func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
