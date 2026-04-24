// Package tui implements the terminal UI (Bubble Tea) for the orchestrator.
package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
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
	orch    *orchestrator.Orchestrator
	styles  Styles
	phase   phase
	input   textarea.Model
	spinner spinner.Model

	goal   string
	plan   *domain.Plan
	events <-chan orchestrator.Event
	final  string
	err    error

	width  int
	height int
}

// eventMsg wraps an orchestrator.Event into a Bubble Tea message.
type eventMsg struct {
	event orchestrator.Event
	ok    bool
}

// NewModel builds the initial UI state.
func NewModel(orch *orchestrator.Orchestrator) Model {
	ta := textarea.New()
	ta.Placeholder = "Descreva uma tarefa complexa... (Enter para executar, Shift+Enter para nova linha)"
	ta.Focus()
	ta.SetHeight(4)
	ta.ShowLineNumbers = false
	ta.CharLimit = 2000

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return Model{
		orch:    orch,
		styles:  DefaultStyles(),
		phase:   phaseInput,
		input:   ta,
		spinner: sp,
	}
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// startRun kicks off the orchestrator and returns a command that reads the
// first event from the channel.
func (m *Model) startRun(goal string) tea.Cmd {
	m.goal = goal
	m.plan = nil
	m.final = ""
	m.err = nil
	m.phase = phaseRunning
	m.events = m.orch.Run(context.Background(), goal)
	return tea.Batch(m.spinner.Tick, waitForEvent(m.events))
}

// waitForEvent returns a command that receives one event from the channel.
func waitForEvent(ch <-chan orchestrator.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		return eventMsg{event: ev, ok: ok}
	}
}
