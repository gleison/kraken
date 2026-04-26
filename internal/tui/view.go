package tui

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/gleison/kraken/internal/domain"
)

// viewSlowThreshold is the duration above which a single render is logged
// as a warning. Bubble Tea calls View after every Update, so a slow View
// freezes the entire UI (including Ctrl+C).
const viewSlowThreshold = 200 * time.Millisecond

// View renders the current state. The header and (optional) pinned plan
// stay at the top; the body in between is fitted to the remaining
// vertical space and can be scrolled with the keyboard.
func (m Model) View() string {
	start := time.Now()
	defer func() {
		if d := time.Since(start); d > viewSlowThreshold {
			log.Printf("tui: View slow (%s, phase=%d, session=%d, final_bytes=%d)",
				d, m.phase, len(m.session), len(m.final))
		}
	}()

	header := m.styles.Title.Render("🐙 kraken") + "  " +
		m.styles.Subtitle.Render("orquestrador de tarefas LLM")

	pinned := m.renderPinnedPlan()
	body := m.renderBody()
	footer := m.footer()

	out := header + "\n\n"
	if pinned != "" {
		out += pinned + "\n"
	}
	out += m.viewport(body, pinned) + "\n" + m.scrollHint(body, pinned) + footer
	return out
}

// renderBody returns the full (unscrolled) body for the current phase.
func (m Model) renderBody() string {
	switch m.phase {
	case phaseInput:
		return m.viewInput()
	case phaseRunning:
		return m.viewRunning()
	case phaseDone:
		return m.viewDone()
	case phaseError:
		return m.viewError()
	}
	return ""
}

// viewport slices body lines according to scrollOffset and the available
// vertical space, so long plans/results don't push the footer offscreen.
// The pinned region sits above the viewport and steals from its height.
func (m Model) viewport(body, pinned string) string {
	bodyHeight := m.bodyHeight(pinned)
	if bodyHeight <= 0 {
		return body
	}
	lines := strings.Split(body, "\n")
	if len(lines) <= bodyHeight {
		return body
	}
	maxOffset := len(lines) - bodyHeight
	off := m.scrollOffset
	if off < 0 {
		off = 0
	}
	if off > maxOffset {
		off = maxOffset
	}
	return strings.Join(lines[off:off+bodyHeight], "\n")
}

// bodyHeight is the number of body lines the terminal can show given the
// space already taken by header, footer, and the pinned plan.
func (m Model) bodyHeight(pinned string) int {
	if m.height <= 0 {
		return 0
	}
	pinnedLines := 0
	if pinned != "" {
		pinnedLines = strings.Count(pinned, "\n") + 2 // pinned + separator line
	}
	reserved := 5 + pinnedLines
	if m.height <= reserved {
		return 1
	}
	return m.height - reserved
}

// scrollHint shows arrows when there is content above or below the viewport.
// Reuses the already-rendered body so View only renders once per frame.
func (m Model) scrollHint(body, pinned string) string {
	bodyHeight := m.bodyHeight(pinned)
	if bodyHeight <= 0 {
		return ""
	}
	total := strings.Count(body, "\n") + 1
	if total <= bodyHeight {
		return ""
	}
	var indicators []string
	if m.scrollOffset > 0 {
		indicators = append(indicators, "↑ mais acima")
	}
	if m.scrollOffset+bodyHeight < total {
		indicators = append(indicators, "↓ mais abaixo")
	}
	if len(indicators) == 0 {
		return ""
	}
	return m.styles.Help.Render(strings.Join(indicators, "  ")) + "\n"
}

func (m Model) viewInput() string {
	var b strings.Builder
	if len(m.session) > 0 {
		last := m.session[len(m.session)-1]
		banner := "↺ refinando: " + truncateInline(last.UserInput, 80)
		b.WriteString(m.styles.Subtitle.Render(banner))
		b.WriteString("\n")
	}
	b.WriteString(m.styles.Label.Render("Objetivo"))
	b.WriteString("\n")
	b.WriteString(m.inputBox().Render(m.input.View()))
	return b.String()
}

// truncateInline shortens s to fit on a single line, replacing newlines
// with spaces. Used for the refinement banner.
func truncateInline(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// inputBox returns a Box style sized to the current terminal width so the
// editor gets a full-width writing surface that grows in height as the user
// adds (or wraps) lines.
func (m Model) inputBox() lipgloss.Style {
	box := m.styles.Box
	if m.width > 6 {
		box = box.Width(m.width - 4) // 2 border + 2 padding
	}
	return box
}

func (m Model) viewRunning() string {
	var b strings.Builder
	b.WriteString(m.renderHistory())

	if m.plan == nil {
		fmt.Fprintf(&b, "%s %s\n", spinnerView(m.spinnerFrame), m.phaseHeadline())
		return b.String()
	}

	if m.pendingInput != "" {
		b.WriteString(m.styles.Label.Render("👤 você"))
		b.WriteString("\n")
		b.WriteString(wrap(summarizeInput(m.pendingInput, 20), m.contentWidth()))
		b.WriteString("\n\n")
	}

	b.WriteString(m.styles.Label.Render("🤖 kraken"))
	b.WriteString("\n")
	for _, t := range m.plan.Tasks {
		switch t.Status {
		case domain.StatusDone:
			fmt.Fprintf(&b, "%s %s. %s\n", m.styles.Done.Render("✓"), t.ID, t.Title)
			if t.Result != "" {
				b.WriteString(indent(wrap(t.Result, m.contentWidth()-4), "    "))
				b.WriteString("\n\n")
			}
		case domain.StatusRunning:
			fmt.Fprintf(&b, "%s %s. %s\n", spinnerView(m.spinnerFrame), t.ID, t.Title)
			if t.Result != "" {
				b.WriteString(indent(wrap(t.Result, m.contentWidth()-4), "    "))
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

func (m Model) viewDone() string {
	var b strings.Builder
	header := "✓ execução concluída"
	if n := len(m.session); n > 1 {
		header = fmt.Sprintf("✓ sessão · %d turnos", n)
	}
	b.WriteString(m.styles.Done.Render(header))
	b.WriteString("\n\n")
	b.WriteString(m.renderHistory())
	return b.String()
}

func (m Model) viewError() string {
	var b strings.Builder
	b.WriteString(m.renderHistory())
	b.WriteString(m.styles.Failed.Render("✗ falha na execução"))
	b.WriteString("\n\n")
	if m.err != nil {
		b.WriteString(m.styles.ErrorText.Render(wrap(m.err.Error(), m.contentWidth())))
	}
	return b.String()
}

// renderPinnedPlan returns the plan + per-task statuses for the run that
// is currently in flight (or just finished/failed). It stays at the top of
// the viewport so the user always sees what is happening, while the body
// below scrolls. Empty during phaseInput, or when no plan exists yet.
func (m Model) renderPinnedPlan() string {
	if m.plan == nil {
		return ""
	}
	if m.phase == phaseInput {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.styles.Label.Render("Plano"))
	b.WriteString("\n")
	for _, t := range m.plan.Tasks {
		icon, style := m.taskDecoration(t)
		fmt.Fprintf(&b, "%s %s. %s\n", style.Render(icon), t.ID, t.Title)
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderHistory stacks all completed turns from the session, oldest first.
// Each turn is a chat-style block with the user's input followed by the
// orchestrator's compact plan and rendered final result.
func (m Model) renderHistory() string {
	if len(m.session) == 0 {
		return ""
	}
	var b strings.Builder
	for i, t := range m.session {
		b.WriteString(m.renderTurn(i, t))
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) renderTurn(idx int, t domain.Turn) string {
	var b strings.Builder

	if len(m.session) > 1 {
		b.WriteString(m.styles.Subtitle.Render(fmt.Sprintf("──── turno %d ────", idx+1)))
		b.WriteString("\n")
	}

	b.WriteString(m.styles.Label.Render("👤 você"))
	b.WriteString("\n")
	b.WriteString(wrap(summarizeInput(t.UserInput, 20), m.contentWidth()))
	b.WriteString("\n\n")

	b.WriteString(m.styles.Label.Render("🤖 kraken"))
	b.WriteString("\n")
	if t.Plan != nil {
		b.WriteString(m.renderCompactPlan(t.Plan))
	}
	if rendered := m.turnRenderFor(idx, t); rendered != "" {
		b.WriteString(strings.TrimRight(rendered, "\n"))
		b.WriteString("\n")
	}
	return b.String()
}

// turnRenderFor returns the cached markdown render for the given turn,
// falling back to a fresh render if the cache is missing it (e.g. after
// a width change). The cache lookup keeps the hot path cheap.
func (m Model) turnRenderFor(idx int, t domain.Turn) string {
	if idx < len(m.turnRender) && m.turnRender[idx] != "" {
		return m.turnRender[idx]
	}
	if t.Result == "" {
		return ""
	}
	return renderMarkdown(t.Result, m.contentWidth())
}

// renderCompactPlan lists the plan as a single icon+title line per task,
// without per-task results. Past turns use this; the live in-flight plan
// uses the verbose renderPlan.
func (m Model) renderCompactPlan(p *domain.Plan) string {
	var b strings.Builder
	for _, task := range p.Tasks {
		icon, style := m.taskDecoration(task)
		fmt.Fprintf(&b, "%s %s. %s\n", style.Render(icon), task.ID, task.Title)
	}
	b.WriteString("\n")
	return b.String()
}

// summarizeInput keeps the user input readable in the history: caps the
// number of lines so a 100-line script paste does not dominate the view.
func summarizeInput(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	hidden := len(lines) - maxLines
	return strings.Join(lines[:maxLines], "\n") +
		fmt.Sprintf("\n... (%d linhas adicionais)", hidden)
}

func (m Model) phaseHeadline() string {
	if m.plan == nil {
		return m.styles.Running.Render("planejando...")
	}
	return m.styles.Running.Render("executando plano...")
}

func (m Model) taskDecoration(t *domain.Task) (string, lipgloss.Style) {
	switch t.Status {
	case domain.StatusRunning:
		return spinnerView(m.spinnerFrame), m.styles.Running
	case domain.StatusDone:
		return "✓", m.styles.Done
	case domain.StatusFailed:
		return "✗", m.styles.Failed
	default:
		return "•", m.styles.Pending
	}
}

func (m Model) footer() string {
	switch m.phase {
	case phaseInput:
		return m.styles.Help.Render("ctrl+d: executar  •  enter: nova linha  •  ctrl+c: sair")
	case phaseRunning:
		return m.styles.Help.Render("↑↓ pgup/pgdn: rolar  •  ctrl+c: sair")
	case phaseDone, phaseError:
		return m.styles.Help.Render("↑↓ pgup/pgdn: rolar  •  c: continuar  •  r: nova tarefa  •  q: sair")
	}
	return ""
}

func (m Model) contentWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width - 4
}

func wrap(s string, width int) string {
	if width <= 10 {
		return s
	}
	var out strings.Builder
	for _, line := range strings.Split(s, "\n") {
		out.WriteString(wrapLine(line, width))
		out.WriteString("\n")
	}
	return strings.TrimRight(out.String(), "\n")
}

func wrapLine(line string, width int) string {
	if len(line) <= width {
		return line
	}
	words := strings.Fields(line)
	var b strings.Builder
	col := 0
	for i, w := range words {
		if col > 0 && col+1+len(w) > width {
			b.WriteString("\n")
			col = 0
		} else if i > 0 {
			b.WriteString(" ")
			col++
		}
		b.WriteString(w)
		col += len(w)
	}
	return b.String()
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
