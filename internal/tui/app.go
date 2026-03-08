package tui

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"git-pulse/internal/aggregator"
	"git-pulse/internal/config"
	"git-pulse/internal/dashboard"
	"git-pulse/internal/remote"
)

var panelOrder = []string{"velocity", "authors", "files", "prs", "branches", "churn"}

type dashboardLoadedMsg struct {
	snapshot aggregator.Snapshot
	remote   remote.RepositoryRef
	warning  string
}

type dashboardRemoteLoadedMsg struct {
	prs     remote.PRSnapshot
	warning string
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
	loader      dashboard.Loader
	remoteBusy  bool
}

type panelPalette struct {
	Border lipgloss.Style
	Title  lipgloss.Style
	Bar    lipgloss.Style
}

func NewModel(cfg config.Config) (Model, error) {
	theme, err := ResolveTheme(cfg.Theme)
	if err != nil {
		return Model{}, err
	}

	index := 1
	for idx, window := range windowOptions() {
		if string(window) == cfg.DefaultWindow {
			index = idx
			break
		}
	}

	return Model{
		cfg:         cfg,
		theme:       theme,
		loading:     true,
		status:      "loading repository metrics",
		windowIndex: index,
		loader:      dashboard.NewLoader(),
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
		m.remote = msg.remote
		m.prs = remote.PRSnapshot{}
		if msg.warning != "" {
			m.status = msg.warning
		} else {
			m.status = fmt.Sprintf("loaded %d commits for %s", m.snapshot.Overview.CommitCount, windowLabel(m.currentWindow()))
		}
		if m.remote.Provider == remote.ProviderGitHub {
			m.remoteBusy = true
			m.status = m.status + "  |  loading remote PR metrics"
			return m, m.refreshRemoteCmd()
		}
	case dashboardRemoteLoadedMsg:
		m.remoteBusy = false
		if msg.warning != "" {
			m.status = msg.warning
		} else {
			m.prs = msg.prs
			m.status = fmt.Sprintf("loaded %d commits for %s", m.snapshot.Overview.CommitCount, windowLabel(m.currentWindow()))
		}
	case dashboardErrorMsg:
		m.loading = false
		m.remoteBusy = false
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
			m.windowIndex = (m.windowIndex + 1) % len(windowOptions())
			m.loading = true
			m.remoteBusy = false
			m.status = fmt.Sprintf("reloading %s", windowLabel(m.currentWindow()))
			return m, m.refreshCmd()
		case "r":
			m.loading = true
			m.remoteBusy = false
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
	if m.height == 0 {
		m.height = 42
	}

	innerWidth := max(78, m.width-2)
	innerHeight := max(10, m.height-2)
	header := m.renderHeader(innerWidth)
	footer := m.renderFooter(innerWidth)
	sections := []string{header}
	var body string
	if m.compactMode() {
		body = m.renderCompact(innerWidth, innerHeight-4)
	} else {
		body = lipgloss.JoinVertical(lipgloss.Left, m.renderWide(innerWidth, innerHeight-4)...)
	}
	sections = append(sections, body)

	used := lipgloss.Height(header) + lipgloss.Height(body) + lipgloss.Height(footer)
	if spare := innerHeight - used; spare > 0 {
		sections = append(sections, strings.Repeat("\n", spare-1))
	}
	sections = append(sections, footer)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	content = padBlockHeight(content, innerWidth, innerHeight)
	return m.theme.Frame.Width(innerWidth + 2).Height(innerHeight).Render(content)
}

func (m Model) compactMode() bool {
	return m.width < 118 || m.height < 34
}

func (m Model) refreshCmd() tea.Cmd {
	repoPath := m.cfg.RepoPath
	window := m.currentWindow()

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := m.loader.LoadLocal(ctx, repoPath, window)
		if err != nil {
			return dashboardErrorMsg{err: err}
		}

		return dashboardLoadedMsg{
			snapshot: result.Snapshot,
			remote:   result.Remote,
			warning:  result.Warning,
		}
	}
}

func (m Model) refreshRemoteCmd() tea.Cmd {
	repoPath := m.cfg.RepoPath
	window := m.currentWindow()

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result, err := m.loader.LoadRemote(ctx, repoPath, window)
		if err != nil {
			return dashboardRemoteLoadedMsg{warning: err.Error()}
		}
		return dashboardRemoteLoadedMsg{
			prs:     result.PRs,
			warning: result.Warning,
		}
	}
}

func (m Model) renderHeader(width int) string {
	repoName := m.repositoryName()
	branch := fallback(m.snapshot.Repository.DefaultBranch, "HEAD")
	if m.compactMode() {
		repoName = filepath.Base(repoName)
	}
	left := m.theme.Header.Render("git-pulse") + m.theme.Muted.Render("  •  ") + m.theme.Strong.Render(repoName) + m.theme.Muted.Render("  •  ") + branch + m.theme.Muted.Render("  •  ") + headerWindowLabel(m.currentWindow(), m.compactMode())
	right := renderWindowTabs(m.currentWindow(), m.theme)
	line := joinEdge(left, right, width)
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.AdaptiveColor{Light: "8", Dark: "8"}).
		Render(truncate(line, width))
}

func (m Model) renderWide(width, height int) []string {
	left := (width - 1) / 2
	right := width - left - 1
	row1, row2, row3 := splitHeights(height)

	return []string{
		lipgloss.JoinHorizontal(lipgloss.Top,
			m.renderSection("velocity", "COMMIT VELOCITY", m.renderVelocity(left-4, row1-2), left, row1),
			m.renderSection("authors", "AUTHORS ACTIVE", m.renderAuthors(right-4, row1-2), right, row1),
		),
		lipgloss.JoinHorizontal(lipgloss.Top,
			m.renderSection("files", "FILE HOTSPOTS", m.renderFiles(left-4, row2-2), left, row2),
			m.renderSection("prs", "PR CYCLE TIME", m.renderPRs(right-4, row2-2), right, row2),
		),
		lipgloss.JoinHorizontal(lipgloss.Top,
			m.renderSection("branches", "BRANCH HEALTH", m.renderBranches(left-4, row3-2), left, row3),
			m.renderSection("churn", "CODE CHURN", m.renderChurn(right-4, row3-2), right, row3),
		),
	}
}

func (m Model) renderCompact(width, height int) string {
	panelKey := panelOrder[m.focused]
	var title string
	var body string
	switch panelKey {
	case "velocity":
		title = "COMMIT VELOCITY"
		body = m.renderVelocity(width-6, height-3)
	case "authors":
		title = "AUTHORS ACTIVE"
		body = m.renderAuthors(width-6, height-3)
	case "files":
		title = "FILE HOTSPOTS"
		body = m.renderFiles(width-6, height-3)
	case "prs":
		title = "PR CYCLE TIME"
		body = m.renderPRs(width-6, height-3)
	case "branches":
		title = "BRANCH HEALTH"
		body = m.renderBranches(width-6, height-3)
	default:
		title = "CODE CHURN"
		body = m.renderChurn(width-6, height-3)
	}

	label := fmt.Sprintf("panel %d/%d", m.focused+1, len(panelOrder))
	return m.theme.PanelFocus.Width(width).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			m.theme.Accent.Render(label),
			m.theme.Accent.Render("▸ "+title),
			fitLines(body, max(1, height-2)),
		),
	)
}

func (m Model) renderSection(key, title, body string, width, height int) string {
	return renderFramedSection(width, height, title, body, m.paletteFor(key), panelOrder[m.focused] == key)
}

func (m Model) renderVelocity(width, height int) string {
	values := dateValuesToInts(m.snapshot.Commits.Daily)
	avg, peak := avgAndPeak(values, windowDays(m.currentWindow()))
	trend := trendLabel(values)
	trend = m.colorizeTrend(trend)
	spark := sparkline(values, max(18, width-4))
	rangeLabel := dateRangeLabel(m.snapshot.Commits.Daily)
	heatmap := renderWeekHeatmap(m.snapshot.Commits.Daily, 5)
	hourly := sparkline(namedValuesToInts(m.snapshot.Commits.Hourly), clamp(width-10, 8, 24))

	lines := []string{
		joinEdge(fmt.Sprintf("Commits/Day (%s)", windowLabel(m.currentWindow())), fmt.Sprintf("%s  avg %.1f  peak %d", trend, avg, peak), width),
		"",
		m.paletteFor("velocity").Bar.Render(indent(spark, 1)),
		joinEdge("◂ "+rangeLabel[0], rangeLabel[1]+" ▸", width),
		"Weekly     " + m.paletteFor("velocity").Bar.Render(sparkline(dateValuesToInts(m.snapshot.Commits.Weekly), clamp(width-12, 10, 34))),
		"",
		"Day-of-Week Heatmap",
		"     M  T  W  T  F  S  S",
	}
	lines = append(lines, heatmap...)
	lines = append(lines, "", "Hour-of-Day "+m.theme.Muted.Render(hourly), fmt.Sprintf("Streak: %s", m.theme.Positive.Render(fmt.Sprintf("%d days", m.snapshot.Overview.CurrentStreak))))
	return strings.Join(trimLines(lines, height), "\n")
}

func (m Model) renderAuthors(width, height int) string {
	thisWeek := m.snapshot.Authors.ActiveThisWeek
	lastWeek := m.snapshot.Authors.ActiveLastWeek
	maxActive := max(1, max(thisWeek, lastWeek))
	leaderboardSlots := clamp(height-8, 3, 10)

	lines := []string{
		fmt.Sprintf("This Week  %s  %s", meterBar(thisWeek, maxActive, clamp(width-28, 8, 24), m.paletteFor("authors").Bar, m.theme.Muted), m.theme.Positive.Render(fmt.Sprintf("%d authors", thisWeek))),
		fmt.Sprintf("Last Week  %s  %s", meterBar(lastWeek, maxActive, clamp(width-28, 8, 24), m.paletteFor("authors").Bar.Copy().Faint(true), m.theme.Muted), m.theme.Muted.Render(fmt.Sprintf("%d authors", lastWeek))),
		fmt.Sprintf("30d Total  %s", m.theme.Strong.Render(fmt.Sprintf("%d active authors", m.snapshot.Authors.ActiveThisMonth))),
		"",
		"LEADERBOARD",
	}

	barWidth := clamp(width-24, 10, 28)
	maxCommits := 1
	for _, author := range m.snapshot.Authors.Leaderboard {
		if author.Commits > maxCommits {
			maxCommits = author.Commits
		}
	}
	for idx, author := range m.snapshot.Authors.Leaderboard {
		if idx >= leaderboardSlots {
			break
		}
		name := truncate(author.Name, clamp(width-20, 12, 18))
		lines = append(lines, fmt.Sprintf("%d. %-12s %s %3d", idx+1, name, meterBar(author.Commits, maxCommits, barWidth, m.paletteFor("authors").Bar, m.theme.Muted), author.Commits))
	}

	risk := "healthy"
	riskStyle := m.theme.Positive
	switch {
	case m.snapshot.Authors.BusFactor <= 2:
		risk = "fragile"
		riskStyle = m.theme.Danger
	case m.snapshot.Authors.BusFactor <= 4:
		risk = "moderate"
		riskStyle = m.theme.Warning
	}
	lines = append(lines, "", fmt.Sprintf("Bus Factor  %s  %d  %s", progressBar(m.snapshot.Authors.BusFactor, max(6, m.snapshot.Authors.BusFactor), 10), m.snapshot.Authors.BusFactor, riskStyle.Render(risk)))
	if len(m.snapshot.Authors.NewContributors) > 0 && len(lines) < height-1 {
		lines = append(lines, "New        "+truncate(m.snapshot.Authors.NewContributors[0].Name, clamp(width-12, 10, 24)))
	}
	return strings.Join(trimLines(lines, height), "\n")
}

func (m Model) renderFiles(width, height int) string {
	lines := []string{"Most Changed Files            hits  churn"}
	maxTouches := 1
	for _, file := range m.snapshot.Files.Hotspots {
		if file.Touches > maxTouches {
			maxTouches = file.Touches
		}
	}
	barWidth := clamp(width-28, 8, 24)
	hotspotSlots := clamp(height-7, 4, 10)
	for idx, file := range m.snapshot.Files.Hotspots {
		if idx >= hotspotSlots {
			break
		}
		churn := file.Additions + file.Deletions
		lines = append(lines, fmt.Sprintf("%-24s %s %3d %s", truncate(file.Path, 24), meterBar(file.Touches, maxTouches, barWidth, m.paletteFor("files").Bar, m.theme.Muted), file.Touches, m.theme.Warning.Render(fmt.Sprintf("+%d", churn))))
	}

	lines = append(lines, "", "Hot Directories")
	maxDir := 1
	for _, dir := range m.snapshot.Files.Directories {
		if dir.Churn > maxDir {
			maxDir = dir.Churn
		}
	}
	dirSlots := clamp(height-len(lines)-1, 1, 5)
	for idx, dir := range m.snapshot.Files.Directories {
		if idx >= dirSlots {
			break
		}
		lines = append(lines, fmt.Sprintf("%-16s %s %4d", truncate(dir.Path, 16), meterBar(dir.Churn, maxDir, clamp(width-25, 6, 16), m.paletteFor("files").Bar, m.theme.Muted), dir.Churn))
	}
	return strings.Join(trimLines(lines, height), "\n")
}

func (m Model) renderPRs(width, height int) string {
	if m.remote.Provider != remote.ProviderGitHub {
		return fitLines("No GitHub remote detected.\nPR metrics appear automatically when `origin` points at GitHub.", height)
	}
	if m.remoteBusy {
		return fitLines("Loading remote pull request metrics...\nLocal repository stats are already available.", height)
	}
	if m.prs.Repository == "" {
		return fitLines("PR metrics unavailable.\nLocal analytics are loaded; remote data did not arrive in time.", height)
	}

	lines := []string{"Median Time to Merge"}
	for _, window := range m.prs.Windows {
		lines = append(lines, fmt.Sprintf("%-4s %8s  review %-5s  merged %s", window.Label, compactDuration(window.MedianCycleTime), compactDuration(window.MedianReviewTime), m.theme.Positive.Render(fmt.Sprintf("%2d", window.MergedCount))))
	}

	cycleValues := weeklyCycleToHours(m.prs.WeeklyCycle)
	lines = append(lines, "", "Cycle Trend  "+m.paletteFor("prs").Bar.Render(sparkline(cycleValues, clamp(width-14, 10, 30))))
	lines = append(lines, "Throughput   "+m.paletteFor("prs").Bar.Render(sparkline(weeklyCountsToInts(m.prs.WeeklyThroughput), clamp(width-14, 10, 30))))

	stale := 0
	for _, pr := range m.prs.OpenPullRequests {
		if pr.AgeDays > 14 {
			stale++
		}
	}
	staleLabel := m.theme.Positive.Render(fmt.Sprintf("%d stale", stale))
	if stale > 0 {
		staleLabel = m.theme.Warning.Render(fmt.Sprintf("%d stale", stale))
	}
	lines = append(lines, fmt.Sprintf("Open PRs     %2d  (%s)", len(m.prs.OpenPullRequests), staleLabel))
	openSlots := clamp(height-len(lines), 1, 6)
	for idx, pr := range m.prs.OpenPullRequests {
		if idx >= openSlots {
			break
		}
		lines = append(lines, fmt.Sprintf("#%-5d %s", pr.Number, truncate(pr.Title, width-8)))
	}
	return strings.Join(trimLines(lines, height), "\n")
}

func (m Model) renderBranches(width, height int) string {
	lines := []string{
		fmt.Sprintf("Active: %s   Stale: %s   Last tag: %s", m.theme.Positive.Render(fmt.Sprintf("%d", len(m.snapshot.Branches.ActiveBranches))), m.theme.Warning.Render(fmt.Sprintf("%d", len(m.snapshot.Branches.StaleBranches))), fallback(m.snapshot.Branches.LastTag, "none")),
		fmt.Sprintf("Release cadence: %s", compactDuration(time.Duration(m.snapshot.Branches.ReleaseCadenceDays*24)*time.Hour)),
		"",
	}

	activeSlots := clamp((height-3)*2/3, 2, 8)
	for idx, branch := range m.snapshot.Branches.ActiveBranches {
		if idx >= activeSlots {
			break
		}
		lines = append(lines, fmt.Sprintf("%-20s %3dd old", truncate(branch.Name, 20), branch.AgeDays))
	}
	staleSlots := clamp(height-len(lines), 1, 4)
	for idx, branch := range m.snapshot.Branches.StaleBranches {
		if idx >= staleSlots {
			break
		}
		lines = append(lines, fmt.Sprintf("%-20s %3dd old", truncate(branch.Name, 20), branch.AgeDays))
	}
	return strings.Join(trimLines(lines, height), "\n")
}

func (m Model) renderChurn(width, height int) string {
	adds := m.snapshot.Overview.Additions
	dels := m.snapshot.Overview.Deletions
	totalChange := adds + dels

	lines := []string{
		fmt.Sprintf("Net LOC (%s)   %s", windowLabel(m.currentWindow()), m.theme.Positive.Render(fmt.Sprintf("%+d", m.snapshot.Overview.NetLines))),
		fmt.Sprintf("Adds  %s  %s", meterBar(adds, max(1, totalChange), clamp(width-18, 8, 24), m.theme.Positive, m.theme.Muted), m.theme.Positive.Render(fmt.Sprintf("%d", adds))),
		fmt.Sprintf("Dels  %s  %s", meterBar(dels, max(1, totalChange), clamp(width-18, 8, 24), m.theme.Danger, m.theme.Muted), m.theme.Danger.Render(fmt.Sprintf("%d", dels))),
		fmt.Sprintf("Change volume  %d lines", totalChange),
		fmt.Sprintf("Commit quality %s", m.colorizePercent(m.snapshot.Overview.ConventionalCommitShare)),
		"",
		"Commit Types",
		renderBreakdownLine(m.snapshot.Overview.ConventionalBreakdown, width),
		"Commit Rhythm",
		sparkline(namedValuesToInts(m.snapshot.Commits.Hourly), clamp(width-2, 12, 32)),
		"Weekday Load",
		sparkline(namedValuesToInts(m.snapshot.Commits.Weekday), clamp(width-2, 7, 20)),
	}
	return strings.Join(trimLines(lines, height), "\n")
}

func (m Model) renderFooter(width int) string {
	keys := m.theme.Accent.Render("tab") + " panel focus   " + m.theme.Accent.Render("1-6") + " jump   " + m.theme.Accent.Render("t") + " time range   " + m.theme.Accent.Render("r") + " refresh   " + m.theme.Accent.Render("q") + " quit "
	if m.compactMode() {
		keys = m.theme.Accent.Render("tab/1-6") + " switch panel   " + m.theme.Accent.Render("t") + " time range   " + m.theme.Accent.Render("r") + " refresh   " + m.theme.Accent.Render("q") + " quit "
	}

	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.AdaptiveColor{Light: "8", Dark: "8"}).
		Render(joinEdge(keys, truncate(m.status, clamp(width/2, 18, width-6)), width))
}

func (m Model) repositoryName() string {
	if fullName := m.remote.FullName(); fullName != "" {
		return fullName
	}
	repoPath := m.snapshot.Repository.Path
	if repoPath == "" {
		repoPath = m.cfg.RepoPath
	}
	base := filepath.Base(repoPath)
	if base == "." || base == "" {
		return repoPath
	}
	return base
}

func windowOptions() []aggregator.TimeWindow {
	return []aggregator.TimeWindow{
		aggregator.Window7Days,
		aggregator.Window30Days,
		aggregator.Window90Days,
		aggregator.Window1Year,
		aggregator.WindowAll,
	}
}

func (m Model) currentWindow() aggregator.TimeWindow {
	return windowOptions()[m.windowIndex]
}

func windowLabel(window aggregator.TimeWindow) string {
	switch window {
	case aggregator.Window7Days:
		return "Last 7 days"
	case aggregator.Window30Days:
		return "Last 30 days"
	case aggregator.Window90Days:
		return "Last 90 days"
	case aggregator.Window1Year:
		return "Last year"
	default:
		return "All time"
	}
}

func headerWindowLabel(window aggregator.TimeWindow, compact bool) string {
	if !compact {
		return windowLabel(window)
	}
	switch window {
	case aggregator.Window7Days:
		return "7d"
	case aggregator.Window30Days:
		return "30d"
	case aggregator.Window90Days:
		return "90d"
	case aggregator.Window1Year:
		return "1y"
	default:
		return "all"
	}
}

func windowDays(window aggregator.TimeWindow) int {
	switch window {
	case aggregator.Window7Days:
		return 7
	case aggregator.Window30Days:
		return 30
	case aggregator.Window90Days:
		return 90
	case aggregator.Window1Year:
		return 365
	default:
		return 0
	}
}

func renderWindowTabs(current aggregator.TimeWindow, theme Theme) string {
	parts := make([]string, 0, len(windowOptions()))
	for _, window := range windowOptions() {
		label := string(window)
		if window == current {
			parts = append(parts, theme.Accent.Render("["+label+"]"))
		} else {
			parts = append(parts, theme.Muted.Render(label))
		}
	}
	return strings.Join(parts, "  ")
}

func renderWeekHeatmap(values []aggregator.DateValue, rows int) []string {
	counts := map[string]int{}
	maxValue := 0
	var latest time.Time
	for _, entry := range values {
		key := entry.Date.Format("2006-01-02")
		counts[key] = entry.Value
		if entry.Value > maxValue {
			maxValue = entry.Value
		}
		if entry.Date.After(latest) {
			latest = entry.Date
		}
	}
	if latest.IsZero() {
		latest = time.Now().UTC()
	}

	start := startOfWeek(latest).AddDate(0, 0, -7*(rows-1))
	lines := make([]string, 0, rows)
	for row := 0; row < rows; row++ {
		week := start.AddDate(0, 0, row*7)
		label := fmt.Sprintf("W-%d", rows-row-1)
		if row == rows-1 {
			label = "Now"
		}

		cells := make([]string, 0, 7)
		for day := 0; day < 7; day++ {
			date := week.AddDate(0, 0, day)
			value := counts[date.Format("2006-01-02")]
			cells = append(cells, heatLevel(value, maxValue))
		}
		lines = append(lines, fmt.Sprintf("%-3s  %s", label, strings.Join(cells, "  ")))
	}
	return lines
}

func startOfWeek(ts time.Time) time.Time {
	offset := int(ts.Weekday()) - 1
	if ts.Weekday() == time.Sunday {
		offset = 6
	}
	return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, ts.Location()).AddDate(0, 0, -offset)
}

func heatLevel(value, maxValue int) string {
	levels := []string{"·", "▁", "▃", "▅", "▇", "█"}
	if value <= 0 || maxValue <= 0 {
		return levels[0]
	}
	idx := int(math.Round(float64(value) / float64(maxValue) * float64(len(levels)-1)))
	if idx < 1 {
		idx = 1
	}
	if idx >= len(levels) {
		idx = len(levels) - 1
	}
	return levels[idx]
}

func avgAndPeak(values []int, days int) (float64, int) {
	if days <= 0 {
		days = len(values)
	}
	if days <= 0 {
		return 0, 0
	}
	sum := 0
	peak := 0
	for _, value := range values {
		sum += value
		if value > peak {
			peak = value
		}
	}
	return float64(sum) / float64(days), peak
}

func trendLabel(values []int) string {
	if len(values) < 2 {
		return "→ flat"
	}
	half := len(values) / 2
	if half == 0 {
		half = 1
	}
	prev := sumInts(values[:half])
	next := sumInts(values[half:])
	if prev == 0 {
		if next == 0 {
			return "→ flat"
		}
		return "↗ new"
	}
	change := int(math.Round((float64(next-prev) / float64(prev)) * 100))
	switch {
	case change > 0:
		return fmt.Sprintf("↗ +%d%%", change)
	case change < 0:
		return fmt.Sprintf("↘ %d%%", change)
	default:
		return "→ 0%"
	}
}

func (m Model) colorizeTrend(label string) string {
	switch {
	case strings.HasPrefix(label, "↗"), strings.HasPrefix(label, "→"):
		return m.theme.Positive.Render(label)
	case strings.HasPrefix(label, "↘"):
		return m.theme.Warning.Render(label)
	default:
		return label
	}
}

func (m Model) colorizePercent(value float64) string {
	text := compactPercent(value)
	switch {
	case value >= 0.7:
		return m.theme.Positive.Render(text)
	case value >= 0.35:
		return m.theme.Warning.Render(text)
	default:
		return m.theme.Danger.Render(text)
	}
}

func dateRangeLabel(values []aggregator.DateValue) [2]string {
	if len(values) == 0 {
		return [2]string{"no history", "today"}
	}
	return [2]string{values[0].Date.Format("Jan 02"), values[len(values)-1].Date.Format("Jan 02")}
}

func renderBreakdownLine(values []aggregator.NamedValue, width int) string {
	if len(values) == 0 {
		return "no conventional breakdown"
	}
	parts := make([]string, 0, len(values))
	maxValue := maxNamedValue(values)
	barWidth := clamp(width/8, 2, 8)
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%s %s", value.Name, progressBar(value.Value, maxValue, barWidth)))
	}
	return truncate(strings.Join(parts, "  "), width)
}

func renderFramedSection(width, height int, title, body string, palette panelPalette, focused bool) string {
	contentHeight := max(1, height-2)
	lines := strings.Split(fitLines(body, contentHeight), "\n")

	rawTitle := " " + title + " "
	if focused {
		rawTitle = "▸ " + title + " "
	}
	titleRendered := palette.Title.Render(rawTitle)
	topPad := max(0, width-2-lipgloss.Width(titleRendered))
	top := palette.Border.Render("┌") + titleRendered + palette.Border.Render(strings.Repeat("─", topPad)+"┐")

	leftBorder := palette.Border.Render("│")
	rightBorder := palette.Border.Render("│")
	out := []string{top}
	for _, line := range lines {
		padded := padRight(truncate(line, width-2), width-2)
		out = append(out, leftBorder+padded+rightBorder)
	}
	out = append(out, palette.Border.Render("└"+strings.Repeat("─", width-2)+"┘"))
	return strings.Join(out, "\n")
}

func padRight(value string, width int) string {
	current := lipgloss.Width(value)
	if current >= width {
		return value
	}
	return value + strings.Repeat(" ", width-current)
}

func padBlockHeight(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = padRight(truncate(line, width), width)
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func splitHeights(total int) (int, int, int) {
	if total < 9 {
		return 3, 3, 3
	}
	row1 := max(8, total*36/100)
	row2 := max(8, total*36/100)
	row3 := total - row1 - row2
	if row3 < 6 {
		row3 = 6
		if row1 > 8 {
			row1--
		}
		if row2 > 8 && row1+row2+row3 > total {
			row2--
		}
	}
	for row1+row2+row3 > total {
		if row2 >= row1 && row2 > 8 {
			row2--
		} else if row1 > 8 {
			row1--
		} else {
			row3--
		}
	}
	for row1+row2+row3 < total {
		row3++
	}
	return row1, row2, row3
}

func weeklyCycleToHours(values []remote.WeeklyCycle) []int {
	out := make([]int, 0, len(values))
	for _, value := range values {
		out = append(out, int(math.Round(value.Median.Hours())))
	}
	return out
}

func namedValuesToInts(values []aggregator.NamedValue) []int {
	out := make([]int, 0, len(values))
	for _, value := range values {
		out = append(out, value.Value)
	}
	return out
}

func sumInts(values []int) int {
	sum := 0
	for _, value := range values {
		sum += value
	}
	return sum
}

func trimLines(lines []string, height int) []string {
	if len(lines) > height {
		return lines[:height]
	}
	return lines
}

func fitLines(text string, height int) string {
	lines := strings.Split(text, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func joinEdge(left, right string, width int) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	if leftWidth+rightWidth >= width {
		return truncate(left+" "+right, width)
	}
	return left + strings.Repeat(" ", width-leftWidth-rightWidth) + right
}

func indent(value string, amount int) string {
	return strings.Repeat(" ", amount) + value
}

func clamp(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func (m Model) paletteFor(key string) panelPalette {
	switch key {
	case "velocity":
		return panelPalette{
			Border: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "2", Dark: "2"}),
			Title:  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "2", Dark: "2"}).Bold(true),
			Bar:    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "2", Dark: "2"}),
		}
	case "authors":
		return panelPalette{
			Border: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "6", Dark: "6"}),
			Title:  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "6", Dark: "6"}).Bold(true),
			Bar:    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "6", Dark: "6"}),
		}
	case "files":
		return panelPalette{
			Border: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "3", Dark: "3"}),
			Title:  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "3", Dark: "3"}).Bold(true),
			Bar:    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "3", Dark: "3"}),
		}
	case "prs":
		return panelPalette{
			Border: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "5", Dark: "5"}),
			Title:  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "5", Dark: "5"}).Bold(true),
			Bar:    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "5", Dark: "5"}),
		}
	case "branches":
		return panelPalette{
			Border: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "4", Dark: "4"}),
			Title:  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "4", Dark: "4"}).Bold(true),
			Bar:    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "4", Dark: "4"}),
		}
	default:
		return panelPalette{
			Border: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "1", Dark: "1"}),
			Title:  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "1", Dark: "1"}).Bold(true),
			Bar:    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "1", Dark: "1"}),
		}
	}
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
