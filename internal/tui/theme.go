package tui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Frame      lipgloss.Style
	Header     lipgloss.Style
	HeaderMeta lipgloss.Style
	Rule       lipgloss.Style
	Panel      lipgloss.Style
	PanelFocus lipgloss.Style
	Title      lipgloss.Style
	Muted      lipgloss.Style
	Strong     lipgloss.Style
	Accent     lipgloss.Style
	Tab        lipgloss.Style
	TabActive  lipgloss.Style
	Key        lipgloss.Style
	Status     lipgloss.Style
	Positive   lipgloss.Style
	Warning    lipgloss.Style
	Danger     lipgloss.Style
}

func ResolveTheme(_ string) (Theme, error) {
	return Theme{
		Frame:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#6f8f8b")),
		Header:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#101314")).Background(lipgloss.Color("#7fb8a4")).Padding(0, 1),
		HeaderMeta: lipgloss.NewStyle().Foreground(lipgloss.Color("#d8e3dc")).Bold(true),
		Rule:       lipgloss.NewStyle().Foreground(lipgloss.Color("#4f6a69")),
		Panel:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#5b7372")).Padding(0, 1),
		PanelFocus: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#8bd5ca")).Padding(0, 1),
		Title:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#dce8dd")),
		Muted:      lipgloss.NewStyle().Foreground(lipgloss.Color("#7c8f90")),
		Strong:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#eff6ef")),
		Accent:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8bd5ca")),
		Tab:        lipgloss.NewStyle().Foreground(lipgloss.Color("#7c8f90")).Padding(0, 1),
		TabActive:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#101314")).Background(lipgloss.Color("#8bd5ca")).Padding(0, 1),
		Key:        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8bd5ca")),
		Status:     lipgloss.NewStyle().Foreground(lipgloss.Color("#a7b9b1")),
		Positive:   lipgloss.NewStyle().Foreground(lipgloss.Color("#a6da95")).Bold(true),
		Warning:    lipgloss.NewStyle().Foreground(lipgloss.Color("#eed49f")).Bold(true),
		Danger:     lipgloss.NewStyle().Foreground(lipgloss.Color("#ed8796")).Bold(true),
	}, nil
}
