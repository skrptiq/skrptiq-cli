package theme

import "github.com/charmbracelet/lipgloss"

// Brand colours.
var (
	Primary   = lipgloss.Color("#7C3AED") // violet
	Secondary = lipgloss.Color("#A78BFA") // light violet
	Accent    = lipgloss.Color("#F59E0B") // amber
)

// Status colours.
var (
	Success = lipgloss.Color("#22C55E") // green
	Warning = lipgloss.Color("#F59E0B") // amber
	Error   = lipgloss.Color("#EF4444") // red
	Muted   = lipgloss.Color("#6B7280") // grey
)

// Diff colours.
var (
	DiffAdd    = lipgloss.Color("#22C55E")
	DiffRemove = lipgloss.Color("#EF4444")
	DiffHeader = lipgloss.Color("#60A5FA") // blue
)

// Text styles.
var (
	Title = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true)

	Subtitle = lipgloss.NewStyle().
			Foreground(Secondary)

	Faint = lipgloss.NewStyle().
		Foreground(Muted)

	Bold = lipgloss.NewStyle().
		Bold(true)

	ErrorText = lipgloss.NewStyle().
			Foreground(Error).
			Bold(true)

	SuccessText = lipgloss.NewStyle().
			Foreground(Success)

	WarningText = lipgloss.NewStyle().
			Foreground(Warning)
)

// UI element styles.
var (
	StatusBar = lipgloss.NewStyle().
			Background(lipgloss.Color("#1F2937")).
			Foreground(lipgloss.Color("#D1D5DB")).
			Padding(0, 1)

	Header = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true).
		Padding(0, 1)

	ActionKey = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true)

	ActionLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB"))

	Prompt = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true)

	TreeBranch = lipgloss.NewStyle().
			Foreground(Muted)

	TreeNode = lipgloss.NewStyle()
)
