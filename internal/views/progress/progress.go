package progress

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/skrptiq/skrptiq-cli/internal/components"
	"github.com/skrptiq/skrptiq-cli/internal/theme"
)

// StepStatus represents the state of an execution step.
type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepDone
	StepFailed
)

// Step represents a single execution step.
type Step struct {
	Name      string
	Status    StepStatus
	Detail    string
	StartedAt time.Time
	DoneAt    time.Time
}

// TickMsg advances the simulation by one step.
type TickMsg struct{}

// DoneMsg signals the progress view is finished.
type DoneMsg struct {
	Summary string
}

// Model is the streaming progress view.
type Model struct {
	steps     []Step
	spinner   spinner.Model
	current   int
	done      bool
	summary   string
	width     int
	startedAt time.Time
}

// New creates a new progress view with the given steps.
func New(stepNames []string) Model {
	steps := make([]Step, len(stepNames))
	for i, name := range stepNames {
		steps[i] = Step{Name: name, Status: StepPending}
	}

	return Model{
		steps:     steps,
		spinner:   components.NewSpinner(),
		current:   -1,
		startedAt: time.Now(),
	}
}

// SetSize updates the progress view dimensions.
func (m *Model) SetSize(width, _ int) {
	m.width = width
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.advanceStep())
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg.(type) {
	case TickMsg:
		if m.done {
			return m, nil
		}

		// Complete the current step.
		if m.current >= 0 && m.current < len(m.steps) {
			m.steps[m.current].Status = StepDone
			m.steps[m.current].DoneAt = time.Now()
		}

		// Advance to the next step.
		m.current++
		if m.current >= len(m.steps) {
			m.done = true
			elapsed := time.Since(m.startedAt).Round(time.Millisecond)
			m.summary = fmt.Sprintf("Completed %d steps in %s", len(m.steps), elapsed)
			return m, func() tea.Msg { return DoneMsg{Summary: m.summary} }
		}

		m.steps[m.current].Status = StepRunning
		m.steps[m.current].StartedAt = time.Now()
		return m, m.advanceStep()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	var s string

	titleStyle := theme.Title.Width(m.width)
	s += titleStyle.Render("Running workflow") + "\n\n"

	for _, step := range m.steps {
		s += m.renderStep(step) + "\n"
	}

	if m.done {
		s += "\n" + theme.SuccessText.Render(m.summary)
	}

	return s
}

// Done returns whether all steps have completed.
func (m Model) Done() bool {
	return m.done
}

// Summary returns the completion summary text.
func (m Model) Summary() string {
	return m.summary
}

func (m Model) renderStep(step Step) string {
	var icon string
	var nameStyle lipgloss.Style
	var detail string

	switch step.Status {
	case StepPending:
		icon = theme.Faint.Render("○")
		nameStyle = lipgloss.NewStyle().Foreground(theme.Muted)
	case StepRunning:
		icon = m.spinner.View()
		nameStyle = lipgloss.NewStyle().Bold(true)
		elapsed := time.Since(step.StartedAt).Round(time.Millisecond)
		detail = theme.Faint.Render(fmt.Sprintf(" %s", elapsed))
	case StepDone:
		icon = theme.SuccessText.Render("✓")
		nameStyle = lipgloss.NewStyle()
		elapsed := step.DoneAt.Sub(step.StartedAt).Round(time.Millisecond)
		detail = theme.Faint.Render(fmt.Sprintf(" %s", elapsed))
	case StepFailed:
		icon = theme.ErrorText.Render("✗")
		nameStyle = lipgloss.NewStyle().Foreground(theme.Error)
	}

	line := fmt.Sprintf("  %s %s%s", icon, nameStyle.Render(step.Name), detail)
	if step.Detail != "" {
		line += "\n    " + theme.Faint.Render(step.Detail)
	}
	return line
}

func (m Model) advanceStep() tea.Cmd {
	// Simulate step execution with a delay.
	return tea.Tick(800*time.Millisecond, func(_ time.Time) tea.Msg {
		return TickMsg{}
	})
}
