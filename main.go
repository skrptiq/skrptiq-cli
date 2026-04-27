package main

import (
	"fmt"
	"os"

	"github.com/skrptiq/skrptiq-cli/internal/app"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	model := app.New()
	p := tea.NewProgram(&model, tea.WithInputTTY())
	model.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
