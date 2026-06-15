package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/skrptiq/engine/manifest"
)

// MigrateIdentity handles `skrptiq migrate-identity <dir>`.
// One-time legacy migration helper — rewrites manifest.id to canonical form.
func MigrateIdentity(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: skrptiq migrate-identity <dir>")
		return ExitBadArgs
	}
	dir := args[0]

	yamlPath := filepath.Join(dir, "skrptiq.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	// Decode the manifest through the engine — the single YAML parse
	// authority (GH#530 Phase 3b). The manifest-only reader is the
	// minimal correct dependency here: migrate-identity runs on legacy /
	// pre-canonical bundles, so it must surface the raw id string (any
	// validation findings are returned as issues, which we deliberately
	// ignore) without the full-package parse rejecting the very drift it
	// exists to fix. Raw bytes are retained only for the write-back.
	m, _, err := manifest.ParseManifestYAML(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid skrptiq.yaml: %v\n", err)
		return ExitFailed
	}

	currentID := m.ID
	if currentID == "" {
		fmt.Println("No manifest.id present — nothing to migrate.")
		return ExitOK
	}

	canonical, issue, err := manifest.MigrateIdentity(currentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	if canonical == currentID {
		fmt.Println("manifest.id is already in canonical form.")
		return ExitOK
	}

	// Rewrite the YAML file.
	updated := replaceYAMLField(string(data), "id", canonical)
	if err := os.WriteFile(yamlPath, []byte(updated), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	if issue != nil {
		fmt.Printf("Migrated: %s\n", issue.Message)
	}
	fmt.Printf("manifest.id rewritten: %s → %s\n", currentID, canonical)
	return ExitOK
}
