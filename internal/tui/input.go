package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var placeholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)

// textInput is a tiny multi-line editor used by the input screen.
// It intentionally avoids the github.com/charmbracelet/bubbles dep:
// all we need is append rune / newline / backspace / clear / render.
type textInput struct {
	value       []rune
	placeholder string
	width       int
	focused     bool
}

func newTextInput(placeholder string) textInput {
	return textInput{placeholder: placeholder, focused: true}
}

// Value returns the current text as a string.
func (t textInput) Value() string { return string(t.value) }

// Reset clears the buffer.
func (t *textInput) Reset() { t.value = t.value[:0] }

// Focus marks the input as focused (affects cursor rendering).
func (t *textInput) Focus() { t.focused = true }

// SetWidth updates the visible width.
func (t *textInput) SetWidth(w int) { t.width = w }

// Newline appends a line break to the buffer.
func (t *textInput) Newline() { t.value = append(t.value, '\n') }

// Update handles a key message by mutating the buffer.
// Enter/Alt+Enter are decided by the caller (different semantics: submit vs.
// newline), so this method ignores tea.KeyEnter entirely.
func (t *textInput) Update(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyBackspace:
		if len(t.value) > 0 {
			t.value = t.value[:len(t.value)-1]
		}
	case tea.KeyRunes, tea.KeySpace:
		t.value = append(t.value, msg.Runes...)
	}
}

// View renders the input, soft-wrapping long lines at the configured width
// and preserving explicit newlines. The cursor always sits at the end of
// the buffer.
func (t textInput) View() string {
	if len(t.value) == 0 {
		return "> " + placeholderStyle.Render(t.placeholder) + t.cursor()
	}

	const gutterFirst = "> "
	const gutterCont = "  "

	innerWidth := t.width - len(gutterFirst)
	if innerWidth < 10 {
		innerWidth = 10
	}

	logicalLines := strings.Split(string(t.value), "\n")
	var visualLines []string
	for i, line := range logicalLines {
		prefix := gutterCont
		if i == 0 {
			prefix = gutterFirst
		}

		runes := []rune(line)
		if len(runes) == 0 {
			visualLines = append(visualLines, prefix)
			continue
		}
		for len(runes) > 0 {
			take := innerWidth
			if take > len(runes) {
				take = len(runes)
			}
			visualLines = append(visualLines, prefix+string(runes[:take]))
			runes = runes[take:]
			prefix = gutterCont
		}
	}

	return strings.Join(visualLines, "\n") + t.cursor()
}

func (t textInput) cursor() string {
	if t.focused {
		return "▌"
	}
	return ""
}
