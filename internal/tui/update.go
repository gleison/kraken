package tui

import (
	"log"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/gleison/kraken/internal/domain"
	"github.com/gleison/kraken/internal/orchestrator"
)

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

	case tea.KeyPressMsg:
		log.Printf("tui: key code=%d str=%q mod=%d phase=%d",
			msg.Code, msg.String(), msg.Mod, m.phase)
		return m.handleKey(msg)

	case tea.PasteMsg:
		if m.phase == phaseInput || (m.phase == phaseRunning && m.mode == modeChat) {
			m.input.Paste(msg.Content)
		}
		return m, nil

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

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		m.finishRun()
		return m, tea.Quit
	}

	switch m.phase {
	case phaseInput:
		if isSubmitKey(msg) {
			return m.submitInput()
		}
		if msg.Code == tea.KeyEnter {
			m.input.Newline()
			return m, nil
		}
		if msg.String() == "ctrl+u" || msg.String() == "ctrl+d" {
			m.handleScroll(msg)
			return m, nil
		}
		m.input.Update(msg)
		return m, nil

	case phaseRunning, phaseDone, phaseError:
		if m.phase == phaseRunning && msg.Code == tea.KeyEsc {
			m.finishRun()
			return m, nil
		}
		if m.phase == phaseRunning && m.mode == modeChat {
			if msg.Code == tea.KeyEnter {
				m.input.Newline()
				return m, nil
			}
			if msg.String() == "ctrl+u" || msg.String() == "ctrl+d" {
				m.handleScroll(msg)
				return m, nil
			}
			if !isSubmitKey(msg) {
				m.input.Update(msg)
			}
			return m, nil
		}
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
		case msg.Code == tea.KeyEnter, msg.String() == "r":
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

func isSubmitKey(msg tea.KeyPressMsg) bool {
	return msg.String() == "ctrl+enter"
}

func (m Model) submitInput() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.input.Value())
	if input == "" {
		return m, nil
	}

	for _, command := range []struct {
		name string
		mode runMode
	}{
		{name: "/plan", mode: modePlan},
		{name: "/chat", mode: modeChat},
	} {
		if input == command.name {
			m.mode = command.mode
			m.input.Reset()
			return m, nil
		}
		prefix := command.name + " "
		if strings.HasPrefix(input, prefix) {
			m.mode = command.mode
			input = strings.TrimSpace(strings.TrimPrefix(input, prefix))
			if input == "" {
				m.input.Reset()
				return m, nil
			}
			break
		}
	}

	cmd := m.startRun(input)
	return m, cmd
}

// handleScroll updates scrollOffset for navigation keys and returns whether
// the key was consumed. The current scroll offset is clamped to the body's
// real maxOffset before any relative movement, so an up-press from the
// auto-scrolled bottom (where scrollOffset is intentionally huge) actually
// moves up one page instead of staying pinned to the bottom.
func (m *Model) handleScroll(msg tea.KeyPressMsg) bool {
	consumed, action := m.applyScroll(msg)
	if consumed {
		log.Printf("tui: scroll %s offset=%d", action, m.scrollOffset)
	}
	return consumed
}

func (m *Model) applyScroll(msg tea.KeyPressMsg) (bool, string) {
	switch {
	case msg.String() == "ctrl+d":
		m.scrollBy(+m.scrollPageStep())
		return true, "pgdown"
	case msg.String() == "ctrl+u":
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
			if m.mode == modeChat {
				m.phase = phaseInput
			} else {
				m.phase = phaseDone
			}
		}
		return m, nil
	}

	ev := msg.event
	switch ev.Type {
	case orchestrator.EventPlanning, orchestrator.EventChatStarted:
	case orchestrator.EventPlanReady:
		m.plan = ev.Plan
		m.scrollOffset = 1 << 20
	case orchestrator.EventTaskStarted, orchestrator.EventTaskCompleted:
		m.updateTask(ev.Task)
		m.scrollOffset = 1 << 20
	case orchestrator.EventToolCall:
		if ev.Tool != nil {
			m.toolActivity = append(m.toolActivity, *ev.Tool)
		}
		if m.mode == modeChat && m.plan == nil {
			m.scrollOffset = m.pendingChatTurnOffset()
		} else {
			m.scrollOffset = 1 << 20
		}
	case orchestrator.EventTaskFailed:
		m.updateTask(ev.Task)
		m.err = ev.Err
		m.scrollOffset = 1 << 20
	case orchestrator.EventRunCompleted:
		m.finishRun()
		m.plan = ev.Plan
		m.final = ev.Final
		m.phase = phaseDone
		m.scrollOffset = 1 << 20 // viewport clamps to bottom
		goal := m.goal
		if ev.Plan != nil {
			goal = ev.Plan.Goal
		}
		m.session = append(m.session, domain.Turn{
			UserInput: m.pendingInput,
			Goal:      goal,
			Plan:      ev.Plan,
			Result:    ev.Final,
		})
		m.turnRender = append(m.turnRender, renderMarkdown(ev.Final, m.contentWidth()))
		if m.mode == modeChat && ev.Plan == nil {
			m.phase = phaseInput
			m.input.Focus()
			m.plan = nil
			m.final = ""
			m.scrollOffset = m.latestChatTurnOffset()
		}
	case orchestrator.EventRunFailed:
		m.finishRun()
		m.err = ev.Err
		if m.mode == modeChat {
			m.phase = phaseInput
			m.input.Focus()
		} else {
			m.phase = phaseError
		}
		m.scrollOffset = 1 << 20
	}

	return m, waitForEvent(m.events)
}

func (m *Model) updateTask(updated *domain.Task) {
	if m.plan == nil || updated == nil {
		return
	}
	for i, task := range m.plan.Tasks {
		if task.ID == updated.ID {
			m.plan.Tasks[i] = updated
			return
		}
	}
}
