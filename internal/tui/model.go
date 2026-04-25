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

	// session accumulates completed turns. While it is non-empty the
	// next submission is treated as a refinement of the last turn.
	session []domain.Turn

	// pendingInput is the raw user text for the run currently in
	// flight, captured so it can be saved on the completed Turn.
	pendingInput string
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

// startRun kicks off the orchestrator with the given user text. When the
// session already has prior turns, the goal is wrapped with that context
// so the planner can treat the call as a refinement.
func (m *Model) startRun(userInput string) tea.Cmd {
	m.pendingInput = userInput
	m.goal = m.buildGoal(userInput)
	m.plan = nil
	m.final = ""
	m.err = nil
	m.phase = phaseRunning
	m.spinnerFrame = 0
	m.scrollOffset = 0
	m.events = m.orch.Run(context.Background(), m.goal)
	return tea.Batch(tickSpinner(), waitForEvent(m.events))
}

// buildGoal produces the goal string sent to the orchestrator. The first
// submission passes through unchanged; subsequent ones include the prior
// turn's user input and result so the planner knows it is refining work.
func (m Model) buildGoal(userInput string) string {
	if len(m.session) == 0 {
		return userInput
	}
	last := m.session[len(m.session)-1]
	return "Você está continuando uma tarefa anterior. " +
		"Use o resultado anterior como ponto de partida e aplique o novo pedido do usuário, " +
		"preservando tudo que já estava correto.\n\n" +
		"PEDIDO ANTERIOR DO USUÁRIO:\n" + last.UserInput + "\n\n" +
		"RESULTADO ANTERIOR:\n" + last.Result + "\n\n" +
		"NOVO PEDIDO DO USUÁRIO:\n" + userInput
}

// waitForEvent returns a command that receives one event from the channel.
func waitForEvent(ch <-chan orchestrator.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		return eventMsg{event: ev, ok: ok}
	}
}
