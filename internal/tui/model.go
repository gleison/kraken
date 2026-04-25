// Package tui implements the terminal UI (Bubble Tea) for the orchestrator.
package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gleison/kraken/internal/domain"
	"github.com/gleison/kraken/internal/orchestrator"
)

// phase enumerates the screens of the TUI.
type phase int

const (
	phaseInput phase = iota
	phaseRunning
	phaseDone
	phaseError
)

// Model holds all UI state. It depends on the orchestrator only through
// its public API (ports-and-adapters: TUI is a driver adapter).
type Model struct {
	orch         *orchestrator.Orchestrator
	styles       Styles
	phase        phase
	input        textInput
	spinnerFrame int

	goal   string
	plan   *domain.Plan
	events <-chan orchestrator.Event
	final  string
	err    error

	width  int
	height int

	// scrollOffset is the number of body lines hidden above the viewport.
	// Used for scrolling through long plans/results.
	scrollOffset int
}

// eventMsg wraps an orchestrator.Event into a Bubble Tea message.
type eventMsg struct {
	event orchestrator.Event
	ok    bool
}

// NewModel builds the initial UI state.
func NewModel(orch *orchestrator.Orchestrator) Model {
	return Model{
		orch:   orch,
		styles: DefaultStyles(),
		phase:  phaseInput,
		input:  newTextInput(""),
	}
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// startRun kicks off the orchestrator and returns a command that reads the
// first event from the channel plus a spinner tick.
func (m *Model) startRun(goal string) tea.Cmd {
	m.goal = goal
	m.plan = nil
	m.final = ""
	m.err = nil
	m.phase = phaseRunning
	m.spinnerFrame = 0
	m.scrollOffset = 0
	m.events = m.orch.Run(context.Background(), goal)
	return tea.Batch(tickSpinner(), waitForEvent(m.events))
}

// waitForEvent returns a command that receives one event from the channel.
func waitForEvent(ch <-chan orchestrator.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		return eventMsg{event: ev, ok: ok}
	}
}
