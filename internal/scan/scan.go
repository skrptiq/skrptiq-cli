package scan

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/skrptiq/engine/parse"
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
	Path       string        `json:"path"`
	NodeCount  int           `json:"nodeCount"`
	EdgeCount  int           `json:"edgeCount"`
	Issues     []ScanIssue   `json:"issues"`
	ErrorCount int           `json:"errorCount"`
	WarnCount  int           `json:"warnCount"`
	InfoCount  int           `json:"infoCount"`
	Package    *parse.Package `json:"package,omitempty"`
}

// Run executes the scan, writing output to stdout, and returns the exit code (0 pass, 1 warnings, 2 errors).
func Run(scanPath string, jsonOutput bool) int {
	return RunTo(scanPath, jsonOutput, os.Stdout)
}

// RunTo executes the scan, writing output to w, and returns the exit code.
//
// F12: delegates directory reading to engine/parse.ReadPackage (the
// canonical single-source reader). The bridge function converts the
// parsed Package into storage.HydrationInput for the temp-DB hydration
// step. Workflow-level validation still needs the full DB state.
func RunTo(scanPath string, jsonOutput bool, w io.Writer) int {
	absPath, err := filepath.Abs(scanPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	// 1. Read the package via the canonical engine reader.
	pkg, parseIssues, err := parse.ReadPackage(absPath)
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

	// 3. Convert parse.Package → storage.HydrationInput via bridge.
	br := bridgeToHydration(pkg, absPath)

	// Collect all issues: parse-time + bridge-time.
	var allIssues []ScanIssue
	allIssues = append(allIssues, parseIssuesToScanIssues(parseIssues)...)
	allIssues = append(allIssues, br.Issues...)

	// 4. Hydrate into the temp DB.
	hydration, err := db.HydratePackage(br.Input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error hydrating package: %v\n", err)
		return 2
	}

	// 5. Convert hydration issues back into ScanIssue with file context.
	for nodeID, issues := range hydration.IssuesByNodeID {
		file := br.NodeFiles[nodeID]
		for _, issue := range issues {
			allIssues = append(allIssues, ScanIssue{
				File:            file,
				NodeSlug:        nodeID,
				ValidationIssue: issue,
			})
		}
	}
	for edgeID, issues := range hydration.IssuesByEdgeID {
		ctx := br.EdgeFiles[edgeID]
		for _, issue := range issues {
			allIssues = append(allIssues, ScanIssue{
				File:            ctx.file,
				NodeSlug:        ctx.sourceSlug,
				ValidationIssue: issue,
			})
		}
	}

	// 6. Validate workflows. Workflow-context checks (binding coverage,
	// step-ref resolution, etc.) need the full DB state.
	for _, nf := range pkg.Nodes {
		if nf.Type != "workflow" {
			continue
		}
		rel := relPath(absPath, nf.FilePath)
		issues := db.ValidateWorkflow(nf.ID)
		for _, issue := range issues {
			allIssues = append(allIssues, ScanIssue{
				File:            rel,
				NodeSlug:        nf.ID,
				ValidationIssue: issue,
			})
		}
	}

	// 7. Build result and output.
	// Strip host-specific FilePath from nodes for portable JSON output.
	// RootDir is kept — it matches result.Path.
	for i := range pkg.Nodes {
		pkg.Nodes[i].FilePath = ""
	}
	result := ScanResult{
		Path:      absPath,
		NodeCount: hydration.NodesInserted,
		EdgeCount: hydration.EdgesInserted,
		Issues:    allIssues,
		Package:   &pkg,
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

	// 8. Exit code.
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
