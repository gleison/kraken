package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
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

	case spinner.TickMsg:
		if m.phase == phaseRunning {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case eventMsg:
		return m.handleEvent(msg)
	}

	if m.phase == phaseInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	switch m.phase {
	case phaseInput:
		if msg.Type == tea.KeyEnter && !msg.Alt {
			goal := strings.TrimSpace(m.input.Value())
			if goal == "" {
				return m, nil
			}
			cmd := m.startRun(goal)
			return m, cmd
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	case phaseDone, phaseError:
		if msg.Type == tea.KeyEnter || msg.String() == "r" {
			m.phase = phaseInput
			m.input.Reset()
			m.input.Focus()
			m.plan = nil
			m.final = ""
			m.err = nil
			return m, textarea.Blink
		}
		if msg.String() == "q" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) handleEvent(msg eventMsg) (tea.Model, tea.Cmd) {
	if !msg.ok {
		// Channel closed without terminal event: treat as done if we have a plan.
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
