package main

import (
	"fmt"
	"os"

	"github.com/skrptiq/skrptiq-cli/internal/app"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Use a shared printer reference that gets wired after program creation.
	printer := &app.Printer{}
	model := app.NewWithPrinter(printer)
	p := tea.NewProgram(model)
	printer.Program = p
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
