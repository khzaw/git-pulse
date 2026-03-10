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

var panelOrder = []string{"velocity", "authors", "prs", "files", "branches", "churn"}

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
	detailPanel string
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
		case "esc":
			if m.detailPanel != "" {
				m.detailPanel = ""
				return m, nil
			}
		case "tab":
			if m.detailPanel != "" {
				return m, nil
			}
			m.focused = (m.focused + 1) % len(panelOrder)
		case "shift+tab":
			if m.detailPanel != "" {
				return m, nil
			}
			m.focused = (m.focused + len(panelOrder) - 1) % len(panelOrder)
		case "1":
			if m.detailPanel == "velocity" {
				m.detailPanel = ""
			} else {
				m.detailPanel = "velocity"
				m.focused = 0
			}
		case "5":
			if m.detailPanel == "branches" {
				m.detailPanel = ""
			} else {
				m.detailPanel = "branches"
				m.focused = 4
			}
		case "2", "3", "4", "6":
			if m.detailPanel != "" {
				return m, nil
			}
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
	if m.detailPanel != "" {
		header = m.renderDetailHeader(innerWidth)
		footer = m.renderDetailFooter(innerWidth)
	}
	var body string
	bodyHeight := max(1, innerHeight-lipgloss.Height(header)-lipgloss.Height(footer))
	if m.detailPanel != "" {
		body = m.renderDetail(innerWidth, bodyHeight)
	} else if m.compactMode() {
		body = m.renderCompact(innerWidth, bodyHeight)
	} else {
		body = m.renderWide(innerWidth, bodyHeight)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
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
	left := m.theme.Header.Render("git-pulse") + " " + m.theme.HeaderMeta.Render(repoName) + m.theme.Muted.Render("  •  ") + m.theme.Strong.Render(branch)
	if m.compactMode() {
		repoName = filepath.Base(repoName)
		left = m.theme.Header.Render("git-pulse") + " " + m.theme.HeaderMeta.Render(repoName) + m.theme.Muted.Render("  •  ") + m.theme.Strong.Render(branch)
	} else {
		left += m.theme.Muted.Render("  •  ") + m.theme.HeaderMeta.Render(headerWindowLabel(m.currentWindow(), false))
	}
	right := renderWindowTabs(m.currentWindow(), m.theme)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		padRight(truncate(joinEdge(left, right, width), width), width),
		m.theme.Rule.Render(strings.Repeat("─", width)),
	)
}

func (m Model) renderWide(width, height int) string {
	c1, c2, c3 := splitThreeWidths(width)
	leftW, rightW := splitWidths(width, 46)

	// 3 border/transition lines + content
	avail := height - 3
	r1 := max(6, avail*33/100)
	r2 := max(6, avail*38/100)
	r3 := max(4, avail-r1-r2)

	var parts []string

	// Row 1: 3 equal columns
	parts = append(parts, m.renderTripleBorder("┌", "┬", "┐", c1, c2, c3,
		"velocity", "Commit Velocity",
		"authors", "Authors Active",
		"prs", "PR Cycle Time"))
	parts = append(parts, m.renderTripleContent(c1, c2, c3, r1,
		"velocity", m.renderVelocity,
		"authors", m.renderAuthors,
		"prs", m.renderPRs))

	// Transition: 3-col → full-width
	parts = append(parts, m.renderTripleToFullBorder(c1, c2, c3, "files", "File Hotspots"))

	// Row 2: full-width
	parts = append(parts, m.renderFullContent(width, r2, "files", m.renderFiles))

	// Transition: full-width → 2-col
	parts = append(parts, m.renderFullToSplitBorder(leftW, rightW, "branches", "Branch Health", "churn", "Code Churn"))

	// Row 3: 2 columns
	parts = append(parts, m.renderSplitContent(leftW, rightW, r3,
		"branches", m.renderBranches,
		"churn", m.renderChurn))

	return strings.Join(parts, "\n")
}

func splitThreeWidths(total int) (int, int, int) {
	usable := total - 1
	c := usable / 3
	rem := usable % 3
	c1, c2, c3 := c, c, c
	if rem >= 1 {
		c1++
	}
	if rem >= 2 {
		c3++
	}
	return c1, c2, c3
}

func (m Model) panelTitleLabel(key, title string) string {
	if panelOrder[m.focused] == key {
		return "[ " + title + " ]"
	}
	return "[ " + title + " ]"
}

func (m Model) renderTripleBorder(leftEdge, mid, rightEdge string, c1, c2, c3 int,
	key1, title1, key2, title2, key3, title3 string) string {
	p1, p2, p3 := m.paletteFor(key1), m.paletteFor(key2), m.paletteFor(key3)
	t1 := p1.Title.Render(m.panelTitleLabel(key1, title1))
	t2 := p2.Title.Render(m.panelTitleLabel(key2, title2))
	t3 := p3.Title.Render(m.panelTitleLabel(key3, title3))
	f1 := max(0, c1-1-lipgloss.Width(t1))
	f2 := max(0, c2-1-lipgloss.Width(t2))
	f3 := max(0, c3-1-lipgloss.Width(t3))
	return p1.Border.Render(leftEdge) + t1 + p1.Border.Render(strings.Repeat("─", f1)) +
		m.centerDivider(key1, key2).Render(mid) + t2 + p2.Border.Render(strings.Repeat("─", f2)) +
		m.centerDivider(key2, key3).Render(mid) + t3 + p3.Border.Render(strings.Repeat("─", f3)) +
		p3.Border.Render(rightEdge)
}

func (m Model) renderTripleContent(c1, c2, c3, height int,
	key1 string, r1 func(int, int) string,
	key2 string, r2 func(int, int) string,
	key3 string, r3 func(int, int) string,
) string {
	b1 := strings.Split(fitLines(r1(c1-2, height), height), "\n")
	b2 := strings.Split(fitLines(r2(c2-2, height), height), "\n")
	b3 := strings.Split(fitLines(r3(c3-2, height), height), "\n")
	lines := make([]string, height)
	for i := 0; i < height; i++ {
		lines[i] = m.paletteFor(key1).Border.Render("│") +
			padRight(truncate(b1[i], c1-1), c1-1) +
			m.centerDivider(key1, key2).Render("│") +
			padRight(truncate(b2[i], c2-1), c2-1) +
			m.centerDivider(key2, key3).Render("│") +
			padRight(truncate(b3[i], c3-1), c3-1) +
			m.paletteFor(key3).Border.Render("│")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderTripleToFullBorder(c1, c2, c3 int, key, title string) string {
	p := m.paletteFor(key)
	t := p.Title.Render(m.panelTitleLabel(key, title))
	tw := lipgloss.Width(t)
	avail := c1 - 1
	if tw > avail {
		t = p.Title.Render(truncate(m.panelTitleLabel(key, title), avail))
		tw = avail
	}
	return p.Border.Render("├") + t + p.Border.Render(strings.Repeat("─", avail-tw)) +
		p.Border.Render("┴") + p.Border.Render(strings.Repeat("─", c2-1)) +
		p.Border.Render("┴") + p.Border.Render(strings.Repeat("─", c3-1)) +
		p.Border.Render("┤")
}

func (m Model) renderFullContent(width, height int, key string, renderer func(int, int) string) string {
	p := m.paletteFor(key)
	body := strings.Split(fitLines(renderer(width-3, height), height), "\n")
	lines := make([]string, height)
	for i := 0; i < height; i++ {
		lines[i] = p.Border.Render("│") + padRight(truncate(body[i], width-2), width-2) + p.Border.Render("│")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderFullToSplitBorder(leftW, rightW int, leftKey, leftTitle, rightKey, rightTitle string) string {
	lp, rp := m.paletteFor(leftKey), m.paletteFor(rightKey)
	lt := lp.Title.Render(m.panelTitleLabel(leftKey, leftTitle))
	rt := rp.Title.Render(m.panelTitleLabel(rightKey, rightTitle))
	lf := max(0, leftW-1-lipgloss.Width(lt))
	rf := max(0, rightW-1-lipgloss.Width(rt))
	return lp.Border.Render("├") + lt + lp.Border.Render(strings.Repeat("─", lf)) +
		m.centerDivider(leftKey, rightKey).Render("┬") +
		rt + rp.Border.Render(strings.Repeat("─", rf)) + rp.Border.Render("┤")
}

func (m Model) renderSplitContent(leftW, rightW, height int,
	leftKey string, leftR func(int, int) string,
	rightKey string, rightR func(int, int) string,
) string {
	lb := strings.Split(fitLines(leftR(leftW-2, height), height), "\n")
	rb := strings.Split(fitLines(rightR(rightW-2, height), height), "\n")
	lines := make([]string, height)
	for i := 0; i < height; i++ {
		lines[i] = m.paletteFor(leftKey).Border.Render("│") +
			padRight(truncate(lb[i], leftW-1), leftW-1) +
			m.centerDivider(leftKey, rightKey).Render("│") +
			padRight(truncate(rb[i], rightW-1), rightW-1) +
			m.paletteFor(rightKey).Border.Render("│")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderDetail(width, height int) string {
	switch m.detailPanel {
	case "velocity":
		return m.renderVelocityDetail(width, height)
	case "branches":
		return m.renderBranchesDetail(width, height)
	default:
		return fitLines("detail view unavailable", height)
	}
}

func (m Model) renderCompact(width, height int) string {
	panelKey := panelOrder[m.focused]
	var title string
	var body string
	switch panelKey {
	case "velocity":
		title = "Commit Velocity"
		body = m.renderVelocity(width-6, height-3)
	case "authors":
		title = "Authors Active"
		body = m.renderAuthors(width-6, height-3)
	case "files":
		title = "File Hotspots"
		body = m.renderFiles(width-6, height-3)
	case "prs":
		title = "PR Cycle Time"
		body = m.renderPRs(width-6, height-3)
	case "branches":
		title = "Branch Health"
		body = m.renderBranches(width-6, height-3)
	default:
		title = "Code Churn"
		body = m.renderChurn(width-6, height-3)
	}

	label := fmt.Sprintf("panel %d/%d", m.focused+1, len(panelOrder))
	return m.theme.PanelFocus.Width(width).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			m.theme.Muted.Render(label),
			m.theme.Accent.Render("[ ▸ "+title+" ]"),
			fitLines(body, max(1, height-2)),
		),
	)
}

func (m Model) renderDetailHeader(width int) string {
	repoName := m.repositoryName()
	branch := fallback(m.snapshot.Repository.DefaultBranch, "HEAD")
	title := detailTitle(m.detailPanel)
	left := m.theme.Header.Render("git-pulse") + " " + m.theme.HeaderMeta.Render(strings.ToUpper(title)) + m.theme.Muted.Render("  •  ") + m.theme.HeaderMeta.Render(repoName) + m.theme.Muted.Render("  •  ") + m.theme.Strong.Render(branch)
	right := m.theme.Key.Render("esc") + m.theme.Muted.Render(" back to dashboard")
	return lipgloss.JoinVertical(
		lipgloss.Left,
		padRight(truncate(joinEdge(left, right, width), width), width),
		m.theme.Rule.Render(strings.Repeat("─", width)),
	)
}

func (m Model) renderDetailFooter(width int) string {
	keys := m.theme.Key.Render("esc") + m.theme.Muted.Render(" dashboard   ") + m.theme.Key.Render("t") + m.theme.Muted.Render(" time range   ") + m.theme.Key.Render("r") + m.theme.Muted.Render(" refresh   ") + m.theme.Key.Render("q") + m.theme.Muted.Render(" quit ")
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.theme.Rule.Render(strings.Repeat("─", width)),
		padRight(truncate(joinEdge(keys, m.theme.Status.Render(truncate(m.status, clamp(width/2, 18, width-6))), width), width), width),
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
	chartWidth := max(24, width-2)
	chartRows := min(10, max(3, height*2/5))
	dailyChart := columnChart(values, chartWidth, chartRows)
	rangeLabel := dateRangeLabel(m.snapshot.Commits.Daily)
	heatmap := renderWeekHeatmap(m.snapshot.Commits.Daily, 5, m.paletteFor("velocity"))
	sparkWidth := clamp(width-12, 12, 60)
	halfSpark := clamp((width-24)/2, 8, 40)
	weeklyChart := sparkline(dateValuesToInts(m.snapshot.Commits.Weekly), halfSpark)
	weekday := sparkline(namedValuesToInts(m.snapshot.Commits.Weekday), min(7, halfSpark))
	hourly := sparkline(namedValuesToInts(m.snapshot.Commits.Hourly), sparkWidth)
	busiestDay, busiestDayCount := maxNamedValueEntry(m.snapshot.Commits.Weekday)
	busiestHour, busiestHourCount := maxNamedValueEntry(m.snapshot.Commits.Hourly)

	lines := []string{
		joinEdge(fmt.Sprintf("Commits/Day (%s)", windowLabel(m.currentWindow())), fmt.Sprintf("%s  avg %.1f  peak %d", trend, avg, peak), width),
	}
	for _, row := range dailyChart {
		lines = append(lines, m.paletteFor("velocity").Bar.Render(indent(row, 1)))
	}
	lines = append(lines,
		joinEdge("◂ "+rangeLabel[0], rangeLabel[1]+" ▸", width),
		joinEdge("Weekly  "+m.paletteFor("velocity").Bar.Render(weeklyChart), "Weekday  "+m.paletteFor("velocity").Bar.Render(weekday), width),
		"Hours   "+m.theme.Muted.Render(hourly),
		"     M  T  W  T  F  S  S",
	)
	lines = append(lines, heatmap...)
	lines = append(
		lines,
		joinEdge(
			fmt.Sprintf("Streak  %s", m.theme.Positive.Render(fmt.Sprintf("%d days", m.snapshot.Overview.CurrentStreak))),
			fmt.Sprintf("Longest  %s", m.theme.Strong.Render(fmt.Sprintf("%d days", m.snapshot.Overview.LongestStreak))),
			width,
		),
		joinEdge(
			fmt.Sprintf("Busiest  %s (%d)", busiestDay, busiestDayCount),
			fmt.Sprintf("Hour  %s:00 (%d)", busiestHour, busiestHourCount),
			width,
		),
	)
	return strings.Join(trimLines(lines, height), "\n")
}

func (m Model) renderVelocityDetail(width, height int) string {
	topHeight := max(14, height*3/5)
	bottomHeight := max(8, height-topHeight-1)
	chart := m.renderVelocityFocusChart(width, topHeight)
	lower := m.renderVelocityFocusLower(width, bottomHeight)
	return lipgloss.JoinVertical(lipgloss.Left, chart, lower)
}

func (m Model) renderVelocityFocusChart(width, height int) string {
	values := dateValuesToInts(m.snapshot.Commits.Daily)
	avg, peak := avgAndPeak(values, windowDays(m.currentWindow()))
	chartHeight := max(8, height-5)
	chartWidth := max(40, width-2)
	rows := renderBrailleAxisChart(values, chartWidth, chartHeight)
	dateAxis := renderDateAxis(m.snapshot.Commits.Daily, chartWidth)

	lines := []string{
		joinEdge(fmt.Sprintf("COMMITS PER DAY (%s)", strings.ToUpper(headerWindowLabel(m.currentWindow(), false))), fmt.Sprintf("avg: %.1f  peak: %d", avg, peak), width),
	}
	lines = append(lines, rows...)
	lines = append(lines,
		dateAxis,
		joinEdge(
			m.theme.Muted.Render("    ─ 7d avg    ━━ 30d avg"),
			m.colorizeTrend(trendLabel(values)),
			width,
		),
	)
	return fitLines(strings.Join(lines, "\n"), height)
}

func (m Model) renderVelocityFocusLower(width, height int) string {
	leftWidth, rightWidth := splitWidths(width, 42)
	vlTitle := m.paletteFor("velocity").Title.Render("[ Hour Of Day ]")
	vrTitle := m.paletteFor("velocity").Title.Render("[ Weekly Summary (last 12 weeks) ]")
	header := m.paletteFor("velocity").Border.Render("├") +
		vlTitle +
		m.paletteFor("velocity").Border.Render(strings.Repeat("─", max(0, leftWidth-1-lipgloss.Width(vlTitle)))) +
		m.centerDivider("velocity", "velocity").Render("┬") +
		vrTitle +
		m.paletteFor("velocity").Border.Render(strings.Repeat("─", max(0, rightWidth-1-lipgloss.Width(vrTitle)))+"┤")

	contentHeight := max(1, height-1)
	left := strings.Split(fitLines(m.renderVelocityHourDetail(leftWidth-2, contentHeight), contentHeight), "\n")
	right := strings.Split(fitLines(m.renderVelocityWeeklyDetail(rightWidth-2, contentHeight), contentHeight), "\n")

	lines := []string{header}
	for idx := 0; idx < contentHeight; idx++ {
		lines = append(lines,
			m.paletteFor("velocity").Border.Render("│")+
				padRight(truncate(left[idx], leftWidth-1), leftWidth-1)+
				m.centerDivider("velocity", "velocity").Render("│")+
				padRight(truncate(right[idx], rightWidth-1), rightWidth-1)+
				m.paletteFor("velocity").Border.Render("│"),
		)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderVelocityHourDetail(width, height int) string {
	values := namedValuesToInts(m.snapshot.Commits.Hourly)
	chartHeight := max(6, height-2)
	chartWidth := max(24, width-4)
	rows := columnChart(values, chartWidth, chartHeight)
	lines := make([]string, 0, height)
	for _, row := range rows {
		lines = append(lines, m.paletteFor("velocity").Bar.Render(row))
	}
	// Generate hour labels spaced to match chart width
	colsPerHour := chartWidth / 24
	if colsPerHour < 1 {
		colsPerHour = 1
	}
	var labelLine strings.Builder
	for hour := 0; hour < 24; hour++ {
		label := fmt.Sprintf("%d", hour)
		labelLine.WriteString(label)
		pad := colsPerHour - len(label)
		if pad > 0 {
			labelLine.WriteString(strings.Repeat(" ", pad))
		}
	}
	lines = append(lines, truncate(labelLine.String(), width))
	lines = append(lines, m.theme.Muted.Render("hour (local repo time)"))
	return strings.Join(trimLines(lines, height), "\n")
}

func (m Model) renderVelocityWeeklyDetail(width, height int) string {
	weekly := m.snapshot.Commits.Weekly
	slots := clamp(height, 4, 12)
	if len(weekly) > slots {
		weekly = weekly[len(weekly)-slots:]
	}
	maxValue := 1
	for _, entry := range weekly {
		if entry.Value > maxValue {
			maxValue = entry.Value
		}
	}
	lines := make([]string, 0, len(weekly))
	for idx, entry := range weekly {
		label := fmt.Sprintf("W-%d", len(weekly)-idx-1)
		if idx == len(weekly)-1 {
			label = "Now"
		}
		lines = append(lines, fmt.Sprintf("%-5s %s %4d", label, meterBar(entry.Value, maxValue, clamp(width-14, 12, 60), m.paletteFor("velocity").Bar, m.theme.Muted), entry.Value))
	}
	return strings.Join(trimLines(lines, height), "\n")
}

func (m Model) renderAuthors(width, height int) string {
	thisWeek := m.snapshot.Authors.ActiveThisWeek
	lastWeek := m.snapshot.Authors.ActiveLastWeek
	maxActive := max(1, max(thisWeek, lastWeek))
	leaderboardSlots := clamp(height-8, 3, 10)
	topShare := 0
	if len(m.snapshot.Authors.Leaderboard) > 0 && m.snapshot.Overview.CommitCount > 0 {
		topShare = int(math.Round(float64(m.snapshot.Authors.Leaderboard[0].Commits) / float64(m.snapshot.Overview.CommitCount) * 100))
	}

	lines := []string{
		fmt.Sprintf("This Week  %s  %s", meterBar(thisWeek, maxActive, clamp(width-28, 8, 24), m.paletteFor("authors").Bar, m.theme.Muted), m.theme.Positive.Render(fmt.Sprintf("%d authors", thisWeek))),
		fmt.Sprintf("Last Week  %s  %s", meterBar(lastWeek, maxActive, clamp(width-28, 8, 24), m.paletteFor("authors").Bar.Copy().Faint(true), m.theme.Muted), m.theme.Muted.Render(fmt.Sprintf("%d authors", lastWeek))),
		joinEdge(fmt.Sprintf("30d Total  %s", m.theme.Strong.Render(fmt.Sprintf("%d active authors", m.snapshot.Authors.ActiveThisMonth))), fmt.Sprintf("Top share  %d%%", topShare), width),
		m.theme.Muted.Render("LEADERBOARD"),
	}

	nameWidth := clamp(width/4, 10, 18)
	// rank(4) + name + space(1) + bar + space(1) + count(4) + space(1) + churn(~7) = name + bar + 18
	barWidth := clamp(width-nameWidth-18, 8, 36)
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
		name := padRight(truncate(author.Name, nameWidth), nameWidth)
		churn := author.Additions + author.Deletions
		lines = append(lines, fmt.Sprintf("%d. %s %s %3d  %s", idx+1, name, meterBar(author.Commits, maxCommits, barWidth, m.paletteFor("authors").Bar, m.theme.Muted), author.Commits, m.theme.Muted.Render(fmt.Sprintf("+%d", churn))))
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
	lines = append(lines, fmt.Sprintf("Bus Factor  %s  %d  %s", progressBar(m.snapshot.Authors.BusFactor, max(6, m.snapshot.Authors.BusFactor), 10), m.snapshot.Authors.BusFactor, riskStyle.Render(risk)))
	newSlots := clamp(height-len(lines)-1, 0, 3)
	if newSlots > 0 {
		if len(m.snapshot.Authors.NewContributors) == 0 {
			lines = append(lines, "New        none this window")
		} else {
			for idx, author := range m.snapshot.Authors.NewContributors {
				if idx >= newSlots {
					break
				}
				prefix := "New        "
				if idx > 0 {
					prefix = "           "
				}
				lines = append(lines, prefix+truncate(author.Name, clamp(width-12, 10, 24)))
			}
		}
	}
	return strings.Join(trimLines(lines, height), "\n")
}

func (m Model) renderFiles(width, height int) string {
	// Measure the longest path to avoid huge gaps in full-width panels
	maxPathLen := 18
	for _, file := range m.snapshot.Files.Hotspots {
		if l := len(file.Path); l > maxPathLen {
			maxPathLen = l
		}
	}
	pathWidth := clamp(maxPathLen, 18, min(60, width/2))
	barWidth := clamp(width/8, 5, 20)
	// Recalculate: if we have spare space after path+bar+numbers, widen the bar
	used := pathWidth + barWidth + 17
	if used < width {
		extra := width - used
		barWidth += min(extra, 20)
	}

	lines := []string{joinEdge("Most Changed Files", padRight("", pathWidth-18)+"hits churn age", width)}
	maxTouches := 1
	for _, file := range m.snapshot.Files.Hotspots {
		if file.Touches > maxTouches {
			maxTouches = file.Touches
		}
	}
	hotspotSlots := clamp(height-4, 5, 16)
	for idx, file := range m.snapshot.Files.Hotspots {
		if idx >= hotspotSlots {
			break
		}
		churn := file.Additions + file.Deletions
		lines = append(lines, fmt.Sprintf("%s %s %3d %5s %4s", padRight(truncate(file.Path, pathWidth), pathWidth), meterBar(file.Touches, maxTouches, barWidth, m.paletteFor("files").Bar, m.theme.Muted), file.Touches, m.theme.Positive.Render(fmt.Sprintf("+%d", churn)), m.theme.Muted.Render(ageLabel(file.LastChange))))
	}

	// Hot directories — measure actual path widths too
	maxDirLen := 12
	for _, dir := range m.snapshot.Files.Directories {
		if l := len(dir.Path); l > maxDirLen {
			maxDirLen = l
		}
	}
	dirPathWidth := clamp(maxDirLen, 12, min(40, width/2))
	dirBarWidth := clamp(width/8, 6, 20)
	dirUsed := dirPathWidth + dirBarWidth + 12
	if dirUsed < width {
		dirBarWidth += min(width-dirUsed, 20)
	}
	maxDir := 1
	for _, dir := range m.snapshot.Files.Directories {
		if dir.Churn > maxDir {
			maxDir = dir.Churn
		}
	}
	dirSlots := clamp(height-len(lines), 1, 6)
	if dirSlots > 0 && len(m.snapshot.Files.Directories) > 0 {
		lines = append(lines, m.theme.Muted.Render("Hot Directories"))
		for idx, dir := range m.snapshot.Files.Directories {
			if idx >= dirSlots-1 {
				break
			}
			lines = append(lines, fmt.Sprintf("%s %s %5d %3dt", padRight(truncate(dir.Path, dirPathWidth), dirPathWidth), meterBar(dir.Churn, maxDir, dirBarWidth, m.paletteFor("files").Bar, m.theme.Muted), dir.Churn, dir.Touches))
		}
	}
	if len(lines) < height && len(m.snapshot.Files.Hotspots) == 0 {
		lines = append(lines, "No file churn in the selected window.")
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
	if len(cycleValues) > 0 {
		lines = append(lines, "Cycle Trend  "+m.paletteFor("prs").Bar.Render(sparkline(cycleValues, clamp(width-14, 10, 60))))
	}
	lines = append(lines, "Throughput   "+m.paletteFor("prs").Bar.Render(sparkline(weeklyCountsToInts(m.prs.WeeklyThroughput), clamp(width-14, 10, 60))))
	lines = append(lines, joinEdge(fmt.Sprintf("Review p50  %s", compactDuration(m.prs.MedianReviewTime)), fmt.Sprintf("Coverage  %d%%", m.prs.ReviewCoverage), width))

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
	openSlots := clamp(height-len(lines), 1, 10)
	for idx, pr := range m.prs.OpenPullRequests {
		if idx >= openSlots {
			break
		}
		lines = append(lines, fmt.Sprintf("#%-5d %-4s %s", pr.Number, m.theme.Muted.Render(fmt.Sprintf("%dd", pr.AgeDays)), truncate(pr.Title, width-14)))
	}
	return strings.Join(trimLines(lines, height), "\n")
}

func (m Model) renderBranches(width, height int) string {
	lines := []string{
		joinEdge(fmt.Sprintf("Active  %s", m.theme.Positive.Render(fmt.Sprintf("%d", len(m.snapshot.Branches.ActiveBranches)))), fmt.Sprintf("Stale  %s", m.theme.Warning.Render(fmt.Sprintf("%d", len(m.snapshot.Branches.StaleBranches)))), width),
		joinEdge(fmt.Sprintf("Last tag  %s", fallback(m.snapshot.Branches.LastTag, "none")), fmt.Sprintf("Cadence  %s", compactDuration(time.Duration(m.snapshot.Branches.ReleaseCadenceDays*24)*time.Hour)), width),
	}

	maxAge := 1
	for _, branch := range append(append([]aggregator.BranchSummary{}, m.snapshot.Branches.ActiveBranches...), m.snapshot.Branches.StaleBranches...) {
		if branch.AgeDays > maxAge {
			maxAge = branch.AgeDays
		}
	}
	barWidth := clamp(width/5, 6, 18)
	nameWidth := max(24, width-barWidth-6)

	remaining := height - len(lines)
	activeSlots := clamp(remaining*2/3, 2, 8)
	if len(m.snapshot.Branches.ActiveBranches) > 0 {
		lines = append(lines, m.theme.Muted.Render("ACTIVE"))
		for idx, branch := range m.snapshot.Branches.ActiveBranches {
			if idx >= activeSlots {
				break
			}
			freshness := meterBar(maxAge-branch.AgeDays, maxAge, barWidth, m.paletteFor("branches").Bar, m.theme.Muted)
			lines = append(lines, fmt.Sprintf("%s %s %4dd", padRight(truncate(branch.Name, nameWidth), nameWidth), freshness, branch.AgeDays))
		}
	}
	staleSlots := clamp(height-len(lines), 1, 8)
	if len(m.snapshot.Branches.StaleBranches) > 0 {
		lines = append(lines, m.theme.Muted.Render("STALE"))
		for idx, branch := range m.snapshot.Branches.StaleBranches {
			if idx >= staleSlots-1 {
				break
			}
			ageBar := meterBar(branch.AgeDays, maxAge, barWidth, m.theme.Warning, m.theme.Muted)
			lines = append(lines, fmt.Sprintf("%s %s %4dd", padRight(truncate(branch.Name, nameWidth), nameWidth), ageBar, branch.AgeDays))
		}
	} else if len(lines) < height {
		lines = append(lines, m.theme.Muted.Render("No stale branches"))
	}
	return strings.Join(trimLines(lines, height), "\n")
}

func (m Model) renderBranchesDetail(width, height int) string {
	topHeight := max(8, height/4)
	bottomHeight := max(12, height-topHeight-1)
	top := m.renderBranchesFocusSummary(width, topHeight)
	bottom := m.renderBranchesFocusLists(width, bottomHeight)
	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

func (m Model) renderBranchesFocusSummary(width, height int) string {
	lines := []string{
		joinEdge(
			fmt.Sprintf("ACTIVE %s", m.theme.Positive.Render(fmt.Sprintf("%d", len(m.snapshot.Branches.ActiveBranches)))),
			fmt.Sprintf("STALE %s", m.theme.Warning.Render(fmt.Sprintf("%d", len(m.snapshot.Branches.StaleBranches)))),
			width,
		),
		joinEdge(
			fmt.Sprintf("LAST TAG  %s", fallback(m.snapshot.Branches.LastTag, "none")),
			fmt.Sprintf("CADENCE  %s", compactDuration(time.Duration(m.snapshot.Branches.ReleaseCadenceDays*24)*time.Hour)),
			width,
		),
		"",
		"Longest-lived stale branch",
		m.theme.Warning.Render(longestBranchLabel(m.snapshot.Branches.StaleBranches)),
	}
	return fitLines(strings.Join(lines, "\n"), height)
}

func (m Model) renderBranchesFocusLists(width, height int) string {
	leftWidth, rightWidth := splitWidths(width, 45)
	lTitle := m.paletteFor("branches").Title.Render("[ Active Queue ]")
	rTitle := m.paletteFor("branches").Title.Render("[ Stale Branches ]")
	header := m.paletteFor("branches").Border.Render("├") +
		lTitle +
		m.paletteFor("branches").Border.Render(strings.Repeat("─", max(0, leftWidth-1-lipgloss.Width(lTitle)))) +
		m.centerDivider("branches", "branches").Render("┬") +
		rTitle +
		m.paletteFor("branches").Border.Render(strings.Repeat("─", max(0, rightWidth-1-lipgloss.Width(rTitle)))+"┤")

	contentHeight := max(1, height-1)
	left := strings.Split(fitLines(m.renderBranchList(m.snapshot.Branches.ActiveBranches, leftWidth-2, contentHeight, false), contentHeight), "\n")
	right := strings.Split(fitLines(m.renderBranchList(m.snapshot.Branches.StaleBranches, rightWidth-2, contentHeight, true), contentHeight), "\n")

	lines := []string{header}
	for idx := 0; idx < contentHeight; idx++ {
		lines = append(lines,
			m.paletteFor("branches").Border.Render("│")+
				padRight(truncate(left[idx], leftWidth-1), leftWidth-1)+
				m.centerDivider("branches", "branches").Render("│")+
				padRight(truncate(right[idx], rightWidth-1), rightWidth-1)+
				m.paletteFor("branches").Border.Render("│"),
		)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderBranchList(branches []aggregator.BranchSummary, width, height int, stale bool) string {
	if len(branches) == 0 {
		return fitLines("none", height)
	}
	maxAge := 1
	for _, branch := range branches {
		if branch.AgeDays > maxAge {
			maxAge = branch.AgeDays
		}
	}
	barWidth := clamp(width/8, 5, 10)
	nameWidth := max(22, width-barWidth-7)
	lines := make([]string, 0, min(len(branches), height))
	for idx, branch := range branches {
		if idx >= height {
			break
		}
		bar := meterBar(maxAge-branch.AgeDays, maxAge, barWidth, m.paletteFor("branches").Bar, m.theme.Muted)
		if stale {
			bar = meterBar(branch.AgeDays, maxAge, barWidth, m.theme.Warning, m.theme.Muted)
		}
		lines = append(lines, fmt.Sprintf("%s %s %4dd", padRight(truncate(branch.Name, nameWidth), nameWidth), bar, branch.AgeDays))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderChurn(width, height int) string {
	adds := m.snapshot.Overview.Additions
	dels := m.snapshot.Overview.Deletions
	totalChange := adds + dels
	barWidth := clamp(width-18, 8, 40)
	sparkW := clamp(width-16, 12, 80)

	lines := []string{
		joinEdge(fmt.Sprintf("Net LOC (%s)  %s", windowLabel(m.currentWindow()), m.theme.Positive.Render(fmt.Sprintf("%+d", m.snapshot.Overview.NetLines))), fmt.Sprintf("Change volume  %d lines", totalChange), width),
		fmt.Sprintf("Adds  %s  %s", meterBar(adds, max(1, totalChange), barWidth, m.theme.Positive, m.theme.Muted), m.theme.Positive.Render(fmt.Sprintf("%d", adds))),
		fmt.Sprintf("Dels  %s  %s", meterBar(dels, max(1, totalChange), barWidth, m.theme.Danger, m.theme.Muted), m.theme.Danger.Render(fmt.Sprintf("%d", dels))),
		joinEdge(fmt.Sprintf("Commits  %d", m.snapshot.Overview.CommitCount), fmt.Sprintf("Authors  %d", m.snapshot.Overview.AuthorCount), width),
		joinEdge(fmt.Sprintf("Current streak  %dd", m.snapshot.Overview.CurrentStreak), fmt.Sprintf("Longest  %dd", m.snapshot.Overview.LongestStreak), width),
		joinEdge(fmt.Sprintf("Commit quality %s", m.colorizePercent(m.snapshot.Overview.ConventionalCommitShare)), "", width),
		"Commit Types  " + renderBreakdownLine(m.snapshot.Overview.ConventionalBreakdown, max(10, width-14)),
		"Net Trend     " + m.paletteFor("churn").Bar.Render(sparkline(dateValuesToInts(m.snapshot.Commits.Daily), sparkW)),
		"Commit Rhythm " + sparkline(namedValuesToInts(m.snapshot.Commits.Hourly), sparkW),
		"Weekday Load  " + sparkline(namedValuesToInts(m.snapshot.Commits.Weekday), clamp(width-16, 7, 40)),
	}
	return strings.Join(trimLines(lines, height), "\n")
}

func (m Model) renderFooter(width int) string {
	keys := m.theme.Key.Render("tab") + m.theme.Muted.Render(" panel focus   ") + m.theme.Key.Render("1-6") + m.theme.Muted.Render(" jump   ") + m.theme.Key.Render("t") + m.theme.Muted.Render(" time range   ") + m.theme.Key.Render("r") + m.theme.Muted.Render(" refresh   ") + m.theme.Key.Render("q") + m.theme.Muted.Render(" quit ")
	if m.compactMode() {
		keys = m.theme.Key.Render("tab/1-6") + m.theme.Muted.Render(" switch panel   ") + m.theme.Key.Render("t") + m.theme.Muted.Render(" time range   ") + m.theme.Key.Render("r") + m.theme.Muted.Render(" refresh   ") + m.theme.Key.Render("q") + m.theme.Muted.Render(" quit ")
	}

	border := m.theme.Rule.Render(strings.Repeat("─", width))
	if !m.compactMode() {
		border = m.renderSplitBottomBorder(width, "branches", "churn")
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		border,
		padRight(truncate(joinEdge(keys, m.theme.Status.Render(truncate(m.status, clamp(width/2, 18, width-6))), width), width), width),
	)
}

func (m Model) repositoryName() string {
	if fullName := m.remote.FullName(); fullName != "" {
		return fullName
	}
	repoPath := m.snapshot.Repository.Path
	if repoPath == "" {
		repoPath = m.cfg.RepoPath
	}
	if repoPath == "." {
		if absPath, err := filepath.Abs(repoPath); err == nil {
			repoPath = absPath
		}
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
			parts = append(parts, theme.TabActive.Render(label))
		} else {
			parts = append(parts, theme.Tab.Render(label))
		}
	}
	return strings.Join(parts, "")
}

func renderWeekHeatmap(values []aggregator.DateValue, rows int, palette panelPalette) []string {
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
			cells = append(cells, coloredHeatLevel(value, maxValue, palette))
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

func coloredHeatLevel(value, maxValue int, palette panelPalette) string {
	levels := []string{"·", "▁", "▃", "▅", "▇", "█"}
	if value <= 0 || maxValue <= 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#4f6a69")).Render(levels[0])
	}
	idx := int(math.Round(float64(value) / float64(maxValue) * float64(len(levels)-1)))
	if idx < 1 {
		idx = 1
	}
	if idx >= len(levels) {
		idx = len(levels) - 1
	}
	if idx <= 2 {
		return palette.Bar.Copy().Faint(true).Render(levels[idx])
	}
	return palette.Bar.Render(levels[idx])
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

func renderDateAxis(daily []aggregator.DateValue, chartWidth int) string {
	axisWidth := chartWidth - 5 // match the chart's axis offset
	if len(daily) == 0 || axisWidth <= 0 {
		return ""
	}

	// Pick ~6 evenly-spaced date labels
	numLabels := clamp(axisWidth/16, 3, 8)
	axis := strings.Repeat(" ", 4) // indent to align with chart body (past "NNN│")

	positions := make([]int, numLabels)
	for i := range positions {
		positions[i] = i * (axisWidth - 6) / max(1, numLabels-1)
	}

	cursor := 0
	for i, pos := range positions {
		// Pick the date corresponding to this position
		dateIdx := pos * max(1, len(daily)-1) / max(1, axisWidth-1)
		if dateIdx >= len(daily) {
			dateIdx = len(daily) - 1
		}
		label := daily[dateIdx].Date.Format("Jan 02")
		if pos > cursor {
			axis += strings.Repeat(" ", pos-cursor)
			cursor = pos
		}
		if i == numLabels-1 {
			// Last label: right-align so it doesn't overflow
			remaining := axisWidth - cursor
			if remaining > len(label) {
				axis += strings.Repeat(" ", remaining-len(label))
			}
			axis += label
		} else {
			axis += label
			cursor += len(label)
		}
	}
	return axis
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
	barWidth := clamp(width/(max(1, len(values))*3), 2, 12)
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%s %s", value.Name, progressBar(value.Value, maxValue, barWidth)))
	}
	return truncate(strings.Join(parts, "  "), width)
}

func renderFramedSection(width, height int, title, body string, palette panelPalette, focused bool) string {
	contentHeight := max(1, height-2)
	lines := strings.Split(fitLines(body, contentHeight), "\n")

	rawTitle := "[ " + title + " ]"
	if focused {
		rawTitle = "[ ▸ " + title + " ]"
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

func (m Model) renderSplitBottomBorder(width int, leftKey, rightKey string) string {
	leftWidth, rightWidth := splitWidths(width, 46)
	return m.paletteFor(leftKey).Border.Render("└"+strings.Repeat("─", max(0, leftWidth-1))) +
		m.centerDivider(leftKey, rightKey).Render("┴") +
		m.paletteFor(rightKey).Border.Render(strings.Repeat("─", max(0, rightWidth-1))+"┘")
}

func (m Model) renderSplitRow(
	width int,
	height int,
	leftPercent int,
	top bool,
	leftKey string,
	leftTitle string,
	leftRenderer func(int, int) string,
	rightKey string,
	rightTitle string,
	rightRenderer func(int, int) string,
) string {
	leftWidth, rightWidth := splitWidths(width, leftPercent)
	contentHeight := max(1, height-1)

	leftBody := strings.Split(fitLines(leftRenderer(leftWidth-2, contentHeight), contentHeight), "\n")
	rightBody := strings.Split(fitLines(rightRenderer(rightWidth-2, contentHeight), contentHeight), "\n")

	header := m.renderSplitBorder(top, leftWidth, rightWidth, leftKey, leftTitle, rightKey, rightTitle)
	lines := []string{header}
	for idx := 0; idx < contentHeight; idx++ {
		leftLine := padRight(truncate(leftBody[idx], leftWidth-1), leftWidth-1)
		rightLine := padRight(truncate(rightBody[idx], rightWidth-1), rightWidth-1)
		lines = append(lines,
			m.paletteFor(leftKey).Border.Render("│")+
				leftLine+
				m.centerDivider(leftKey, rightKey).Render("│")+
				rightLine+
				m.paletteFor(rightKey).Border.Render("│"),
		)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderSplitBorder(top bool, leftWidth, rightWidth int, leftKey, leftTitle, rightKey, rightTitle string) string {
	leftPalette := m.paletteFor(leftKey)
	rightPalette := m.paletteFor(rightKey)
	leftLabel := m.panelTitleLabel(leftKey, leftTitle)
	rightLabel := m.panelTitleLabel(rightKey, rightTitle)

	leftRendered := leftPalette.Title.Render(leftLabel)
	rightRendered := rightPalette.Title.Render(rightLabel)
	leftFill := max(0, leftWidth-1-lipgloss.Width(leftRendered))
	rightFill := max(0, rightWidth-1-lipgloss.Width(rightRendered))

	leftEdge, center, rightEdge := "├", "┼", "┤"
	if top {
		leftEdge, center, rightEdge = "┌", "┬", "┐"
	}

	return leftPalette.Border.Render(leftEdge) +
		leftRendered +
		leftPalette.Border.Render(strings.Repeat("─", leftFill)) +
		m.centerDivider(leftKey, rightKey).Render(center) +
		rightRendered +
		rightPalette.Border.Render(strings.Repeat("─", rightFill)+rightEdge)
}

func (m Model) centerDivider(leftKey, rightKey string) lipgloss.Style {
	if panelOrder[m.focused] == leftKey || panelOrder[m.focused] == rightKey {
		return m.theme.Accent
	}
	return m.theme.Rule
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
	row2 := max(8, total*34/100)
	row3 := total - row1 - row2
	if row3 < 7 {
		row3 = 7
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
		row1++
	}
	return row1, row2, row3
}

func splitWidths(total, leftPercent int) (int, int) {
	if total <= 3 {
		return 1, 1
	}
	if leftPercent < 35 {
		leftPercent = 35
	}
	if leftPercent > 65 {
		leftPercent = 65
	}

	left := total * leftPercent / 100
	right := total - left - 1

	if left < 28 {
		left = 28
		right = total - left - 1
	}
	if right < 24 {
		right = 24
		left = total - right - 1
	}
	if left < 1 {
		left = 1
	}
	if right < 1 {
		right = 1
	}
	return left, right
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

func ageLabel(ts time.Time) string {
	if ts.IsZero() {
		return "--"
	}
	age := int(time.Since(ts).Hours() / 24)
	if age < 0 {
		age = 0
	}
	return fmt.Sprintf("%dd", age)
}

func detailTitle(panel string) string {
	switch panel {
	case "velocity":
		return "Commit Velocity"
	case "branches":
		return "Branch Health"
	default:
		return "Detail"
	}
}

func longestBranchLabel(branches []aggregator.BranchSummary) string {
	if len(branches) == 0 {
		return "none"
	}
	longest := branches[0]
	for _, branch := range branches[1:] {
		if branch.AgeDays > longest.AgeDays {
			longest = branch
		}
	}
	return fmt.Sprintf("%s  (%dd old)", longest.Name, longest.AgeDays)
}

func maxNamedValueEntry(values []aggregator.NamedValue) (string, int) {
	if len(values) == 0 {
		return "n/a", 0
	}
	best := values[0]
	for _, value := range values[1:] {
		if value.Value > best.Value {
			best = value
		}
	}
	return best.Name, best.Value
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
			Border: lipgloss.NewStyle().Foreground(lipgloss.Color("#8bd5a0")),
			Title:  lipgloss.NewStyle().Foreground(lipgloss.Color("#8bd5a0")).Bold(true),
			Bar:    lipgloss.NewStyle().Foreground(lipgloss.Color("#8bd5a0")),
		}
	case "authors":
		return panelPalette{
			Border: lipgloss.NewStyle().Foreground(lipgloss.Color("#eed49f")),
			Title:  lipgloss.NewStyle().Foreground(lipgloss.Color("#eed49f")).Bold(true),
			Bar:    lipgloss.NewStyle().Foreground(lipgloss.Color("#eed49f")),
		}
	case "files":
		return panelPalette{
			Border: lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c17a")),
			Title:  lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c17a")).Bold(true),
			Bar:    lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c17a")),
		}
	case "prs":
		return panelPalette{
			Border: lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")),
			Title:  lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).Bold(true),
			Bar:    lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")),
		}
	case "branches":
		return panelPalette{
			Border: lipgloss.NewStyle().Foreground(lipgloss.Color("#8aadf4")),
			Title:  lipgloss.NewStyle().Foreground(lipgloss.Color("#8aadf4")).Bold(true),
			Bar:    lipgloss.NewStyle().Foreground(lipgloss.Color("#8aadf4")),
		}
	default:
		return panelPalette{
			Border: lipgloss.NewStyle().Foreground(lipgloss.Color("#7dc4e4")),
			Title:  lipgloss.NewStyle().Foreground(lipgloss.Color("#7dc4e4")).Bold(true),
			Bar:    lipgloss.NewStyle().Foreground(lipgloss.Color("#7dc4e4")),
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
