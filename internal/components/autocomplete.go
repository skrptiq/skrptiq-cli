package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/skrptiq/skrptiq-cli/internal/theme"
)

// Command represents a slash command in the autocomplete list.
type Command struct {
	Name        string
	Description string
}

// AutocompleteSelectMsg is sent when a command is selected.
type AutocompleteSelectMsg struct {
	Command string
}

// AutocompleteDismissMsg is sent when the autocomplete is dismissed.
type AutocompleteDismissMsg struct{}

// AutocompleteKeyMap defines autocomplete key bindings.
type AutocompleteKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Select  key.Binding
	Dismiss key.Binding
}

// DefaultAutocompleteKeyMap returns default autocomplete key bindings.
func DefaultAutocompleteKeyMap() AutocompleteKeyMap {
	return AutocompleteKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", "tab"),
		),
		Dismiss: key.NewBinding(
			key.WithKeys("esc"),
		),
	}
}

// Autocomplete is a filtered command popup.
type Autocomplete struct {
	keys     AutocompleteKeyMap
	commands []Command
	filtered []Command
	cursor   int
	filter   string
	width    int
	maxShow  int
	visible  bool
}

// NewAutocomplete creates a new autocomplete component.
func NewAutocomplete(commands []Command) Autocomplete {
	return Autocomplete{
		keys:     DefaultAutocompleteKeyMap(),
		commands: commands,
		filtered: commands,
		maxShow:  8,
	}
}

// SetWidth sets the rendering width.
func (a *Autocomplete) SetWidth(w int) {
	a.width = w
}

// Show activates the autocomplete popup with an initial filter.
func (a *Autocomplete) Show(filter string) {
	a.visible = true
	a.filter = filter
	a.applyFilter()
	a.cursor = 0
}

// Hide deactivates the autocomplete popup.
func (a *Autocomplete) Hide() {
	a.visible = false
	a.filter = ""
	a.cursor = 0
}

// Visible returns whether the popup is shown.
func (a Autocomplete) Visible() bool {
	return a.visible
}

// SetFilter updates the filter text and refilters the list.
func (a *Autocomplete) SetFilter(filter string) {
	a.filter = filter
	a.applyFilter()
	if a.cursor >= len(a.filtered) {
		a.cursor = len(a.filtered) - 1
	}
	if a.cursor < 0 {
		a.cursor = 0
	}
}

// Update handles key events for the autocomplete.
// Returns the updated model, an optional command, and whether the key was consumed.
func (a Autocomplete) Update(msg tea.Msg) (Autocomplete, tea.Cmd, bool) {
	if !a.visible {
		return a, nil, false
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, a.keys.Up):
			if a.cursor > 0 {
				a.cursor--
			}
			return a, nil, true

		case key.Matches(msg, a.keys.Down):
			if a.cursor < len(a.filtered)-1 {
				a.cursor++
			}
			return a, nil, true

		case key.Matches(msg, a.keys.Select):
			if len(a.filtered) > 0 {
				selected := a.filtered[a.cursor].Name
				a.Hide()
				return a, func() tea.Msg {
					return AutocompleteSelectMsg{Command: selected}
				}, true
			}
			return a, nil, true

		case key.Matches(msg, a.keys.Dismiss):
			a.Hide()
			return a, func() tea.Msg {
				return AutocompleteDismissMsg{}
			}, true
		}
	}

	return a, nil, false
}

// View renders the autocomplete popup.
// The popup renders upward from the input line (like Claude Code).
func (a Autocomplete) View() string {
	if !a.visible || len(a.filtered) == 0 {
		return ""
	}

	// Determine visible window.
	show := a.filtered
	if len(show) > a.maxShow {
		// Scroll so cursor is always visible.
		start := a.cursor - a.maxShow/2
		if start < 0 {
			start = 0
		}
		end := start + a.maxShow
		if end > len(show) {
			end = len(show)
			start = end - a.maxShow
		}
		show = show[start:end]
	}

	nameWidth := 0
	for _, cmd := range show {
		w := lipgloss.Width(cmd.Name)
		if w > nameWidth {
			nameWidth = w
		}
	}
	// Pad name column.
	nameWidth += 2

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#374151")).
		Foreground(lipgloss.Color("#F9FAFB")).
		Bold(true)

	normalNameStyle := lipgloss.NewStyle().
		Foreground(theme.Primary)

	descStyle := lipgloss.NewStyle().
		Foreground(theme.Muted)

	var lines []string
	for i, cmd := range show {
		// Find the actual index in filtered to compare with cursor.
		actualIdx := i
		if len(a.filtered) > a.maxShow {
			start := a.cursor - a.maxShow/2
			if start < 0 {
				start = 0
			}
			if start+a.maxShow > len(a.filtered) {
				start = len(a.filtered) - a.maxShow
			}
			actualIdx = start + i
		}

		name := cmd.Name
		desc := cmd.Description

		// Truncate description to fit.
		maxDesc := a.width - nameWidth - 4
		if maxDesc < 0 {
			maxDesc = 0
		}
		if lipgloss.Width(desc) > maxDesc {
			desc = desc[:maxDesc-1] + "…"
		}

		paddedName := name + strings.Repeat(" ", nameWidth-lipgloss.Width(name))

		if actualIdx == a.cursor {
			line := selectedStyle.Width(a.width - 2).
				Render("  " + paddedName + desc)
			lines = append(lines, line)
		} else {
			line := "  " + normalNameStyle.Render(paddedName) + descStyle.Render(desc)
			lines = append(lines, line)
		}
	}

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Muted).
		Width(a.width - 2)

	return border.Render(strings.Join(lines, "\n"))
}

func (a *Autocomplete) applyFilter() {
	if a.filter == "" || a.filter == "/" {
		a.filtered = a.commands
		return
	}

	query := strings.ToLower(strings.TrimPrefix(a.filter, "/"))
	a.filtered = nil
	for _, cmd := range a.commands {
		name := strings.TrimPrefix(cmd.Name, "/")
		if strings.HasPrefix(strings.ToLower(name), query) {
			a.filtered = append(a.filtered, cmd)
		}
	}
}
