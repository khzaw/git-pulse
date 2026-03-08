package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

type Theme struct {
	Name      string
	Frame     lipgloss.Style
	Panel     lipgloss.Style
	Title     lipgloss.Style
	Subtle    lipgloss.Style
	Highlight lipgloss.Style
}

func ResolveTheme(name string) (Theme, error) {
	switch name {
	case "", "tokyo-night":
		return Theme{
			Name:      "tokyo-night",
			Frame:     lipgloss.NewStyle().Foreground(lipgloss.Color("#c0caf5")).Background(lipgloss.Color("#1a1b26")),
			Panel:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#414868")).Padding(0, 1),
			Title:     lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Bold(true),
			Subtle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#a9b1d6")),
			Highlight: lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a")).Bold(true),
		}, nil
	case "gruvbox":
		return Theme{
			Name:      "gruvbox",
			Frame:     lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2")).Background(lipgloss.Color("#282828")),
			Panel:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#665c54")).Padding(0, 1),
			Title:     lipgloss.NewStyle().Foreground(lipgloss.Color("#fabd2f")).Bold(true),
			Subtle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#d5c4a1")),
			Highlight: lipgloss.NewStyle().Foreground(lipgloss.Color("#b8bb26")).Bold(true),
		}, nil
	default:
		return Theme{}, fmt.Errorf("unknown theme %q", name)
	}
}
