package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/skrptiq/skrptiq-cli/internal/app"
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

	model, err := app.New(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	p := tea.NewProgram(&model, tea.WithInputTTY())
	model.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
