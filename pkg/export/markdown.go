package export

import (
	"fmt"
	"strings"

	"git-pulse/internal/dashboard"
)

func Markdown(result dashboard.Result) string {
	lines := []string{
		"# git-pulse snapshot",
		"",
		fmt.Sprintf("- Repository: `%s`", result.Snapshot.Repository.Path),
		fmt.Sprintf("- Window: `%s`", result.Snapshot.Window),
		fmt.Sprintf("- Commits: `%d`", result.Snapshot.Overview.CommitCount),
		fmt.Sprintf("- Authors: `%d`", result.Snapshot.Overview.AuthorCount),
		fmt.Sprintf("- Net lines: `%d`", result.Snapshot.Overview.NetLines),
		fmt.Sprintf("- Conventional commits: `%.0f%%`", result.Snapshot.Overview.ConventionalCommitShare*100),
		"",
		"## Top authors",
	}

	for _, author := range result.Snapshot.Authors.Leaderboard {
		lines = append(lines, fmt.Sprintf("- %s: %d commits (+%d/-%d)", author.Name, author.Commits, author.Additions, author.Deletions))
	}

	lines = append(lines, "", "## File hotspots")
	for _, file := range result.Snapshot.Files.Hotspots {
		lines = append(lines, fmt.Sprintf("- %s: %d touches (+%d/-%d)", file.Path, file.Touches, file.Additions, file.Deletions))
	}

	if result.PRs.Repository != "" {
		lines = append(lines, "", "## Pull requests")
		for _, window := range result.PRs.Windows {
			lines = append(lines, fmt.Sprintf("- %s: %d merged, median cycle %s", window.Label, window.MergedCount, window.MedianCycleTime))
		}
	}

	if result.Warning != "" {
		lines = append(lines, "", fmt.Sprintf("> %s", result.Warning))
	}

	return strings.Join(lines, "\n")
}
