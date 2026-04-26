package main

import (
	"fmt"
	"os"

	"github.com/skrptiq/skrptiq-cli/internal/app"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(app.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
