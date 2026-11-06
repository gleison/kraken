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

type runMode int

const (
	modeChat runMode = iota
	modePlan
)

type ModelOption func(*Model)

func WithPlanMode() ModelOption {
	return func(model *Model) {
		model.mode = modePlan
	}
}

// Model holds all UI state. It depends on the orchestrator only through
// its public API (ports-and-adapters: TUI is a driver adapter).
type Model struct {
	orch         *orchestrator.Orchestrator
	styles       Styles
	phase        phase
	mode         runMode
	input        textInput
	spinnerFrame int

	goal   string
	plan   *domain.Plan
	events <-chan orchestrator.Event
	cancel context.CancelFunc
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

	// turnRender mirrors session, holding the markdown-rendered result
	// for each turn. Filled when a turn is appended so View doesn't
	// re-run glamour on every keystroke.
	turnRender []string

	// pendingInput is the raw user text for the run currently in
	// flight, captured so it can be saved on the completed Turn.
	pendingInput string

	// toolActivity is the rolling list of tool calls observed during
	// the in-flight run, displayed under the corresponding task in
	// the body so the user can see what the LLM is touching.
	toolActivity []orchestrator.ToolActivity
}

// eventMsg wraps an orchestrator.Event into a Bubble Tea message.
type eventMsg struct {
	event orchestrator.Event
	ok    bool
}

// NewModel builds the initial UI state.
func NewModel(orch *orchestrator.Orchestrator, options ...ModelOption) Model {
	model := Model{
		orch:   orch,
		styles: DefaultStyles(),
		phase:  phaseInput,
		input:  newTextInput(""),
	}
	for _, option := range options {
		option(&model)
	}
	return model
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) startRun(userInput string) tea.Cmd {
	m.pendingInput = userInput
	m.goal = userInput
	m.plan = nil
	m.final = ""
	m.err = nil
	m.phase = phaseRunning
	m.spinnerFrame = 0
	m.scrollOffset = 0
	m.toolActivity = nil
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	if m.mode == modePlan {
		m.goal = m.buildGoal(userInput)
		m.events = m.orch.Plan(ctx, m.goal)
	} else {
		m.events = m.orch.Chat(ctx, userInput, m.session)
	}
	return tea.Batch(tickSpinner(), waitForEvent(m.events))
}

func (m *Model) finishRun() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
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
