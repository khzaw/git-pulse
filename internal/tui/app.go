package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"git-pulse/internal/config"
)

type Model struct {
	cfg    config.Config
	theme  Theme
	width  int
	height int
}

func NewModel(cfg config.Config) (Model, error) {
	theme, err := ResolveTheme(cfg.Theme)
	if err != nil {
		return Model{}, err
	}

	return Model{
		cfg:   cfg,
		theme: theme,
	}, nil
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) View() string {
	header := m.theme.Title.Render("git-pulse")
	subtitle := m.theme.Subtle.Render(fmt.Sprintf("repo %s | theme %s | q quit", m.cfg.RepoPath, m.theme.Name))

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.panel("Commit Velocity", "Waiting for repository scan\nRolling trends and streaks will render here."),
		m.panel("Author Activity", "Contributor leaderboards and active counts will render here."),
	)

	lower := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.panel("File Hotspots", "Churn-heavy files, directories, and coupling insights."),
		m.panel("Branch & PR Health", "Branch freshness, release cadence, and PR cycle trends."),
	)

	return m.theme.Frame.Padding(1, 2).Render(strings.Join([]string{header, subtitle, "", body, lower}, "\n"))
}

func (m Model) panel(title, body string) string {
	return m.theme.Panel.Width(40).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			m.theme.Title.Render(title),
			m.theme.Subtle.Render(body),
		),
	)
}
