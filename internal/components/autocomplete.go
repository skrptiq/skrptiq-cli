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

// Command represents a top-level slash command.
type Command struct {
	Name        string
	Description string
	// Subcommands are shown after selecting this command (stage 2).
	// e.g. /hub → list, search, import, update.
	Subcommands []Subcommand
	// ArgProvider returns completions for the argument after the command.
	// Used for commands without subcommands (e.g. /show, /run).
	// If both Subcommands and ArgProvider are set, Subcommands take priority.
	ArgProvider func(partial string) []Completion
}

// Subcommand is a second-level command under a parent.
type Subcommand struct {
	Name        string
	Description string
	// ArgProvider returns completions for arguments after the subcommand.
	ArgProvider func(partial string) []Completion
}

// HasSubcommands returns true if this command has subcommands.
func (c Command) HasSubcommands() bool {
	return len(c.Subcommands) > 0
}

// AutocompleteSelectMsg is sent when a completion is selected.
type AutocompleteSelectMsg struct {
	// FullText is the complete text to insert (e.g. "/show My Workflow").
	FullText string
	// NeedsMore is true if the autocomplete should continue to the next stage.
	NeedsMore bool
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

// stage tracks what level of completion we're at.
type stage int

const (
	stageCommand stage = iota // top-level: /hub, /run, /help
	stageSub                  // subcommand: list, search, import
	stageArg                  // argument: workflow name, profile name
)

// Autocomplete is a hierarchical command/subcommand/argument popup.
type Autocomplete struct {
	keys      AutocompleteKeyMap
	commands  []Command
	items     []Completion
	cursor    int
	filter    string
	width     int
	maxShow   int
	visible   bool
	stage     stage
	activeCmd *Command    // set during stageSub and stageArg
	activeSub *Subcommand // set during stageArg (if came via subcommand)
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

// Show activates stage 1: top-level command completion.
func (a *Autocomplete) Show(filter string) {
	a.visible = true
	a.stage = stageCommand
	a.activeCmd = nil
	a.activeSub = nil
	a.filter = filter
	a.applyFilter()
	a.cursor = 0
}

// Hide deactivates the popup.
func (a *Autocomplete) Hide() {
	a.visible = false
	a.filter = ""
	a.cursor = 0
	a.stage = stageCommand
	a.activeCmd = nil
	a.activeSub = nil
	a.items = nil
}

// Visible returns whether the popup is shown.
func (a Autocomplete) Visible() bool {
	return a.visible
}

// SetFilter updates the filter and refilters at the current stage.
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

// ShowSubcommands activates stage 2: subcommand completion for a parent command.
func (a *Autocomplete) ShowSubcommands(cmd *Command, filter string) {
	if cmd == nil || !cmd.HasSubcommands() {
		return
	}
	a.visible = true
	a.stage = stageSub
	a.activeCmd = cmd
	a.activeSub = nil
	a.filter = filter
	a.applyFilter()
	a.cursor = 0
}

// ShowArgs activates stage 3: argument completion.
func (a *Autocomplete) ShowArgs(cmd *Command, sub *Subcommand, partial string) {
	var provider func(string) []Completion
	if sub != nil && sub.ArgProvider != nil {
		provider = sub.ArgProvider
	} else if cmd != nil && cmd.ArgProvider != nil {
		provider = cmd.ArgProvider
	}
	if provider == nil {
		return
	}
	a.visible = true
	a.stage = stageArg
	a.activeCmd = cmd
	a.activeSub = sub
	a.filter = partial
	a.items = provider(partial)
	a.cursor = 0
}

// FindCommand looks up a top-level command by name.
func (a *Autocomplete) FindCommand(name string) *Command {
	name = strings.ToLower(strings.TrimSpace(name))
	for i := range a.commands {
		if strings.ToLower(a.commands[i].Name) == name {
			return &a.commands[i]
		}
	}
	return nil
}

// FindSubcommand looks up a subcommand within a command.
func (a *Autocomplete) FindSubcommand(cmd *Command, name string) *Subcommand {
	if cmd == nil {
		return nil
	}
	name = strings.ToLower(strings.TrimSpace(name))
	for i := range cmd.Subcommands {
		if strings.ToLower(cmd.Subcommands[i].Name) == name {
			return &cmd.Subcommands[i]
		}
	}
	return nil
}

// Update handles key events.
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

			switch a.stage {
			case stageCommand:
				cmd := a.FindCommand(selected.Value)
				needsMore := cmd != nil && (cmd.HasSubcommands() || cmd.ArgProvider != nil)
				a.Hide()
				return a, func() tea.Msg {
					return AutocompleteSelectMsg{
						FullText:  selected.Value,
						NeedsMore: needsMore,
					}
				}, true

			case stageSub:
				sub := a.FindSubcommand(a.activeCmd, selected.Value)
				needsMore := sub != nil && sub.ArgProvider != nil
				fullText := a.activeCmd.Name + " " + selected.Value
				a.Hide()
				return a, func() tea.Msg {
					return AutocompleteSelectMsg{
						FullText:  fullText,
						NeedsMore: needsMore,
					}
				}, true

			case stageArg:
				prefix := a.activeCmd.Name
				if a.activeSub != nil {
					prefix += " " + a.activeSub.Name
				}
				fullText := prefix + " " + selected.Value
				a.Hide()
				return a, func() tea.Msg {
					return AutocompleteSelectMsg{
						FullText:  fullText,
						NeedsMore: false,
					}
				}, true
			}

		case key.Matches(msg, a.keys.Dismiss):
			a.Hide()
			return a, func() tea.Msg {
				return AutocompleteDismissMsg{}
			}, true
		}
	}

	return a, nil, false
}

// View renders the popup.
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
	switch a.stage {
	case stageCommand:
		// Filter top-level commands only.
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

	case stageSub:
		// Filter subcommands of the active command.
		a.items = nil
		query := strings.ToLower(a.filter)
		if a.activeCmd != nil {
			for _, sub := range a.activeCmd.Subcommands {
				if query == "" || strings.HasPrefix(strings.ToLower(sub.Name), query) {
					a.items = append(a.items, Completion{
						Value:       sub.Name,
						Description: sub.Description,
					})
				}
			}
		}

	case stageArg:
		var provider func(string) []Completion
		if a.activeSub != nil && a.activeSub.ArgProvider != nil {
			provider = a.activeSub.ArgProvider
		} else if a.activeCmd != nil && a.activeCmd.ArgProvider != nil {
			provider = a.activeCmd.ArgProvider
		}
		if provider != nil {
			a.items = provider(a.filter)
		}
	}
}
