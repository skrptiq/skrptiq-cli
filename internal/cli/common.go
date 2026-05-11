// Package cli implements headless subcommands for direct CLI execution.
// These run without the TUI — plain text stdout, structured JSON via --json.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	eng "github.com/skrptiq/skrptiq-cli/internal/engine"
)

// Exit codes per GH#437 spec.
const (
	ExitOK         = 0
	ExitFailed     = 1 // workflow failed or gate rejected
	ExitBadArgs    = 2 // invalid arguments
	ExitMissingDep = 3 // missing connection/dependency
	ExitTimeout    = 4 // timeout
)

// OpenEngine opens the engine DB, printing an error and returning nil on failure.
func OpenEngine(dbPath string) *eng.App {
	engine, err := eng.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return nil
	}
	return engine
}

// outputJSON writes v as indented JSON to w.
func outputJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}
