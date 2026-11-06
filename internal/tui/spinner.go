package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const spinnerInterval = 100 * time.Millisecond

// spinnerTickMsg advances the spinner frame.
type spinnerTickMsg time.Time

// tickSpinner returns a command that emits a tick after spinnerInterval.
// We avoid bubbles/spinner so the dep tree stays minimal.
func tickSpinner() tea.Cmd {
	return tea.Tick(spinnerInterval, func(t time.Time) tea.Msg {
		return spinnerTickMsg(t)
	})
}

// spinnerView renders the frame for the given index.
func spinnerView(frame int) string {
	return spinnerFrames[frame%len(spinnerFrames)]
}
