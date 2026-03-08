package export

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git-pulse/internal/aggregator"
	"git-pulse/internal/dashboard"
	"git-pulse/internal/remote"
)

func TestJSONExport(t *testing.T) {
	t.Parallel()

	payload, err := JSON(sampleResult())
	require.NoError(t, err)
	require.Contains(t, string(payload), "\"snapshot\"")
	require.Contains(t, string(payload), "\"prs\"")
}

func TestMarkdownExport(t *testing.T) {
	t.Parallel()

	payload := Markdown(sampleResult())
	require.Contains(t, payload, "# git-pulse snapshot")
	require.Contains(t, payload, "Top authors")
	require.Contains(t, payload, "Pull requests")
}

func TestCSVExport(t *testing.T) {
	t.Parallel()

	payload, err := CSV(sampleResult())
	require.NoError(t, err)
	require.Contains(t, payload, "metric,value")
	require.Contains(t, payload, "commits,12")
}

func sampleResult() dashboard.Result {
	return dashboard.Result{
		Snapshot: aggregator.Snapshot{
			Window: aggregator.Window30Days,
			Repository: aggregator.RepositorySummary{
				Path: "/tmp/repo",
			},
			Overview: aggregator.Overview{
				CommitCount:   12,
				AuthorCount:   3,
				NetLines:      40,
				CurrentStreak: 4,
				LongestStreak: 7,
			},
			Authors: aggregator.AuthorActivity{
				Leaderboard: []aggregator.AuthorSummary{
					{Name: "Ada", Commits: 6, Additions: 30, Deletions: 8},
				},
			},
			Files: aggregator.FileActivity{
				Hotspots: []aggregator.FileSummary{
					{Path: "internal/tui/app.go", Touches: 4, Additions: 20, Deletions: 5},
				},
			},
		},
		PRs: remote.PRSnapshot{
			Repository: "acme/git-pulse",
			Windows: []remote.WindowMetric{
				{Label: "30d", MergedCount: 5},
			},
		},
	}
}
