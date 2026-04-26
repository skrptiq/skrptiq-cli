package components

import (
	"strings"

	"github.com/skrptiq/skrptiq-cli/internal/theme"
)

// StatusBar renders the bottom status bar.
type StatusBar struct {
	Profile   string
	Workspace string
	MCP       []MCPStatus
	Width     int
}

// MCPStatus represents a connected MCP server.
type MCPStatus struct {
	Name      string
	Connected bool
}

// NewStatusBar creates a new status bar with defaults.
func NewStatusBar() StatusBar {
	return StatusBar{
		Profile:   "Default",
		Workspace: "~",
		MCP: []MCPStatus{
			{Name: "GitHub", Connected: false},
			{Name: "Filesystem", Connected: false},
		},
	}
}

// View renders the status bar.
func (s StatusBar) View() string {
	profile := theme.Faint.Render("Profile: ") + s.Profile
	workspace := theme.Faint.Render("Workspace: ") + s.Workspace

	var mcpParts []string
	for _, m := range s.MCP {
		indicator := theme.ErrorText.Render("●")
		if m.Connected {
			indicator = theme.SuccessText.Render("●")
		}
		mcpParts = append(mcpParts, m.Name+" "+indicator)
	}
	mcp := theme.Faint.Render("MCP: ") + strings.Join(mcpParts, " ")

	content := profile + "  " + workspace + "  " + mcp

	return theme.StatusBar.Width(s.Width).Render(content)
}
