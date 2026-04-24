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

// View renders the input, preserving embedded newlines.
func (t textInput) View() string {
	if len(t.value) == 0 {
		return "> " + placeholderStyle.Render(t.placeholder) + t.cursor()
	}

	lines := strings.Split(string(t.value), "\n")
	var b strings.Builder
	for i, line := range lines {
		if i == 0 {
			b.WriteString("> ")
		} else {
			b.WriteString("  ")
		}
		b.WriteString(line)
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	b.WriteString(t.cursor())
	return b.String()
}

func (t textInput) cursor() string {
	if t.focused {
		return "▌"
	}
	return ""
}
