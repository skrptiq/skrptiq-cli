package scan

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

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
func RunTo(scanPath string, jsonOutput bool, w io.Writer) int {
	absPath, err := filepath.Abs(scanPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	// 1. Parse the skrpt directory.
	files, parseIssues, err := ParseDirectory(absPath)
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

	var allIssues []ScanIssue
	allIssues = append(allIssues, parseIssues...)

	nodeCount := 0
	edgeCount := 0

	// 3. Load nodes — validate separately, insert with raw SQL.
	for _, f := range files {
		rel := relPath(absPath, f.Path)
		slug := f.Frontmatter.ID
		node := buildNode(f)

		// Validate.
		issues := db.ValidateNode(node)
		for _, issue := range issues {
			allIssues = append(allIssues, ScanIssue{
				File:            rel,
				NodeSlug:        slug,
				ValidationIssue: issue,
			})
		}

		// Insert via raw SQL (bypass validation gate).
		if err := insertNodeRaw(db, node); err != nil {
			// If insert fails (e.g. CHECK constraint on type), record it.
			allIssues = append(allIssues, ScanIssue{
				File:            rel,
				NodeSlug:        slug,
				ValidationIssue: makeIssue("scan.insert_failed", "error", err.Error(), "", ""),
			})
			continue
		}
		nodeCount++
	}

	// 4. Load edges — resolve targets, validate, insert.
	for _, f := range files {
		rel := relPath(absPath, f.Path)
		sourceSlug := f.Frontmatter.ID

		for _, conn := range f.Frontmatter.Connections {
			targetSlug := conn.Target
			edgeID := fmt.Sprintf("%s--%s--%s", sourceSlug, conn.Type, targetSlug)

			// Check target exists.
			targetNode, _ := db.GetNode(targetSlug)
			if targetNode == nil {
				allIssues = append(allIssues, ScanIssue{
					File:     rel,
					NodeSlug: sourceSlug,
					ValidationIssue: makeIssue("scan.edge_target_unresolved", "error",
						fmt.Sprintf("connection target %q not found", targetSlug),
						"§6.4", "connections"),
				})
				continue
			}

			edge := storage.Edge{
				ID:       edgeID,
				SourceID: sourceSlug,
				TargetID: targetSlug,
				Type:     conn.Type,
				Position: conn.Position,
			}

			// Validate.
			issues := db.ValidateEdge(edge)
			for _, issue := range issues {
				allIssues = append(allIssues, ScanIssue{
					File:            rel,
					NodeSlug:        sourceSlug,
					ValidationIssue: issue,
				})
			}

			// Insert.
			if err := insertEdgeRaw(db, edge); err != nil {
				allIssues = append(allIssues, ScanIssue{
					File:            rel,
					NodeSlug:        sourceSlug,
					ValidationIssue: makeIssue("scan.insert_failed", "error", err.Error(), "", ""),
				})
				continue
			}
			edgeCount++
		}
	}

	// 5. Validate workflows.
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

	// 6. Build result and output.
	result := ScanResult{
		Path:      absPath,
		NodeCount: nodeCount,
		EdgeCount: edgeCount,
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

	// 7. Exit code.
	if result.ErrorCount > 0 {
		return 2
	}
	if result.WarnCount > 0 {
		return 1
	}
	return 0
}

// buildNode constructs a storage.Node from a ParsedFile.
func buildNode(f ParsedFile) storage.Node {
	slug := f.Frontmatter.ID
	now := time.Now().UTC().Format(time.RFC3339)

	var desc, content, metadata, fileSlug *string
	if f.Frontmatter.Description != "" {
		d := f.Frontmatter.Description
		desc = &d
	}
	if f.Body != "" {
		c := f.Body
		content = &c
	}
	metaJSON := BuildMetadataJSON(f.Frontmatter)
	if metaJSON != "" {
		metadata = &metaJSON
	}
	fileSlug = &slug

	return storage.Node{
		ID:        slug,
		Type:      f.Frontmatter.Type,
		Title:     f.Frontmatter.Title,
		Description: desc,
		Content:   content,
		Metadata:  metadata,
		FileSlug:  fileSlug,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// insertNodeRaw inserts a node via raw SQL, bypassing the validation gate.
func insertNodeRaw(db *storage.DB, n storage.Node) error {
	_, err := db.Exec(
		`INSERT OR REPLACE INTO nodes (id, type, title, description, content, metadata, file_slug, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Type, n.Title, n.Description, n.Content, n.Metadata, n.FileSlug, n.CreatedAt, n.UpdatedAt,
	)
	return err
}

// insertEdgeRaw inserts an edge via raw SQL, bypassing the validation gate.
func insertEdgeRaw(db *storage.DB, e storage.Edge) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT OR REPLACE INTO edges (id, source_id, target_id, type, position, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID, e.SourceID, e.TargetID, e.Type, e.Position, now,
	)
	return err
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
