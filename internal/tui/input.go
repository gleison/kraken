package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var placeholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)

const (
	tabSize     = 4
	gutterFirst = "> "
	gutterCont  = "  "
)

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
	case tea.KeyTab:
		t.value = appendNormalized(t.value, []rune{'\t'})
	case tea.KeyCtrlJ:
		// LF byte (0x0A) from a paste when the terminal didn't emit
		// bracketed-paste markers. Bubble Tea maps it to KeyCtrlJ, so
		// without this case embedded newlines would be silently dropped.
		t.value = append(t.value, '\n')
	case tea.KeyRunes, tea.KeySpace:
		t.value = appendNormalized(t.value, msg.Runes)
	}
}

// appendNormalized appends runes to the buffer, expanding tabs to spaces and
// folding any line-ending convention (LF, CR, CRLF) to a single '\n'. Pasted
// content frequently uses CR or CRLF, and treating CR as a literal character
// (or dropping it) would either confuse the renderer or silently lose the
// newlines from the paste.
func appendNormalized(dst, src []rune) []rune {
	for i := 0; i < len(src); i++ {
		r := src[i]
		switch r {
		case '\t':
			for j := 0; j < tabSize; j++ {
				dst = append(dst, ' ')
			}
		case '\r':
			dst = append(dst, '\n')
			if i+1 < len(src) && src[i+1] == '\n' {
				i++ // collapse CRLF
			}
		default:
			dst = append(dst, r)
		}
	}
	return dst
}

// View renders the input, soft-wrapping long lines at the configured width
// and preserving explicit newlines. The cursor always sits at the end of
// the buffer.
func (t textInput) View() string {
	if len(t.value) == 0 {
		return gutterFirst + placeholderStyle.Render(t.placeholder) + t.cursor()
	}

	innerWidth := t.width - lipgloss.Width(gutterFirst)
	if innerWidth < 10 {
		innerWidth = 10
	}

	var visualLines []string
	for i, logical := range strings.Split(string(t.value), "\n") {
		prefix := gutterCont
		if i == 0 {
			prefix = gutterFirst
		}
		visualLines = append(visualLines, wrapByCells(logical, prefix, innerWidth)...)
	}

	return strings.Join(visualLines, "\n") + t.cursor()
}

// wrapByCells breaks a logical line into visual lines at innerWidth cells.
// The first produced line uses firstPrefix; continuations use gutterCont so
// they align under the "> " prompt.
func wrapByCells(line, firstPrefix string, innerWidth int) []string {
	if line == "" {
		return []string{firstPrefix}
	}

	var out []string
	prefix := firstPrefix
	cells := 0
	start := 0
	runes := []rune(line)

	for i, r := range runes {
		w := lipgloss.Width(string(r))
		if cells+w > innerWidth && i > start {
			out = append(out, prefix+string(runes[start:i]))
			prefix = gutterCont
			start = i
			cells = 0
		}
		cells += w
	}
	out = append(out, prefix+string(runes[start:]))
	return out
}

func (t textInput) cursor() string {
	if t.focused {
		return "▌"
	}
	return ""
}
