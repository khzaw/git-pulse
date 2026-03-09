package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"

	"git-pulse/internal/aggregator"
	"git-pulse/internal/config"
	"git-pulse/internal/remote"
)

func TestResolveTheme(t *testing.T) {
	t.Parallel()

	theme, err := ResolveTheme("tokyo-night")
	require.NoError(t, err)
	require.NotZero(t, theme.Frame)
}

func TestResolveThemeIgnoresNamedThemes(t *testing.T) {
	t.Parallel()

	_, err := ResolveTheme("missing")
	require.NoError(t, err)
}

func TestViewIncludesLoadedPanels(t *testing.T) {
	t.Parallel()

	model, err := NewModel(config.Default())
	require.NoError(t, err)
	model.width = 170
	model.height = 42
	model.loading = false
	model.snapshot = aggregator.Snapshot{
		Repository: aggregator.RepositorySummary{Path: "/tmp/repo"},
		Overview: aggregator.Overview{
			CommitCount:             12,
			AuthorCount:             3,
			Additions:               20,
			Deletions:               8,
			CurrentStreak:           4,
			LongestStreak:           6,
			ConventionalCommitShare: 0.75,
			ConventionalBreakdown:   []aggregator.NamedValue{{Name: "feat", Value: 6}},
		},
		Commits: aggregator.CommitActivity{
			Daily:   []aggregator.DateValue{{Date: time.Now(), Value: 1}},
			Weekly:  []aggregator.DateValue{{Date: time.Now(), Value: 3}},
			Weekday: []aggregator.NamedValue{{Name: "Mon", Value: 1}},
		},
		Authors: aggregator.AuthorActivity{
			ActiveThisWeek:  2,
			ActiveLastWeek:  1,
			ActiveThisMonth: 3,
			BusFactor:       1,
			Leaderboard:     []aggregator.AuthorSummary{{Name: "Ada", Commits: 5}},
		},
		Files: aggregator.FileActivity{
			Hotspots:    []aggregator.FileSummary{{Path: "internal/tui/app.go", Touches: 3}},
			Directories: []aggregator.DirectorySummary{{Path: "internal", Churn: 12, Touches: 3}},
		},
		Branches: aggregator.BranchActivity{
			ActiveBranches:     []aggregator.BranchSummary{{Name: "main", AgeDays: 1}},
			ReleaseCadenceDays: 7,
			LastTag:            "v0.2.0",
		},
	}
	model.remote = remote.RepositoryRef{Provider: remote.ProviderGitHub, Owner: "acme", Name: "git-pulse"}
	model.prs = remote.PRSnapshot{
		Repository: "acme/git-pulse",
		Windows: []remote.WindowMetric{
			{Label: "7d", MergedCount: 2},
		},
	}

	view := model.View()
	require.Contains(t, view, "git-pulse")
	require.Contains(t, view, "Commit Velocity")
	require.Contains(t, view, "Authors Active")
	require.Contains(t, view, "File Hotspots")
	require.Contains(t, view, "PR Cycle Time")
	require.Contains(t, view, "Branch Health")
	require.Contains(t, view, "Code Churn")
	require.Contains(t, view, "acme/git-pulse")
	require.Contains(t, view, "┬")
	require.Contains(t, view, "┴")
}

func TestCompactModeShowsFocusedPanelOnly(t *testing.T) {
	t.Parallel()

	model, err := NewModel(config.Default())
	require.NoError(t, err)
	model.width = 80
	model.height = 24
	model.loading = false
	model.focused = 2
	model.snapshot = aggregator.Snapshot{
		Authors: aggregator.AuthorActivity{
			Leaderboard: []aggregator.AuthorSummary{{Name: "Ada", Commits: 5}},
		},
	}

	view := model.View()
	require.Contains(t, view, "panel 3/6")
	require.Contains(t, view, "File Hotspots")
	require.NotContains(t, view, "Authors Active")
}

func TestViewFillsTerminalHeight(t *testing.T) {
	t.Parallel()

	model, err := NewModel(config.Default())
	require.NoError(t, err)
	model.width = 120
	model.height = 40
	model.loading = false
	model.snapshot = aggregator.Snapshot{}

	view := model.View()
	require.Equal(t, 40, len(strings.Split(view, "\n")))
}

func TestPressingOneOpensVelocityDetailAndEscReturns(t *testing.T) {
	t.Parallel()

	model, err := NewModel(config.Default())
	require.NoError(t, err)
	model.width = 150
	model.height = 42
	model.loading = false

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	detailModel := updated.(Model)
	require.Equal(t, "velocity", detailModel.detailPanel)
	require.Contains(t, detailModel.View(), "back to dashboard")
	require.Contains(t, detailModel.View(), "COMMITS PER DAY")

	updated, _ = detailModel.Update(tea.KeyMsg{Type: tea.KeyEsc})
	dashboardModel := updated.(Model)
	require.Equal(t, "", dashboardModel.detailPanel)
}

func TestPressingFiveOpensBranchDetailAndEscReturns(t *testing.T) {
	t.Parallel()

	model, err := NewModel(config.Default())
	require.NoError(t, err)
	model.width = 150
	model.height = 42
	model.loading = false
	model.snapshot = aggregator.Snapshot{
		Branches: aggregator.BranchActivity{
			ActiveBranches: []aggregator.BranchSummary{
				{Name: "master", AgeDays: 6},
			},
			StaleBranches: []aggregator.BranchSummary{
				{Name: "release_PABLO.24.31-fixup", AgeDays: 592},
			},
		},
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	detailModel := updated.(Model)
	require.Equal(t, "branches", detailModel.detailPanel)
	require.Contains(t, detailModel.View(), "BRANCH HEALTH")
	require.Contains(t, detailModel.View(), "Active Queue")
	require.Contains(t, detailModel.View(), "Stale Branches")

	updated, _ = detailModel.Update(tea.KeyMsg{Type: tea.KeyEsc})
	dashboardModel := updated.(Model)
	require.Equal(t, "", dashboardModel.detailPanel)
}

func TestRenderFilesPrefersPathVisibility(t *testing.T) {
	t.Parallel()

	model, err := NewModel(config.Default())
	require.NoError(t, err)
	model.snapshot = aggregator.Snapshot{
		Files: aggregator.FileActivity{
			Hotspots: []aggregator.FileSummary{
				{
					Path:       "pkg/alfred/purchase/service/internal/handler.go",
					Touches:    8,
					Additions:  80,
					Deletions:  11,
					LastChange: time.Now(),
				},
			},
			Directories: []aggregator.DirectorySummary{
				{Path: "pkg/alfred/purchase/service", Churn: 91, Touches: 8},
			},
		},
	}

	view := model.renderFiles(64, 12)
	require.Contains(t, view, "pkg/alfred/purchase/service/")
	require.Contains(t, view, "hits churn age")
}

func TestRenderBranchesPrefersBranchVisibility(t *testing.T) {
	t.Parallel()

	model, err := NewModel(config.Default())
	require.NoError(t, err)
	model.snapshot = aggregator.Snapshot{
		Branches: aggregator.BranchActivity{
			ActiveBranches: []aggregator.BranchSummary{
				{Name: "release_PABLO.24.31-deliveryhero-hotfix-payment-routing", AgeDays: 6},
			},
			StaleBranches: []aggregator.BranchSummary{
				{Name: "PYMNTINT-26-generic-payment-intent-cleanup-and-follow-up", AgeDays: 1021},
			},
		},
	}

	view := model.renderBranches(68, 12)
	require.Contains(t, view, "release_PABLO.24.31")
	require.Contains(t, view, "PYMNTINT-26-generic")
}

func TestRenderVelocityUsesMultiLineChart(t *testing.T) {
	t.Parallel()

	model, err := NewModel(config.Default())
	require.NoError(t, err)
	now := time.Now()
	model.snapshot = aggregator.Snapshot{
		Overview: aggregator.Overview{
			CurrentStreak: 2,
			LongestStreak: 4,
		},
		Commits: aggregator.CommitActivity{
			Daily: []aggregator.DateValue{
				{Date: now.AddDate(0, 0, -5), Value: 1},
				{Date: now.AddDate(0, 0, -4), Value: 3},
				{Date: now.AddDate(0, 0, -3), Value: 2},
				{Date: now.AddDate(0, 0, -2), Value: 6},
				{Date: now.AddDate(0, 0, -1), Value: 4},
				{Date: now, Value: 5},
			},
			Weekly: []aggregator.DateValue{
				{Date: now.AddDate(0, 0, -21), Value: 8},
				{Date: now.AddDate(0, 0, -14), Value: 11},
				{Date: now.AddDate(0, 0, -7), Value: 14},
			},
			Weekday: []aggregator.NamedValue{
				{Name: "Mon", Value: 3}, {Name: "Tue", Value: 2}, {Name: "Wed", Value: 5}, {Name: "Thu", Value: 4},
			},
			Hourly: []aggregator.NamedValue{
				{Name: "09", Value: 1}, {Name: "10", Value: 3}, {Name: "11", Value: 5}, {Name: "12", Value: 2},
			},
		},
	}

	view := model.renderVelocity(70, 18)
	require.Contains(t, view, "Weekly")
	require.Contains(t, view, "Day-of-Week Heatmap")
	require.GreaterOrEqual(t, strings.Count(view, "█"), 4)
}

func TestRenderVelocityDetailIncludesBreakdownSections(t *testing.T) {
	t.Parallel()

	model, err := NewModel(config.Default())
	require.NoError(t, err)
	now := time.Now()
	model.snapshot = aggregator.Snapshot{
		Overview: aggregator.Overview{
			CurrentStreak: 2,
			LongestStreak: 4,
		},
		Commits: aggregator.CommitActivity{
			Daily: []aggregator.DateValue{
				{Date: now.AddDate(0, 0, -5), Value: 1},
				{Date: now.AddDate(0, 0, -4), Value: 3},
				{Date: now.AddDate(0, 0, -3), Value: 2},
				{Date: now.AddDate(0, 0, -2), Value: 6},
				{Date: now.AddDate(0, 0, -1), Value: 4},
				{Date: now, Value: 5},
			},
			Weekly: []aggregator.DateValue{
				{Date: now.AddDate(0, 0, -21), Value: 8},
				{Date: now.AddDate(0, 0, -14), Value: 11},
				{Date: now.AddDate(0, 0, -7), Value: 14},
			},
			Weekday: []aggregator.NamedValue{
				{Name: "Mon", Value: 3}, {Name: "Tue", Value: 2}, {Name: "Wed", Value: 5}, {Name: "Thu", Value: 4},
			},
			Hourly: []aggregator.NamedValue{
				{Name: "09", Value: 1}, {Name: "10", Value: 3}, {Name: "11", Value: 5}, {Name: "12", Value: 2},
			},
		},
	}

	view := model.renderVelocityDetail(120, 28)
	require.Contains(t, view, "COMMITS PER DAY")
	require.Contains(t, view, "Hour Of Day")
	require.Contains(t, view, "Weekly Summary")
}

func TestRenderBranchesDetailIncludesBranchTables(t *testing.T) {
	t.Parallel()

	model, err := NewModel(config.Default())
	require.NoError(t, err)
	model.snapshot = aggregator.Snapshot{
		Branches: aggregator.BranchActivity{
			ActiveBranches: []aggregator.BranchSummary{
				{Name: "master", AgeDays: 6},
				{Name: "feature/search-index-upgrade", AgeDays: 2},
			},
			StaleBranches: []aggregator.BranchSummary{
				{Name: "release_PABLO.24.31-fixup", AgeDays: 592},
			},
			LastTag:            "delop-549-db-migration-gdp-03",
			ReleaseCadenceDays: 12,
		},
	}

	view := model.renderBranchesDetail(120, 24)
	require.Contains(t, view, "Longest-lived stale branch")
	require.Contains(t, view, "Active Queue")
	require.Contains(t, view, "Stale Branches")
	require.Contains(t, view, "feature/search-index-upgrade")
	require.Contains(t, view, "release_PABLO.24.31-fixup")
}

func TestSplitWidthsAllowsAsymmetricRows(t *testing.T) {
	t.Parallel()

	left, right := splitWidths(120, 56)
	require.Equal(t, 67, left)
	require.Equal(t, 52, right)

	left, right = splitWidths(120, 46)
	require.Equal(t, 55, left)
	require.Equal(t, 64, right)
}

func TestTruncatePreservesStyledTextWidth(t *testing.T) {
	t.Parallel()

	value := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render("abcdefghijk")
	truncated := truncate(value, 8)

	require.Equal(t, 8, lipgloss.Width(truncated))
	require.NotContains(t, truncated, "�")
}

func TestViewDoesNotEmitCorruptedReplacementGlyphsForStyledLayout(t *testing.T) {
	t.Parallel()

	model, err := NewModel(config.Default())
	require.NoError(t, err)
	model.width = 170
	model.height = 42
	model.loading = false
	model.snapshot = aggregator.Snapshot{
		Repository: aggregator.RepositorySummary{Path: "/tmp/repo", DefaultBranch: "main"},
		Overview: aggregator.Overview{
			CommitCount:             15,
			AuthorCount:             1,
			Additions:               4956,
			Deletions:               487,
			CurrentStreak:           1,
			LongestStreak:           15,
			ConventionalCommitShare: 0,
		},
		Commits: aggregator.CommitActivity{
			Daily:   []aggregator.DateValue{{Date: time.Now(), Value: 15}},
			Weekly:  []aggregator.DateValue{{Date: time.Now(), Value: 15}},
			Weekday: []aggregator.NamedValue{{Name: "Sun", Value: 15}},
			Hourly:  []aggregator.NamedValue{{Name: "18", Value: 9}},
		},
		Authors: aggregator.AuthorActivity{
			ActiveThisWeek:  1,
			ActiveThisMonth: 1,
			BusFactor:       1,
			Leaderboard: []aggregator.AuthorSummary{
				{Name: "Kaung Htet", Commits: 15, Additions: 5443},
			},
			NewContributors: []aggregator.AuthorSummary{{Name: "Kaung Htet"}},
		},
		Files: aggregator.FileActivity{
			Hotspots: []aggregator.FileSummary{
				{Path: "internal/tui/app.go", Touches: 8, Additions: 91, LastChange: time.Now()},
				{Path: "internal/tui/app_test.go", Touches: 7, Additions: 84, LastChange: time.Now()},
			},
			Directories: []aggregator.DirectorySummary{{Path: "internal", Churn: 175, Touches: 15}},
		},
		Branches: aggregator.BranchActivity{
			ActiveBranches: []aggregator.BranchSummary{{Name: "master", AgeDays: 0}},
		},
	}

	view := model.View()
	require.NotContains(t, view, "�")
}
