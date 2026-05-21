package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/skrptiq/engine/parse"
)

// catalogueEntry is a lightweight manifest summary for JSON output.
type catalogueEntry struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	DisplayName string `json:"displayName,omitempty"`
	ID          string `json:"id,omitempty"`
}

// Catalogue handles `skrptiq catalogue <dir> [--json]`.
// Iterates subdirectories, calls parse.ReadPackage per skrpt,
// emits a JSON array of manifest summaries. Closes W-HUB-1.
func Catalogue(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: skrptiq catalogue <dir> [--json]")
		return ExitBadArgs
	}
	dir := args[0]

	info, err := os.Stat(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", dir)
		return ExitBadArgs
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	// Sort for deterministic output.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var results []catalogueEntry
	var hadErrors bool

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip hidden directories.
		if len(name) > 0 && name[0] == '.' {
			continue
		}

		subdir := filepath.Join(dir, name)
		// Check for skrptiq.yaml — skip non-skrpt directories silently.
		if _, err := os.Stat(filepath.Join(subdir, "skrptiq.yaml")); os.IsNotExist(err) {
			continue
		}

		pkg, _, err := parse.ReadPackage(subdir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", name, err)
			hadErrors = true
			continue
		}

		results = append(results, catalogueEntry{
			Slug:        name,
			Name:        pkg.Manifest.Name,
			Version:     pkg.Manifest.Version,
			DisplayName: pkg.Manifest.DisplayName,
			ID:          pkg.Manifest.ID,
		})
	}

	outputJSON(os.Stdout, results)

	if hadErrors {
		return ExitFailed
	}
	return ExitOK
}
