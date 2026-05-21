package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/skrptiq/engine/manifest"
)

// New handles `skrptiq new <name>`.
// Creates a skrpt directory with a valid skrptiq.yaml and empty node-type dirs.
// The argument may be a path (e.g. /tmp/my-skrpt); the manifest name is
// derived from the base directory name.
func New(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: skrptiq new <name>")
		return ExitBadArgs
	}
	name := filepath.Base(args[0])

	if issue := manifest.ValidateName(name); issue != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", issue.Message)
		if issue.Suggestion != "" {
			fmt.Fprintf(os.Stderr, "  Suggestion: %s\n", issue.Suggestion)
		}
		return ExitBadArgs
	}

	dir := args[0]
	if _, err := os.Stat(dir); err == nil {
		fmt.Fprintf(os.Stderr, "Error: directory %q already exists\n", dir)
		return ExitFailed
	}

	id := manifest.GenerateID()
	displayName := toDisplayName(name)

	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	yamlContent := fmt.Sprintf("id: %s\nname: %s\nversion: \"0.1.0\"\ndisplay_name: %q\n", id, name, displayName)
	if err := os.WriteFile(filepath.Join(dir, "skrptiq.yaml"), []byte(yamlContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	for _, sub := range []string{"skills", "prompts", "workflows"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return ExitFailed
		}
	}

	fmt.Printf("Created %s/ with manifest id %s\n", name, id)
	return ExitOK
}

// toDisplayName converts a slug like "my-cool-skrpt" to "My Cool Skrpt".
func toDisplayName(slug string) string {
	words := strings.Split(slug, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
