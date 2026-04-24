package tui

import "github.com/charmbracelet/lipgloss"

// Styles groups the visual tokens used by the TUI.
// Keeping styles in one place makes the view pure rendering.
type Styles struct {
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	Box       lipgloss.Style
	Label     lipgloss.Style
	Pending   lipgloss.Style
	Running   lipgloss.Style
	Done      lipgloss.Style
	Failed    lipgloss.Style
	Result    lipgloss.Style
	ErrorText lipgloss.Style
	Help      lipgloss.Style
}

// DefaultStyles returns the standard palette.
func DefaultStyles() Styles {
	primary := lipgloss.Color("#7C3AED")
	muted := lipgloss.Color("#6B7280")
	green := lipgloss.Color("#10B981")
	yellow := lipgloss.Color("#F59E0B")
	red := lipgloss.Color("#EF4444")

	return Styles{
		Title:     lipgloss.NewStyle().Bold(true).Foreground(primary),
		Subtitle:  lipgloss.NewStyle().Foreground(muted),
		Box:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(primary).Padding(0, 1),
		Label:     lipgloss.NewStyle().Bold(true),
		Pending:   lipgloss.NewStyle().Foreground(muted),
		Running:   lipgloss.NewStyle().Foreground(yellow).Bold(true),
		Done:      lipgloss.NewStyle().Foreground(green),
		Failed:    lipgloss.NewStyle().Foreground(red).Bold(true),
		Result:    lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB")),
		ErrorText: lipgloss.NewStyle().Foreground(red),
		Help:      lipgloss.NewStyle().Foreground(muted).Italic(true),
	}
}
