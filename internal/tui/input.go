package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var placeholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)

// textInput is a tiny single-field editor used by the input screen.
// It intentionally avoids the github.com/charmbracelet/bubbles dep:
// all we need is append rune / backspace / clear / render.
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

// Update handles a key message and reports whether the buffer changed.
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

// View renders the input line.
func (t textInput) View() string {
	var b strings.Builder
	b.WriteString("> ")
	if len(t.value) == 0 {
		b.WriteString(placeholderStyle.Render(t.placeholder))
	} else {
		b.WriteString(string(t.value))
	}
	if t.focused {
		b.WriteString("▌")
	}
	return b.String()
}
