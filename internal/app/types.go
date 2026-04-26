package app

// Command represents a top-level slash command.
type Command struct {
	Name        string
	Description string
	Subcommands []Subcommand
	ArgProvider func(partial string) []Completion
}

// HasSubcommands returns true if this command has subcommands.
func (c Command) HasSubcommands() bool {
	return len(c.Subcommands) > 0
}

// Subcommand is a second-level command under a parent.
type Subcommand struct {
	Name        string
	Description string
	ArgProvider func(partial string) []Completion
}

// Completion represents a single item in a completion list.
type Completion struct {
	Value       string
	Description string
}
