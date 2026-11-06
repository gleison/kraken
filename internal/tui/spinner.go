package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const spinnerInterval = 100 * time.Millisecond

type spinnerTickMsg time.Time

// tickSpinner returns a command that emits a tick after spinnerInterval.
// We avoid bubbles/spinner so the dep tree stays minimal.
func tickSpinner() tea.Cmd {
	return tea.Tick(spinnerInterval, func(t time.Time) tea.Msg {
		return spinnerTickMsg(t)
	})
}

func spinnerView(frame int) string {
	return spinnerFrames[frame%len(spinnerFrames)]
}
