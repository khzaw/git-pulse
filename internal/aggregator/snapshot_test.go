package aggregator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git-pulse/internal/git"
)

func TestAggregateBuildsWindowedSnapshot(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	data := git.RepositoryData{
		Path:          "/tmp/repo",
		DefaultBranch: "main",
		Head:          "abc123",
		Commits: []git.CommitRecord{
			{
				Hash:             "1",
				AuthorName:       "Ada",
				AuthorEmail:      "ada@example.com",
				When:             now.AddDate(0, 0, -2),
				Subject:          "feat: dashboard",
				ConventionalType: "feat",
				Additions:        12,
				Deletions:        3,
				Files: []git.FileStat{
					{Path: "internal/tui/app.go", Additions: 12, Deletions: 3},
				},
			},
			{
				Hash:             "2",
				AuthorName:       "Bob",
				AuthorEmail:      "bob@example.com",
				When:             now.AddDate(0, 0, -1),
				Subject:          "fix: sorting",
				ConventionalType: "fix",
				Additions:        5,
				Deletions:        1,
				Files: []git.FileStat{
					{Path: "internal/aggregator/snapshot.go", Additions: 5, Deletions: 1},
					{Path: "README.md", Additions: 1, Deletions: 0},
				},
			},
			{
				Hash:        "3",
				AuthorName:  "Ada",
				AuthorEmail: "ada@example.com",
				When:        now.AddDate(0, 0, -45),
				Subject:     "legacy commit",
				Additions:   2,
				Deletions:   4,
				Files: []git.FileStat{
					{Path: "docs/notes.md", Additions: 2, Deletions: 4},
				},
			},
		},
		Branches: []git.BranchRecord{
			{Name: "main", LastCommitAt: now.AddDate(0, 0, -1)},
			{Name: "feature/old", LastCommitAt: now.AddDate(0, 0, -32)},
		},
		Tags: []git.TagRecord{
			{Name: "v0.1.0", When: now.AddDate(0, 0, -60)},
			{Name: "v0.2.0", When: now.AddDate(0, 0, -15)},
		},
	}

	snapshot := Aggregate(data, Options{Now: now, Window: Window30Days})

	require.Equal(t, 2, snapshot.Overview.CommitCount)
	require.Equal(t, 2, snapshot.Overview.AuthorCount)
	require.Equal(t, 17, snapshot.Overview.Additions)
	require.Equal(t, 4, snapshot.Overview.Deletions)
	require.Equal(t, 13, snapshot.Overview.NetLines)
	require.Equal(t, 2, snapshot.Overview.CurrentStreak)
	require.Equal(t, 2, snapshot.Overview.LongestStreak)
	require.InDelta(t, 1.0, snapshot.Overview.ConventionalCommitShare, 0.001)

	require.Len(t, snapshot.Commits.Daily, 2)
	require.Equal(t, 2, snapshot.Authors.ActiveThisWeek)
	require.Equal(t, 0, snapshot.Authors.ActiveLastWeek)
	require.Equal(t, 2, snapshot.Authors.ActiveThisMonth)
	require.Equal(t, 1, snapshot.Authors.BusFactor)
	require.Len(t, snapshot.Files.Hotspots, 3)
	require.Equal(t, "internal", snapshot.Files.Directories[0].Path)
	require.Len(t, snapshot.Branches.ActiveBranches, 1)
	require.Len(t, snapshot.Branches.StaleBranches, 1)
	require.Equal(t, "v0.2.0", snapshot.Branches.LastTag)
	require.InDelta(t, 45.0, snapshot.Branches.ReleaseCadenceDays, 0.001)
}

func TestAggregateAllWindowIncludesOldCommits(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	data := git.RepositoryData{
		Commits: []git.CommitRecord{
			{AuthorEmail: "ada@example.com", When: now.AddDate(0, 0, -60)},
			{AuthorEmail: "bob@example.com", When: now.AddDate(0, 0, -1)},
		},
	}

	snapshot := Aggregate(data, Options{Now: now, Window: WindowAll})
	require.Equal(t, 2, snapshot.Overview.CommitCount)
	require.Equal(t, 2, snapshot.Overview.AuthorCount)
}
