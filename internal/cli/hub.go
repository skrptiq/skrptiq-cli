package cli

import (
	"flag"
	"fmt"
	"os"
)

// Hub handles `skrptiq hub <subcommand> [args]`.
func Hub(args []string, dbPath string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: skrptiq hub <search|list> [args]")
		return ExitBadArgs
	}

	sub := args[0]
	subArgs := args[1:]

	engine := OpenEngine(dbPath)
	if engine == nil {
		return ExitFailed
	}
	defer engine.Close()

	switch sub {
	case "list":
		fs := flag.NewFlagSet("hub list", flag.ContinueOnError)
		jsonOut := fs.Bool("json", false, "Output as JSON")
		fs.Parse(subArgs)

		imports, err := engine.HubImports()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return ExitFailed
		}

		if *jsonOut {
			outputJSON(os.Stdout, imports)
			return ExitOK
		}
		if len(imports) == 0 {
			fmt.Println("No skrpts imported from Hub.")
			return ExitOK
		}
		for _, imp := range imports {
			ver := ""
			if imp.Version != nil {
				ver = " v" + *imp.Version
			}
			fmt.Printf("  %s%s\n", imp.Name, ver)
		}
		return ExitOK

	case "search":
		fs := flag.NewFlagSet("hub search", flag.ContinueOnError)
		jsonOut := fs.Bool("json", false, "Output as JSON")
		fs.Parse(subArgs)

		query := fs.Arg(0)
		if query == "" {
			fmt.Fprintln(os.Stderr, "Usage: skrptiq hub search <query> [--json]")
			return ExitBadArgs
		}

		results, err := engine.Hub.Search(query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Hub search error: %v\n", err)
			return ExitFailed
		}

		if *jsonOut {
			outputJSON(os.Stdout, results)
			return ExitOK
		}
		if len(results) == 0 {
			fmt.Printf("No results for: %s\n", query)
			return ExitOK
		}
		fmt.Printf("%d results for %q:\n", len(results), query)
		for _, r := range results {
			line := fmt.Sprintf("  %s", r.Name)
			if r.Category != "" {
				line += " [" + r.Category + "]"
			}
			fmt.Println(line)
			if r.Description != "" {
				fmt.Printf("    %s\n", r.Description)
			}
		}
		return ExitOK

	default:
		fmt.Fprintf(os.Stderr, "Unknown hub subcommand: %s\nValid: list, search\n", sub)
		return ExitBadArgs
	}
}
