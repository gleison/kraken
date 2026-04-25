package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gleison/kraken/internal/orchestrator"
)

// Update handles a single Bubble Tea message and returns the new state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(msg.Width - 4)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinnerTickMsg:
		if m.phase == phaseRunning {
			m.spinnerFrame++
			return m, tickSpinner()
		}
		return m, nil

	case eventMsg:
		return m.handleEvent(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	switch m.phase {
	case phaseInput:
		// Enter always inserts a newline so that pasted content never
		// loses its line breaks (some terminals deliver paste line breaks
		// as bare CR events indistinguishable from a typed Enter).
		// Submit is an explicit, non-ambiguous chord: Ctrl+D.
		if msg.Type == tea.KeyEnter {
			m.input.Newline()
			return m, nil
		}
		if msg.Type == tea.KeyCtrlD {
			goal := strings.TrimSpace(m.input.Value())
			if goal == "" {
				return m, nil
			}
			cmd := m.startRun(goal)
			return m, cmd
		}
		m.input.Update(msg)
		return m, nil

	case phaseRunning, phaseDone, phaseError:
		if m.handleScroll(msg) {
			return m, nil
		}
		if m.phase == phaseRunning {
			return m, nil
		}
		switch {
		case msg.Type == tea.KeyEnter, msg.String() == "r":
			m.phase = phaseInput
			m.input.Reset()
			m.input.Focus()
			m.plan = nil
			m.final = ""
			m.err = nil
			m.scrollOffset = 0
			return m, nil
		case msg.String() == "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

// handleScroll updates scrollOffset for navigation keys and returns whether
// the key was consumed. Clamping to the valid range happens in viewport().
func (m *Model) handleScroll(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "up", "k":
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
		return true
	case "down", "j":
		m.scrollOffset++
		return true
	case "pgup":
		step := m.bodyHeight()
		if step < 1 {
			step = 1
		}
		m.scrollOffset -= step
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
		return true
	case "pgdown":
		step := m.bodyHeight()
		if step < 1 {
			step = 1
		}
		m.scrollOffset += step
		return true
	case "home", "g":
		m.scrollOffset = 0
		return true
	case "end", "G":
		m.scrollOffset = 1 << 20 // viewport() will clamp to maxOffset
		return true
	}
	return false
}

func (m Model) handleEvent(msg eventMsg) (tea.Model, tea.Cmd) {
	if !msg.ok {
		if m.err == nil && m.phase == phaseRunning {
			m.phase = phaseDone
		}
		return m, nil
	}

	ev := msg.event
	switch ev.Type {
	case orchestrator.EventPlanning:
		// spinner already ticking
	case orchestrator.EventPlanReady:
		m.plan = ev.Plan
	case orchestrator.EventTaskStarted, orchestrator.EventTaskCompleted:
		// Task pointers are shared with the plan; state already updated.
	case orchestrator.EventTaskFailed:
		m.err = ev.Err
	case orchestrator.EventRunCompleted:
		m.final = ev.Final
		m.phase = phaseDone
	case orchestrator.EventRunFailed:
		m.err = ev.Err
		m.phase = phaseError
	}

	return m, waitForEvent(m.events)
}
