package components

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/skrptiq/skrptiq-cli/internal/theme"
)

// NewSpinner creates a spinner with the brand styling.
func NewSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.Primary)
	return s
}
