package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/skrptiq/skrptiq-cli/internal/theme"
)

// Header renders the top bar with app name and version.
type Header struct {
	Name    string
	Version string
	Width   int
}

// NewHeader creates a new header component.
func NewHeader(name, version string) Header {
	return Header{Name: name, Version: version}
}

// View renders the header.
func (h Header) View() string {
	title := theme.Title.Render(h.Name)
	ver := theme.Faint.Render(h.Version)

	return lipgloss.NewStyle().
		Width(h.Width).
		Background(lipgloss.Color("#111827")).
		Padding(0, 1).
		Render(title + " " + ver)
}
