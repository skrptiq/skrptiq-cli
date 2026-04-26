package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/skrptiq/skrptiq-cli/internal/theme"
)

// Completion represents a single item in the autocomplete list.
type Completion struct {
	Value       string
	Description string
}

// Command represents a slash command with optional argument completion.
type Command struct {
	Name        string
	Description string
	// ArgProvider returns completions for the argument after the command.
	// Called with the partial argument text typed so far.
	// If nil, no argument completion is offered.
	ArgProvider func(partial string) []Completion
}

// AutocompleteSelectMsg is sent when a completion is selected.
type AutocompleteSelectMsg struct {
	// FullText is the complete text to insert (e.g. "/show My Workflow").
	FullText string
	// IsCommand is true if a command was selected (stage 1),
	// false if an argument was selected (stage 2).
	IsCommand bool
	// HasArgs indicates the selected command has an ArgProvider.
	HasArgs bool
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

// stage tracks whether we're completing commands or arguments.
type stage int

const (
	stageCommand stage = iota
	stageArg
)

// Autocomplete is a filtered command/argument popup.
type Autocomplete struct {
	keys     AutocompleteKeyMap
	commands []Command
	items    []Completion
	cursor   int
	filter   string
	width    int
	maxShow  int
	visible  bool
	stage    stage
	// activeCmd is the command selected in stage 1, used for stage 2 arg completion.
	activeCmd *Command
}

// NewAutocomplete creates a new autocomplete component.
func NewAutocomplete(commands []Command) Autocomplete {
	return Autocomplete{
		keys:    DefaultAutocompleteKeyMap(),
		commands: commands,
		maxShow: 8,
	}
}

// SetWidth sets the rendering width.
func (a *Autocomplete) SetWidth(w int) {
	a.width = w
}

// Show activates the autocomplete popup with an initial filter.
func (a *Autocomplete) Show(filter string) {
	a.visible = true
	a.stage = stageCommand
	a.activeCmd = nil
	a.filter = filter
	a.applyFilter()
	a.cursor = 0
}

// ShowArgs activates argument completion for a specific command.
func (a *Autocomplete) ShowArgs(cmd *Command, partial string) {
	if cmd == nil || cmd.ArgProvider == nil {
		return
	}
	a.visible = true
	a.stage = stageArg
	a.activeCmd = cmd
	a.filter = partial
	completions := cmd.ArgProvider(partial)
	a.items = completions
	a.cursor = 0
}

// Hide deactivates the autocomplete popup.
func (a *Autocomplete) Hide() {
	a.visible = false
	a.filter = ""
	a.cursor = 0
	a.stage = stageCommand
	a.activeCmd = nil
	a.items = nil
}

// Visible returns whether the popup is shown.
func (a Autocomplete) Visible() bool {
	return a.visible
}

// SetFilter updates the filter text and refilters the list.
func (a *Autocomplete) SetFilter(filter string) {
	a.filter = filter
	a.applyFilter()
	if a.cursor >= len(a.items) {
		a.cursor = len(a.items) - 1
	}
	if a.cursor < 0 {
		a.cursor = 0
	}
}

// FindCommand looks up a command by name.
func (a *Autocomplete) FindCommand(name string) *Command {
	name = strings.ToLower(strings.TrimSpace(name))
	for i := range a.commands {
		if strings.ToLower(a.commands[i].Name) == name {
			return &a.commands[i]
		}
	}
	return nil
}

// Update handles key events for the autocomplete.
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
			if a.cursor < len(a.items)-1 {
				a.cursor++
			}
			return a, nil, true

		case key.Matches(msg, a.keys.Select):
			if len(a.items) == 0 {
				return a, nil, true
			}
			selected := a.items[a.cursor]

			if a.stage == stageCommand {
				// Check if this command has arg completion.
				cmd := a.FindCommand(selected.Value)
				hasArgs := cmd != nil && cmd.ArgProvider != nil
				a.Hide()
				return a, func() tea.Msg {
					return AutocompleteSelectMsg{
						FullText:  selected.Value,
						IsCommand: true,
						HasArgs:   hasArgs,
					}
				}, true
			}

			// Stage 2: argument selected — build full text.
			fullText := a.activeCmd.Name + " " + selected.Value
			a.Hide()
			return a, func() tea.Msg {
				return AutocompleteSelectMsg{
					FullText:  fullText,
					IsCommand: false,
					HasArgs:   false,
				}
			}, true

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
func (a Autocomplete) View() string {
	if !a.visible || len(a.items) == 0 {
		return ""
	}

	show := a.items
	startOffset := 0
	if len(show) > a.maxShow {
		start := a.cursor - a.maxShow/2
		if start < 0 {
			start = 0
		}
		end := start + a.maxShow
		if end > len(show) {
			end = len(show)
			start = end - a.maxShow
		}
		startOffset = start
		show = show[start:end]
	}

	nameWidth := 0
	for _, item := range show {
		w := lipgloss.Width(item.Value)
		if w > nameWidth {
			nameWidth = w
		}
	}
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
	for i, item := range show {
		actualIdx := startOffset + i

		name := item.Value
		desc := item.Description

		maxDesc := a.width - nameWidth - 4
		if maxDesc < 0 {
			maxDesc = 0
		}
		if maxDesc > 0 && lipgloss.Width(desc) > maxDesc {
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
	if a.stage == stageArg && a.activeCmd != nil && a.activeCmd.ArgProvider != nil {
		a.items = a.activeCmd.ArgProvider(a.filter)
		return
	}

	// Stage 1: filter commands.
	a.items = nil
	query := strings.ToLower(strings.TrimPrefix(a.filter, "/"))
	for _, cmd := range a.commands {
		name := strings.TrimPrefix(cmd.Name, "/")
		if query == "" || strings.HasPrefix(strings.ToLower(name), query) {
			a.items = append(a.items, Completion{
				Value:       cmd.Name,
				Description: cmd.Description,
			})
		}
	}
}
