package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"git-pulse/internal/aggregator"
	"git-pulse/internal/config"
	gitrepo "git-pulse/internal/git"
	"git-pulse/internal/remote"
)

var panelOrder = []string{"overview", "velocity", "authors", "files", "branches", "prs"}

type dashboardLoadedMsg struct {
	snapshot aggregator.Snapshot
	prs      remote.PRSnapshot
	remote   remote.RepositoryRef
	warning  string
}

type dashboardErrorMsg struct {
	err error
}

type Model struct {
	cfg         config.Config
	theme       Theme
	width       int
	height      int
	loading     bool
	focused     int
	windowIndex int
	snapshot    aggregator.Snapshot
	prs         remote.PRSnapshot
	remote      remote.RepositoryRef
	status      string
}

func NewModel(cfg config.Config) (Model, error) {
	theme, err := ResolveTheme(cfg.Theme)
	if err != nil {
		return Model{}, err
	}

	index := 1
	for idx, window := range []aggregator.TimeWindow{
		aggregator.Window7Days,
		aggregator.Window30Days,
		aggregator.Window90Days,
		aggregator.Window1Year,
		aggregator.WindowAll,
	} {
		if string(window) == cfg.DefaultWindow {
			index = idx
			break
		}
	}

	return Model{
		cfg:         cfg,
		theme:       theme,
		loading:     true,
		focused:     0,
		windowIndex: index,
		status:      "loading repository metrics",
	}, nil
}

func (m Model) Init() tea.Cmd {
	return m.refreshCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case dashboardLoadedMsg:
		m.loading = false
		m.snapshot = msg.snapshot
		m.prs = msg.prs
		m.remote = msg.remote
		if msg.warning != "" {
			m.status = msg.warning
		} else {
			m.status = fmt.Sprintf("loaded %d commits for %s window", m.snapshot.Overview.CommitCount, m.currentWindow())
		}
	case dashboardErrorMsg:
		m.loading = false
		m.status = msg.err.Error()
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.focused = (m.focused + 1) % len(panelOrder)
		case "shift+tab":
			m.focused = (m.focused + len(panelOrder) - 1) % len(panelOrder)
		case "1", "2", "3", "4", "5", "6":
			m.focused = int(msg.String()[0] - '1')
		case "t":
			m.windowIndex = (m.windowIndex + 1) % len(m.windows())
			m.loading = true
			m.status = fmt.Sprintf("reloading %s window", m.currentWindow())
			return m, m.refreshCmd()
		case "r":
			m.loading = true
			m.status = "refreshing"
			return m, m.refreshCmd()
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		m.width = 160
	}

	header := m.renderHeader()
	panels := []string{
		m.renderPanel("overview", "Overview", m.renderOverview()),
		m.renderPanel("velocity", "Commit Velocity", m.renderCommitVelocity()),
		m.renderPanel("authors", "Author Activity", m.renderAuthors()),
		m.renderPanel("files", "File Hotspots", m.renderFiles()),
		m.renderPanel("branches", "Branch Health", m.renderBranches()),
		m.renderPanel("prs", "PR Cycle", m.renderPRs()),
	}

	body := m.renderGrid(panels)
	status := m.renderStatusBar()
	return m.theme.Frame.Padding(1, 2).Render(strings.Join([]string{header, "", body, "", status}, "\n"))
}

func (m Model) refreshCmd() tea.Cmd {
	repoPath := m.cfg.RepoPath
	window := m.currentWindow()

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		data, err := gitrepo.Scan(ctx, repoPath)
		if err != nil {
			return dashboardErrorMsg{err: err}
		}

		snapshot := aggregator.Aggregate(data, aggregator.Options{
			Now:    time.Now().UTC(),
			Window: window,
		})

		ref, err := remote.Detect(repoPath)
		if err != nil {
			return dashboardErrorMsg{err: err}
		}

		var prs remote.PRSnapshot
		warning := ""
		if ref.Provider == remote.ProviderGitHub {
			client := remote.NewGitHubClient(nil)
			prs, err = client.FetchSnapshot(ctx, ref)
			if err != nil {
				warning = fmt.Sprintf("remote metrics unavailable: %v", err)
			}
		}

		return dashboardLoadedMsg{
			snapshot: snapshot,
			prs:      prs,
			remote:   ref,
			warning:  warning,
		}
	}
}

func (m Model) renderHeader() string {
	repoName := m.snapshot.Repository.Path
	if repoName == "" {
		repoName = m.cfg.RepoPath
	}
	remoteName := "local-only"
	if fullName := m.remote.FullName(); fullName != "" {
		remoteName = string(m.remote.Provider) + " " + fullName
	}

	title := m.theme.Title.Render("git-pulse")
	meta := m.theme.Subtle.Render(fmt.Sprintf("%s | window %s | %s", repoName, m.currentWindow(), remoteName))
	if m.loading {
		meta = m.theme.Highlight.Render(meta)
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, meta)
}

func (m Model) renderGrid(panels []string) string {
	columns := 2
	if m.width >= 180 {
		columns = 3
	}
	panelWidth := (m.width - 8 - ((columns - 1) * 2)) / columns
	if panelWidth < 42 {
		columns = 1
		panelWidth = m.width - 8
	}

	resized := make([]string, len(panels))
	for idx, panel := range panels {
		resized[idx] = lipgloss.NewStyle().Width(panelWidth).Render(panel)
	}

	var rows []string
	for idx := 0; idx < len(resized); idx += columns {
		end := idx + columns
		if end > len(resized) {
			end = len(resized)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, resized[idx:end]...))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) renderPanel(key, title, body string) string {
	style := m.theme.Panel.Width(48).Height(13)
	if panelOrder[m.focused] == key {
		style = style.BorderForeground(lipgloss.Color("#7dcfff"))
	}
	return style.Render(lipgloss.JoinVertical(lipgloss.Left, m.theme.Title.Render(title), body))
}

func (m Model) renderOverview() string {
	breakdown := "no conventional commits"
	if len(m.snapshot.Overview.ConventionalBreakdown) > 0 {
		parts := make([]string, 0, len(m.snapshot.Overview.ConventionalBreakdown))
		for _, entry := range m.snapshot.Overview.ConventionalBreakdown {
			parts = append(parts, fmt.Sprintf("%s %d", entry.Name, entry.Value))
		}
		breakdown = strings.Join(parts, " | ")
	}

	lines := []string{
		fmt.Sprintf("%s commits  %s authors", m.theme.Highlight.Render(fmt.Sprintf("%d", m.snapshot.Overview.CommitCount)), m.theme.Highlight.Render(fmt.Sprintf("%d", m.snapshot.Overview.AuthorCount))),
		fmt.Sprintf("streak %d day  longest %d day", m.snapshot.Overview.CurrentStreak, m.snapshot.Overview.LongestStreak),
		fmt.Sprintf("net lines %+d", m.snapshot.Overview.NetLines),
		fmt.Sprintf("conventional %s", compactPercent(m.snapshot.Overview.ConventionalCommitShare)),
		truncate(breakdown, 42),
	}
	return m.theme.Subtle.Render(strings.Join(lines, "\n"))
}

func (m Model) renderCommitVelocity() string {
	dailyValues := make([]int, 0, len(m.snapshot.Commits.Daily))
	for _, entry := range m.snapshot.Commits.Daily {
		dailyValues = append(dailyValues, entry.Value)
	}

	weekdayLines := make([]string, 0, len(m.snapshot.Commits.Weekday))
	maxWeekday := maxNamedValue(m.snapshot.Commits.Weekday)
	for _, entry := range m.snapshot.Commits.Weekday {
		weekdayLines = append(weekdayLines, fmt.Sprintf("%s %s %d", entry.Name, progressBar(entry.Value, maxWeekday, 10), entry.Value))
	}

	lines := []string{
		fmt.Sprintf("90d spark %s", sparkline(dailyValues, 28)),
		fmt.Sprintf("weekly buckets %s", sparkline(dateValuesToInts(m.snapshot.Commits.Weekly), 20)),
		strings.Join(weekdayLines, "\n"),
	}
	return m.theme.Subtle.Render(strings.Join(lines, "\n"))
}

func (m Model) renderAuthors() string {
	lines := []string{
		fmt.Sprintf("active 7d %d  30d %d  bus factor %d", m.snapshot.Authors.ActiveThisWeek, m.snapshot.Authors.ActiveThisMonth, m.snapshot.Authors.BusFactor),
		"leaders",
	}

	for _, entry := range m.snapshot.Authors.Leaderboard {
		lines = append(lines, truncate(fmt.Sprintf("%-10s %2d commits %+d/-%d", entry.Name, entry.Commits, entry.Additions, entry.Deletions), 42))
	}
	if len(m.snapshot.Authors.NewContributors) > 0 {
		lines = append(lines, "new")
		for _, entry := range m.snapshot.Authors.NewContributors {
			lines = append(lines, truncate(entry.Name, 42))
		}
	}
	return m.theme.Subtle.Render(strings.Join(lines, "\n"))
}

func (m Model) renderFiles() string {
	lines := []string{"hotspots"}
	for _, file := range m.snapshot.Files.Hotspots {
		lines = append(lines, truncate(fmt.Sprintf("%-18s %2d touches %+d/-%d", file.Path, file.Touches, file.Additions, file.Deletions), 42))
	}
	if len(m.snapshot.Files.Directories) > 0 {
		lines = append(lines, "dirs")
		for _, entry := range m.snapshot.Files.Directories {
			lines = append(lines, truncate(fmt.Sprintf("%-12s churn %d  hits %d", entry.Path, entry.Churn, entry.Touches), 42))
		}
	}
	return m.theme.Subtle.Render(strings.Join(lines, "\n"))
}

func (m Model) renderBranches() string {
	lines := []string{
		fmt.Sprintf("active %d  stale %d", len(m.snapshot.Branches.ActiveBranches), len(m.snapshot.Branches.StaleBranches)),
		fmt.Sprintf("release cadence %s", compactDuration(time.Duration(m.snapshot.Branches.ReleaseCadenceDays*24)*time.Hour)),
		fmt.Sprintf("last tag %s", fallback(m.snapshot.Branches.LastTag, "none")),
		"branches",
	}
	for _, branch := range m.snapshot.Branches.ActiveBranches {
		lines = append(lines, truncate(fmt.Sprintf("%-16s %2dd old", branch.Name, branch.AgeDays), 42))
	}
	for idx, branch := range m.snapshot.Branches.StaleBranches {
		if idx >= 3 {
			break
		}
		lines = append(lines, truncate(fmt.Sprintf("%-16s %2dd old", branch.Name, branch.AgeDays), 42))
	}
	return m.theme.Subtle.Render(strings.Join(lines, "\n"))
}

func (m Model) renderPRs() string {
	if m.remote.Provider != remote.ProviderGitHub {
		return m.theme.Subtle.Render("No GitHub remote detected.\nPR metrics appear automatically when `origin` points at GitHub.")
	}
	if m.prs.Repository == "" {
		return m.theme.Subtle.Render("PR metrics unavailable.\nSet `GITHUB_TOKEN` for better rate limits and refresh with `r`.")
	}

	lines := []string{"medians"}
	for _, window := range m.prs.Windows {
		lines = append(lines, fmt.Sprintf("%s cycle %s  review %s  merged %d", window.Label, compactDuration(window.MedianCycleTime), compactDuration(window.MedianReviewTime), window.MergedCount))
	}
	lines = append(lines, fmt.Sprintf("throughput %s", sparkline(weeklyCountsToInts(m.prs.WeeklyThroughput), 18)))
	if len(m.prs.OpenPullRequests) > 0 {
		lines = append(lines, "open")
		for idx, pr := range m.prs.OpenPullRequests {
			if idx >= 3 {
				break
			}
			lines = append(lines, truncate(fmt.Sprintf("#%d %s (%dd)", pr.Number, pr.Title, pr.AgeDays), 42))
		}
	}
	return m.theme.Subtle.Render(strings.Join(lines, "\n"))
}

func (m Model) renderStatusBar() string {
	keys := "tab focus  1-6 jump  t window  r refresh  q quit"
	return m.theme.Panel.Border(lipgloss.HiddenBorder()).Padding(0, 1).Render(
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			m.theme.Highlight.Render(keys),
			m.theme.Subtle.Render("  |  "),
			m.theme.Subtle.Render(truncate(m.status, max(12, m.width-50))),
		),
	)
}

func (m Model) windows() []aggregator.TimeWindow {
	return []aggregator.TimeWindow{
		aggregator.Window7Days,
		aggregator.Window30Days,
		aggregator.Window90Days,
		aggregator.Window1Year,
		aggregator.WindowAll,
	}
}

func (m Model) currentWindow() aggregator.TimeWindow {
	return m.windows()[m.windowIndex]
}

func dateValuesToInts(values []aggregator.DateValue) []int {
	out := make([]int, 0, len(values))
	for _, entry := range values {
		out = append(out, entry.Value)
	}
	return out
}

func weeklyCountsToInts(values []remote.WeeklyCount) []int {
	out := make([]int, 0, len(values))
	for _, entry := range values {
		out = append(out, entry.Count)
	}
	return out
}

func maxNamedValue(values []aggregator.NamedValue) int {
	maximum := 0
	for _, entry := range values {
		if entry.Value > maximum {
			maximum = entry.Value
		}
	}
	return maximum
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
