package tui

import (
	"log"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gleison/kraken/internal/domain"
	"github.com/gleison/kraken/internal/orchestrator"
)

// Update handles a single Bubble Tea message and returns the new state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		if m.width != msg.Width {
			// Width changed → cached markdown renders are wrong width;
			// drop them and let turnRenderFor rebuild lazily.
			m.turnRender = nil
		}
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(msg.Width - 4)
		return m, nil

	case tea.KeyMsg:
		log.Printf("tui: key type=%d str=%q alt=%t paste=%t phase=%d",
			msg.Type, msg.String(), msg.Alt, msg.Paste, m.phase)
		return m.handleKey(msg)

	case spinnerTickMsg:
		if m.phase == phaseRunning {
			m.spinnerFrame++
			return m, tickSpinner()
		}
		return m, nil

	case eventMsg:
		log.Printf("tui: orch event type=%s ok=%t phase=%d",
			msg.event.Type, msg.ok, m.phase)
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
		// Ctrl+Enter submits. Most terminals send it as Esc+Enter, which
		// Bubble Tea reports as KeyEnter with Alt=true. Plain Enter (no
		// modifier) inserts a newline so pasted content keeps its breaks.
		if msg.Type == tea.KeyEnter {
			if msg.Alt || msg.String() == "ctrl+enter" {
				goal := strings.TrimSpace(m.input.Value())
				if goal == "" {
					return m, nil
				}
				cmd := m.startRun(goal)
				return m, cmd
			}
			m.input.Newline()
			return m, nil
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
		case msg.String() == "c":
			// Continue the conversation: keep session, just open input.
			m.phase = phaseInput
			m.input.Reset()
			m.input.Focus()
			m.plan = nil
			m.final = ""
			m.err = nil
			m.scrollOffset = 0
			return m, nil
		case msg.Type == tea.KeyEnter, msg.String() == "r":
			// Reset: drop the session and start fresh.
			m.phase = phaseInput
			m.input.Reset()
			m.input.Focus()
			m.plan = nil
			m.final = ""
			m.err = nil
			m.scrollOffset = 0
			m.session = nil
			m.turnRender = nil
			return m, nil
		case msg.String() == "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

// handleScroll updates scrollOffset for navigation keys and returns whether
// the key was consumed. The current scroll offset is clamped to the body's
// real maxOffset before any relative movement, so an up-press from the
// auto-scrolled bottom (where scrollOffset is intentionally huge) actually
// moves up one page instead of staying pinned to the bottom.
func (m *Model) handleScroll(msg tea.KeyMsg) bool {
	consumed, action := m.applyScroll(msg)
	if consumed {
		log.Printf("tui: scroll %s offset=%d", action, m.scrollOffset)
	}
	return consumed
}

func (m *Model) applyScroll(msg tea.KeyMsg) (bool, string) {
	switch {
	case msg.Type == tea.KeyCtrlD:
		m.scrollBy(+m.scrollPageStep())
		return true, "pgdown"
	case msg.Type == tea.KeyCtrlU:
		m.scrollBy(-m.scrollPageStep())
		return true, "pgup"
	}
	switch msg.String() {
	case "up", "k":
		m.scrollBy(-1)
		return true, "up"
	case "down", "j":
		m.scrollBy(+1)
		return true, "down"
	case "home", "g":
		m.scrollOffset = 0
		return true, "home"
	case "end", "G":
		m.scrollOffset = 1 << 20 // viewport clamps; max is fine here
		return true, "end"
	}
	return false, ""
}

// scrollBy applies a relative move after first re-grounding scrollOffset
// to the real maxOffset, so movement is always relative to the visible
// position, not to a sentinel left over from auto-scroll.
func (m *Model) scrollBy(delta int) {
	pinned := m.renderPinnedPlan()
	body := m.renderBody()
	bodyHeight := m.bodyHeight(pinned)
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	totalLines := strings.Count(body, "\n") + 1
	maxOffset := totalLines - bodyHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	off := m.scrollOffset
	if off > maxOffset {
		off = maxOffset
	}
	off += delta
	if off < 0 {
		off = 0
	}
	if off > maxOffset {
		off = maxOffset
	}
	m.scrollOffset = off
}

func (m *Model) scrollPageStep() int {
	step := m.bodyHeight("")
	if step < 1 {
		return 1
	}
	return step
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
		m.scrollOffset = 1 << 20
	case orchestrator.EventTaskStarted, orchestrator.EventTaskCompleted:
		// Task pointers are shared with the plan; state already updated.
		// Stick to the bottom so the user sees the latest output.
		m.scrollOffset = 1 << 20
	case orchestrator.EventTaskFailed:
		m.err = ev.Err
		m.scrollOffset = 1 << 20
	case orchestrator.EventRunCompleted:
		m.final = ev.Final
		m.phase = phaseDone
		m.scrollOffset = 1 << 20 // viewport clamps to bottom
		if ev.Plan != nil {
			m.session = append(m.session, domain.Turn{
				UserInput: m.pendingInput,
				Goal:      ev.Plan.Goal,
				Plan:      ev.Plan,
				Result:    ev.Final,
			})
			m.turnRender = append(m.turnRender, renderMarkdown(ev.Final, m.contentWidth()))
		}
	case orchestrator.EventRunFailed:
		m.err = ev.Err
		m.phase = phaseError
		m.scrollOffset = 1 << 20
	}

	return m, waitForEvent(m.events)
}
