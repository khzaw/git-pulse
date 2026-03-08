package tui

import (
	"testing"
	"time"

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
	require.Contains(t, view, "COMMIT VELOCITY")
	require.Contains(t, view, "AUTHORS ACTIVE")
	require.Contains(t, view, "FILE HOTSPOTS")
	require.Contains(t, view, "PR CYCLE TIME")
	require.Contains(t, view, "BRANCH HEALTH")
	require.Contains(t, view, "CODE CHURN")
	require.Contains(t, view, "acme/git-pulse")
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
	require.Contains(t, view, "FILE HOTSPOTS")
	require.NotContains(t, view, "AUTHORS ACTIVE")
}
