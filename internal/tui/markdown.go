package tui

import (
	"sync"

	"github.com/charmbracelet/glamour"
)

// Renderer construction is moderately expensive and, more importantly,
// glamour.WithAutoStyle() probes the terminal for its background colour
// on each render. The probe response (OSC 11) comes back through stdin,
// where Bubble Tea misreads it as a flood of key events that swamp the
// real ones. We side-step that entirely with a fixed dark style.
const glamourStyle = "dark"

var (
	rendererOnce  sync.Once
	rendererCache map[int]*glamour.TermRenderer
	rendererMu    sync.Mutex
)

func getRenderer(width int) *glamour.TermRenderer {
	rendererOnce.Do(func() {
		rendererCache = make(map[int]*glamour.TermRenderer)
	})
	rendererMu.Lock()
	defer rendererMu.Unlock()
	if r, ok := rendererCache[width]; ok {
		return r
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(glamourStyle),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil
	}
	rendererCache[width] = r
	return r
}

// renderMarkdown turns the LLM's Markdown output into ANSI-styled text
// (code fences highlighted, headings/bold/italic rendered) sized for the
// given width. On failure it returns the original text so the user always
// sees something.
func renderMarkdown(content string, width int) string {
	if width < 20 {
		width = 80
	}
	r := getRenderer(width)
	if r == nil {
		return content
	}
	out, err := r.Render(content)
	if err != nil {
		return content
	}
	return out
}
