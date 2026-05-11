package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/skrptiq/skrptiq-cli/internal/app"
	"github.com/skrptiq/skrptiq-cli/internal/cli"
	"github.com/skrptiq/skrptiq-cli/internal/scan"
	"github.com/skrptiq/skrptiq-cli/internal/version"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	dbPath := flag.String("db-path", "", "Path to SQLite database (overrides default)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("skrptiq " + version.Full())
		return
	}

	// Subcommand dispatch — headless commands run before TUI.
	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "scan":
			scanFlags := flag.NewFlagSet("scan", flag.ExitOnError)
			jsonOutput := scanFlags.Bool("json", false, "Output as JSON")
			scanFlags.Parse(args[1:])
			scanPath := scanFlags.Arg(0)
			if scanPath == "" {
				fmt.Fprintln(os.Stderr, "Usage: skrptiq scan [--json] <path>")
				os.Exit(2)
			}
			os.Exit(scan.Run(scanPath, *jsonOutput))
		case "run":
			os.Exit(cli.Run(args[1:], *dbPath))
		case "list":
			os.Exit(cli.List(args[1:], *dbPath))
		case "show":
			os.Exit(cli.Show(args[1:], *dbPath))
		case "hub":
			os.Exit(cli.Hub(args[1:], *dbPath))
		case "version":
			fmt.Println("skrptiq " + version.Full())
			return
		}
	}

	model, err := app.New(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	p := tea.NewProgram(&model, tea.WithInputTTY())
	model.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: terminal session failed\n\n"+
			"  This usually means the terminal does not support interactive input.\n"+
			"  Try: run from a standard terminal emulator (not a pipe or script).\n\n"+
			"  Detail: %v\n", err)
		os.Exit(1)
	}
}
