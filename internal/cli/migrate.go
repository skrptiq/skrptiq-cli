package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/skrptiq/engine/manifest"
	"gopkg.in/yaml.v3"
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

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid skrptiq.yaml: %v\n", err)
		return ExitFailed
	}

	idVal, ok := raw["id"]
	if !ok || idVal == nil {
		fmt.Println("No manifest.id present — nothing to migrate.")
		return ExitOK
	}

	currentID, ok := idVal.(string)
	if !ok || currentID == "" {
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
