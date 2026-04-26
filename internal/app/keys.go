package app

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines global key bindings.
type KeyMap struct {
	ForceQuit key.Binding
	Exit      key.Binding
	Help      key.Binding
	Back      key.Binding
}

// DefaultKeyMap returns the default global key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "force quit"),
		),
		Exit: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d ctrl+d", "exit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
	}
}
