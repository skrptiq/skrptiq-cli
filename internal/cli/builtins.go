package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	exec "github.com/skrptiq/engine/execution"
)

// Builtins handles `skrptiq builtins [--json]` (GH#877 / K-057) — emit the engine
// builtin registry, the single authoritative source. The Hub regenerates its
// `builtin-registry.json` from `--json` and `git diff --exit-code`s it, turning a
// hand-authored mirror into a verified cache of the engine (closes the display-
// text-drift gap the membership guard can't catch).
//
// Engine-free: reads the pure `execution.Builtins()` registry — no DB needed.
func Builtins(args []string) int {
	fs := flag.NewFlagSet("builtins", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Emit the registry as JSON (Hub regen source)")
	if err := fs.Parse(args); err != nil {
		return ExitBadArgs
	}

	if *jsonOut {
		return emitBuiltinsJSON()
	}
	return printBuiltinsTable()
}

// builtinEntry is one operation's display metadata. Field order is the emitted
// JSON order (declaration order); op is the map key, not a field (LOCKED shape,
// GH#877). offline/tokenCost are kept so the Hub mirror is fully registry-driven.
type builtinEntry struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Offline     bool   `json:"offline"`
	TokenCost   int    `json:"tokenCost"`
}

// builtinRegistryDoc is the top-level emit: `{ "operations": { "<op>": {…} } }`.
type builtinRegistryDoc struct {
	Operations map[string]builtinEntry `json:"operations"`
}

// emitBuiltinsJSON writes the registry as deterministic JSON. A map[string]… is
// marshaled with alphabetically-sorted keys, so the output is stable for the
// Hub's `git diff --exit-code` regen check.
func emitBuiltinsJSON() int {
	ops := make(map[string]builtinEntry)
	for _, d := range exec.Builtins() {
		ops[d.Name] = builtinEntry{
			Title:       d.Title,
			Description: d.Description,
			Category:    d.Category,
			Offline:     d.Offline,
			TokenCost:   d.TokenCost,
		}
	}
	body, err := json.MarshalIndent(builtinRegistryDoc{Operations: ops}, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}
	fmt.Println(string(body))
	return ExitOK
}

// printBuiltinsTable prints a human-readable listing (op · title · category),
// sorted by operation.
func printBuiltinsTable() int {
	builtins := exec.Builtins()
	sort.Slice(builtins, func(i, j int) bool { return builtins[i].Name < builtins[j].Name })

	fmt.Printf("%d built-in operations (offline, 0 tokens):\n\n", len(builtins))
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "OPERATION\tTITLE\tCATEGORY")
	for _, d := range builtins {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", d.Name, d.Title, d.Category)
	}
	tw.Flush()
	return ExitOK
}
