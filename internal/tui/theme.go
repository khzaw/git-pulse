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
	Positive   lipgloss.Style
	Warning    lipgloss.Style
	Danger     lipgloss.Style
}

func ResolveTheme(_ string) (Theme, error) {
	return Theme{
		Frame:      lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.AdaptiveColor{Light: "8", Dark: "8"}),
		Header:     lipgloss.NewStyle().Bold(true),
		Panel:      lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.AdaptiveColor{Light: "8", Dark: "8"}).Padding(0, 1),
		PanelFocus: lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(lipgloss.AdaptiveColor{Light: "6", Dark: "6"}).Padding(0, 1),
		Title:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "6", Dark: "6"}),
		Muted:      lipgloss.NewStyle().Faint(true),
		Strong:     lipgloss.NewStyle().Bold(true),
		Accent:     lipgloss.NewStyle().Underline(true).Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "6", Dark: "6"}),
		Positive:   lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "2", Dark: "2"}).Bold(true),
		Warning:    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "3", Dark: "3"}).Bold(true),
		Danger:     lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "1", Dark: "1"}).Bold(true),
	}, nil
}
