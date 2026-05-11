package scan

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/skrptiq/engine/storage"
)

// OutputJSON writes the scan result as indented JSON.
func OutputJSON(result ScanResult, w io.Writer) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(result)
}

// OutputTable writes the scan result as a human-readable table.
func OutputTable(result ScanResult, w io.Writer) {
	fmt.Fprintf(w, "Scanning %s\n", result.Path)
	fmt.Fprintf(w, "  %d nodes, %d edges\n\n", result.NodeCount, result.EdgeCount)

	if len(result.Issues) == 0 {
		fmt.Fprintln(w, "  No issues found.")
		return
	}

	// Group by file for readability.
	byFile := make(map[string][]ScanIssue)
	var fileOrder []string
	for _, issue := range result.Issues {
		if _, seen := byFile[issue.File]; !seen {
			fileOrder = append(fileOrder, issue.File)
		}
		byFile[issue.File] = append(byFile[issue.File], issue)
	}

	for _, file := range fileOrder {
		fmt.Fprintf(w, "  %s\n", file)
		for _, issue := range byFile[file] {
			sev := severityLabel(issue.Severity)
			fmt.Fprintf(w, "    %s  %s  %s\n", sev, issue.Code, issue.Message)
		}
		fmt.Fprintln(w)
	}

	// Summary line.
	var parts []string
	if result.ErrorCount > 0 {
		parts = append(parts, fmt.Sprintf("%d error(s)", result.ErrorCount))
	}
	if result.WarnCount > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", result.WarnCount))
	}
	if result.InfoCount > 0 {
		parts = append(parts, fmt.Sprintf("%d info", result.InfoCount))
	}
	fmt.Fprintf(w, "  %s\n", strings.Join(parts, ", "))
}

func severityLabel(s storage.ValidationSeverity) string {
	switch s {
	case storage.SeverityError:
		return "ERROR  "
	case storage.SeverityWarning:
		return "WARNING"
	case storage.SeverityInfo:
		return "INFO   "
	default:
		return string(s)
	}
}
