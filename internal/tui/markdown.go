package tui

import "github.com/charmbracelet/glamour"

// renderMarkdown turns the LLM's Markdown output into ANSI-styled text
// (code fences highlighted, headings/bold/italic rendered) sized for the
// given width. On failure it returns the original text so the user always
// sees something.
func renderMarkdown(content string, width int) string {
	if width < 20 {
		width = 80
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return content
	}
	out, err := renderer.Render(content)
	if err != nil {
		return content
	}
	return out
}
