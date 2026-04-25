package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/gleison/kraken/internal/domain"
)

// View renders the current state. The header and footer are always
// visible; the body in between is fitted to the remaining vertical space
// and can be scrolled with the keyboard when it overflows.
func (m Model) View() string {
	header := m.styles.Title.Render("🐙 kraken") + "  " +
		m.styles.Subtitle.Render("orquestrador de tarefas LLM")

	body := m.renderBody()
	footer := m.footer()

	return header + "\n\n" + m.viewport(body) + "\n" + m.scrollHint() + footer
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
func (m Model) viewport(body string) string {
	bodyHeight := m.bodyHeight()
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

// bodyHeight is the number of body lines the terminal can show without
// pushing the footer offscreen. 5 lines are reserved for header/footer/gaps.
func (m Model) bodyHeight() int {
	if m.height <= 0 {
		return 0
	}
	const reserved = 5
	if m.height <= reserved {
		return 1
	}
	return m.height - reserved
}

// scrollHint shows arrows when there is content above or below the viewport.
func (m Model) scrollHint() string {
	bodyHeight := m.bodyHeight()
	if bodyHeight <= 0 {
		return ""
	}
	total := m.bodyLineCount()
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

// bodyLineCount returns the number of lines in the current body.
// Used by scroll clamping and the scroll hint.
func (m Model) bodyLineCount() int {
	return len(strings.Split(m.renderBody(), "\n"))
}

func (m Model) viewInput() string {
	var b strings.Builder
	b.WriteString(m.styles.Label.Render("Objetivo"))
	b.WriteString("\n")
	b.WriteString(m.inputBox().Render(m.input.View()))
	return b.String()
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
	fmt.Fprintf(&b, "%s %s\n\n", spinnerView(m.spinnerFrame), m.phaseHeadline())

	if m.plan == nil {
		b.WriteString(m.styles.Subtitle.Render("Planejando tarefas..."))
		return b.String()
	}
	b.WriteString(m.renderPlan())
	return b.String()
}

func (m Model) viewDone() string {
	var b strings.Builder
	b.WriteString(m.styles.Done.Render("✓ execução concluída"))
	b.WriteString("\n\n")
	if m.plan != nil {
		b.WriteString(m.renderPlan())
		b.WriteString("\n")
	}
	if m.final != "" {
		b.WriteString(m.styles.Label.Render("Resultado final"))
		b.WriteString("\n")
		rendered := strings.TrimRight(renderMarkdown(m.final, m.contentWidth()-2), "\n")
		b.WriteString(m.inputBox().Render(rendered))
	}
	return b.String()
}

func (m Model) viewError() string {
	var b strings.Builder
	b.WriteString(m.styles.Failed.Render("✗ falha na execução"))
	b.WriteString("\n\n")
	if m.plan != nil {
		b.WriteString(m.renderPlan())
		b.WriteString("\n")
	}
	if m.err != nil {
		b.WriteString(m.styles.ErrorText.Render(m.err.Error()))
	}
	return b.String()
}

func (m Model) phaseHeadline() string {
	if m.plan == nil {
		return m.styles.Running.Render("planejando...")
	}
	return m.styles.Running.Render("executando plano...")
}

func (m Model) renderPlan() string {
	var b strings.Builder
	b.WriteString(m.styles.Label.Render("Plano"))
	b.WriteString("\n")
	for _, t := range m.plan.Tasks {
		b.WriteString(m.renderTask(t))
	}
	return b.String()
}

func (m Model) renderTask(t *domain.Task) string {
	icon, style := m.taskDecoration(t)
	header := fmt.Sprintf("%s %s. %s", icon, t.ID, t.Title)

	var b strings.Builder
	b.WriteString(style.Render(header))
	b.WriteString("\n")

	if t.Status == domain.StatusDone && t.Result != "" {
		indented := indent(wrap(t.Result, m.contentWidth()-4), "    ")
		b.WriteString(m.styles.Result.Render(indented))
		b.WriteString("\n")
	}
	if t.Status == domain.StatusFailed && t.Err != "" {
		b.WriteString(m.styles.ErrorText.Render("    " + t.Err))
		b.WriteString("\n")
	}
	return b.String()
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
		return m.styles.Help.Render("↑↓ pgup/pgdn: rolar  •  enter/r: nova tarefa  •  q: sair")
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
