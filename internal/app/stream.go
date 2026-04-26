package app

import (
	tea "github.com/charmbracelet/bubbletea"

	exec "github.com/skrptiq/engine/execution"
)

// StreamChunkMsg carries a chunk of streaming LLM output.
type StreamChunkMsg struct {
	Text string
}

// StreamDoneMsg signals the LLM stream has ended.
type StreamDoneMsg struct {
	FullOutput string
	Provider   string
	Model      string
	InputTokens  int
	OutputTokens int
}

// StreamErrorMsg carries an error from a stream or execution.
type StreamErrorMsg struct {
	Err error
}

// ProgressEventMsg wraps a workflow execution progress event.
type ProgressEventMsg struct {
	exec.ProgressEvent
}

// ExecutionDoneMsg signals workflow execution has completed.
type ExecutionDoneMsg struct {
	ExecutionID string
	Status      string // "completed" or "failed"
	Error       string
}

// streamChannel is a channel that carries tea.Msg values from goroutines.
type streamChannel <-chan tea.Msg

// readStream returns a tea.Cmd that reads one message from the channel.
// Call this repeatedly — each invocation blocks until the next message arrives.
// When the channel is closed, returns nil (no more messages).
func readStream(ch streamChannel) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}
