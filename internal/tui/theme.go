package tui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Frame      lipgloss.Style
	Header     lipgloss.Style
	Panel      lipgloss.Style
	PanelFocus lipgloss.Style
	Title      lipgloss.Style
	Muted      lipgloss.Style
	Strong     lipgloss.Style
	Accent     lipgloss.Style
}

func ResolveTheme(_ string) (Theme, error) {
	return Theme{
		Frame:      lipgloss.NewStyle().Border(lipgloss.NormalBorder()),
		Header:     lipgloss.NewStyle().Bold(true),
		Panel:      lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1),
		PanelFocus: lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).Padding(0, 1),
		Title:      lipgloss.NewStyle().Bold(true),
		Muted:      lipgloss.NewStyle().Faint(true),
		Strong:     lipgloss.NewStyle().Bold(true),
		Accent:     lipgloss.NewStyle().Underline(true).Bold(true),
	}, nil
}
