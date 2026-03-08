package aggregator

import (
	"sort"
	"strings"
	"time"

	"git-pulse/internal/git"
)

type TimeWindow string

const (
	Window7Days  TimeWindow = "7d"
	Window30Days TimeWindow = "30d"
	Window90Days TimeWindow = "90d"
	Window1Year  TimeWindow = "1y"
	WindowAll    TimeWindow = "all"
)

type Options struct {
	Now    time.Time
	Window TimeWindow
}

type Snapshot struct {
	GeneratedAt time.Time
	Window      TimeWindow
	Repository  RepositorySummary
	Overview    Overview
	Commits     CommitActivity
	Authors     AuthorActivity
	Files       FileActivity
	Branches    BranchActivity
}

type RepositorySummary struct {
	Path          string
	DefaultBranch string
	Head          string
}

type Overview struct {
	CommitCount             int
	AuthorCount             int
	Additions               int
	Deletions               int
	NetLines                int
	CurrentStreak           int
	LongestStreak           int
	ConventionalCommitShare float64
	ConventionalBreakdown   []NamedValue
}

type CommitActivity struct {
	Daily   []DateValue
	Weekly  []DateValue
	Weekday []NamedValue
	Hourly  []NamedValue
}

type AuthorActivity struct {
	ActiveThisWeek  int
	ActiveLastWeek  int
	ActiveThisMonth int
	NewContributors []AuthorSummary
	Leaderboard     []AuthorSummary
	BusFactor       int
}

type FileActivity struct {
	Hotspots    []FileSummary
	Directories []DirectorySummary
}

type BranchActivity struct {
	ActiveBranches     []BranchSummary
	StaleBranches      []BranchSummary
	ReleaseCadenceDays float64
	LastTag            string
}

type NamedValue struct {
	Name  string
	Value int
}

type DateValue struct {
	Date  time.Time
	Value int
}

type AuthorSummary struct {
	Name      string
	Email     string
	Commits   int
	Additions int
	Deletions int
	FirstSeen time.Time
	LastSeen  time.Time
}

type FileSummary struct {
	Path       string
	Touches    int
	Additions  int
	Deletions  int
	LastChange time.Time
}

type DirectorySummary struct {
	Path    string
	Touches int
	Churn   int
}

type BranchSummary struct {
	Name         string
	LastCommitAt time.Time
	AgeDays      int
}

func Aggregate(data git.RepositoryData, opts Options) Snapshot {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	filtered := filterCommits(data.Commits, now, opts.Window)
	weekCommits := filterCommits(data.Commits, now, Window7Days)
	lastWeekCommits := filterCommitsRange(data.Commits, now.AddDate(0, 0, -14), now.AddDate(0, 0, -7))
	monthCommits := filterCommits(data.Commits, now, Window30Days)

	return Snapshot{
		GeneratedAt: now,
		Window:      normalizedWindow(opts.Window),
		Repository: RepositorySummary{
			Path:          data.Path,
			DefaultBranch: data.DefaultBranch,
			Head:          data.Head,
		},
		Overview: buildOverview(filtered),
		Commits:  buildCommitActivity(filtered),
		Authors:  buildAuthorActivity(data.Commits, filtered, weekCommits, lastWeekCommits, monthCommits, now),
		Files:    buildFileActivity(filtered),
		Branches: buildBranchActivity(data, now),
	}
}

func normalizedWindow(window TimeWindow) TimeWindow {
	switch window {
	case Window7Days, Window30Days, Window90Days, Window1Year, WindowAll:
		return window
	default:
		return Window30Days
	}
}

func filterCommits(commits []git.CommitRecord, now time.Time, window TimeWindow) []git.CommitRecord {
	window = normalizedWindow(window)
	if window == WindowAll {
		return append([]git.CommitRecord(nil), commits...)
	}

	var cutoff time.Time
	switch window {
	case Window7Days:
		cutoff = now.AddDate(0, 0, -7)
	case Window30Days:
		cutoff = now.AddDate(0, 0, -30)
	case Window90Days:
		cutoff = now.AddDate(0, 0, -90)
	case Window1Year:
		cutoff = now.AddDate(-1, 0, 0)
	}

	filtered := make([]git.CommitRecord, 0, len(commits))
	for _, commit := range commits {
		if !commit.When.Before(cutoff) {
			filtered = append(filtered, commit)
		}
	}
	return filtered
}

func buildOverview(commits []git.CommitRecord) Overview {
	authors := map[string]struct{}{}
	days := map[string]int{}
	breakdown := map[string]int{}
	netLines := 0
	additions := 0
	deletions := 0
	conventional := 0

	for _, commit := range commits {
		authors[commit.AuthorEmail] = struct{}{}
		day := commit.When.Format("2006-01-02")
		days[day]++
		additions += commit.Additions
		deletions += commit.Deletions
		netLines += commit.Additions - commit.Deletions
		if commit.ConventionalType != "" {
			conventional++
			breakdown[commit.ConventionalType]++
		}
	}

	var breakdownList []NamedValue
	for name, value := range breakdown {
		breakdownList = append(breakdownList, NamedValue{Name: name, Value: value})
	}
	sort.Slice(breakdownList, func(i, j int) bool {
		if breakdownList[i].Value == breakdownList[j].Value {
			return breakdownList[i].Name < breakdownList[j].Name
		}
		return breakdownList[i].Value > breakdownList[j].Value
	})

	return Overview{
		CommitCount:             len(commits),
		AuthorCount:             len(authors),
		Additions:               additions,
		Deletions:               deletions,
		NetLines:                netLines,
		CurrentStreak:           currentStreak(days),
		LongestStreak:           longestStreak(days),
		ConventionalCommitShare: percentage(conventional, len(commits)),
		ConventionalBreakdown:   breakdownList,
	}
}

func filterCommitsRange(commits []git.CommitRecord, startInclusive, endExclusive time.Time) []git.CommitRecord {
	filtered := make([]git.CommitRecord, 0, len(commits))
	for _, commit := range commits {
		if commit.When.Before(startInclusive) {
			continue
		}
		if !endExclusive.IsZero() && !commit.When.Before(endExclusive) {
			continue
		}
		filtered = append(filtered, commit)
	}
	return filtered
}

func currentStreak(days map[string]int) int {
	if len(days) == 0 {
		return 0
	}

	parsed := make(map[string]time.Time, len(days))
	var latest time.Time
	for key := range days {
		day, err := time.Parse("2006-01-02", key)
		if err != nil {
			continue
		}
		parsed[key] = day
		if day.After(latest) {
			latest = day
		}
	}

	streak := 0
	for day := latest; ; day = day.AddDate(0, 0, -1) {
		if _, ok := days[day.Format("2006-01-02")]; !ok {
			return streak
		}
		streak++
	}
}

func longestStreak(days map[string]int) int {
	if len(days) == 0 {
		return 0
	}

	var ordered []time.Time
	for key := range days {
		day, err := time.Parse("2006-01-02", key)
		if err == nil {
			ordered = append(ordered, day)
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Before(ordered[j]) })

	longest := 1
	current := 1
	for idx := 1; idx < len(ordered); idx++ {
		if ordered[idx-1].AddDate(0, 0, 1).Equal(ordered[idx]) {
			current++
		} else {
			current = 1
		}
		if current > longest {
			longest = current
		}
	}
	return longest
}

func buildCommitActivity(commits []git.CommitRecord) CommitActivity {
	dailyMap := map[string]int{}
	weeklyMap := map[string]int{}
	weekday := []NamedValue{
		{Name: "Mon"}, {Name: "Tue"}, {Name: "Wed"}, {Name: "Thu"}, {Name: "Fri"}, {Name: "Sat"}, {Name: "Sun"},
	}
	hourly := make([]NamedValue, 24)
	for idx := range hourly {
		hourly[idx] = NamedValue{Name: time.Date(0, 1, 1, idx, 0, 0, 0, time.UTC).Format("15")}
	}

	for _, commit := range commits {
		dayKey := commit.When.Format("2006-01-02")
		weekStart := startOfWeek(commit.When).Format("2006-01-02")
		dailyMap[dayKey]++
		weeklyMap[weekStart]++
		weekday[weekdayIndex(commit.When.Weekday())].Value++
		hourly[commit.When.Hour()].Value++
	}

	return CommitActivity{
		Daily:   sortedDateValues(dailyMap),
		Weekly:  sortedDateValues(weeklyMap),
		Weekday: weekday,
		Hourly:  hourly,
	}
}

func buildAuthorActivity(allCommits, commits, weekCommits, lastWeekCommits, monthCommits []git.CommitRecord, now time.Time) AuthorActivity {
	leaderboardMap := map[string]*AuthorSummary{}
	firstSeen := map[string]time.Time{}
	seenBeforeWindow := map[string]bool{}
	weekAuthors := map[string]struct{}{}
	lastWeekAuthors := map[string]struct{}{}
	monthAuthors := map[string]struct{}{}

	windowStart := time.Time{}
	if len(commits) > 0 {
		windowStart = commits[0].When
		for _, commit := range commits[1:] {
			if commit.When.Before(windowStart) {
				windowStart = commit.When
			}
		}
	}

	for _, commit := range allCommits {
		if first, ok := firstSeen[commit.AuthorEmail]; !ok || commit.When.Before(first) {
			firstSeen[commit.AuthorEmail] = commit.When
		}
		if !windowStart.IsZero() && commit.When.Before(windowStart) {
			seenBeforeWindow[commit.AuthorEmail] = true
		}
	}

	for _, commit := range commits {
		entry, ok := leaderboardMap[commit.AuthorEmail]
		if !ok {
			entry = &AuthorSummary{
				Name:      commit.AuthorName,
				Email:     commit.AuthorEmail,
				FirstSeen: firstSeen[commit.AuthorEmail],
			}
			leaderboardMap[commit.AuthorEmail] = entry
		}
		entry.Commits++
		entry.Additions += commit.Additions
		entry.Deletions += commit.Deletions
		if commit.When.After(entry.LastSeen) {
			entry.LastSeen = commit.When
		}
	}

	for _, commit := range weekCommits {
		weekAuthors[commit.AuthorEmail] = struct{}{}
	}
	for _, commit := range monthCommits {
		monthAuthors[commit.AuthorEmail] = struct{}{}
	}
	for _, commit := range lastWeekCommits {
		lastWeekAuthors[commit.AuthorEmail] = struct{}{}
	}

	var leaderboard []AuthorSummary
	var newContributors []AuthorSummary
	for email, entry := range leaderboardMap {
		leaderboard = append(leaderboard, *entry)
		if !seenBeforeWindow[email] && !entry.FirstSeen.IsZero() && !entry.FirstSeen.After(now) {
			newContributors = append(newContributors, *entry)
		}
	}

	sort.Slice(leaderboard, func(i, j int) bool {
		if leaderboard[i].Commits == leaderboard[j].Commits {
			return leaderboard[i].Name < leaderboard[j].Name
		}
		return leaderboard[i].Commits > leaderboard[j].Commits
	})
	sort.Slice(newContributors, func(i, j int) bool {
		return newContributors[i].FirstSeen.After(newContributors[j].FirstSeen)
	})
	if len(leaderboard) > 8 {
		leaderboard = leaderboard[:8]
	}
	if len(newContributors) > 5 {
		newContributors = newContributors[:5]
	}

	return AuthorActivity{
		ActiveThisWeek:  len(weekAuthors),
		ActiveLastWeek:  len(lastWeekAuthors),
		ActiveThisMonth: len(monthAuthors),
		NewContributors: newContributors,
		Leaderboard:     leaderboard,
		BusFactor:       busFactor(leaderboardMap, len(commits)),
	}
}

func busFactor(authors map[string]*AuthorSummary, totalCommits int) int {
	if totalCommits == 0 {
		return 0
	}

	var contributions []int
	for _, author := range authors {
		contributions = append(contributions, author.Commits)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(contributions)))

	threshold := totalCommits / 2
	if totalCommits%2 != 0 {
		threshold++
	}

	sum := 0
	for idx, commits := range contributions {
		sum += commits
		if sum >= threshold {
			return idx + 1
		}
	}
	return len(contributions)
}

func buildFileActivity(commits []git.CommitRecord) FileActivity {
	fileMap := map[string]*FileSummary{}
	dirMap := map[string]*DirectorySummary{}

	for _, commit := range commits {
		for _, file := range commit.Files {
			entry, ok := fileMap[file.Path]
			if !ok {
				entry = &FileSummary{Path: file.Path}
				fileMap[file.Path] = entry
			}
			entry.Touches++
			entry.Additions += file.Additions
			entry.Deletions += file.Deletions
			if commit.When.After(entry.LastChange) {
				entry.LastChange = commit.When
			}

			dir := topLevelDir(file.Path)
			dirEntry, ok := dirMap[dir]
			if !ok {
				dirEntry = &DirectorySummary{Path: dir}
				dirMap[dir] = dirEntry
			}
			dirEntry.Touches++
			dirEntry.Churn += file.Additions + file.Deletions
		}
	}

	var hotspots []FileSummary
	for _, entry := range fileMap {
		hotspots = append(hotspots, *entry)
	}
	sort.Slice(hotspots, func(i, j int) bool {
		if hotspots[i].Touches == hotspots[j].Touches {
			return hotspots[i].Path < hotspots[j].Path
		}
		return hotspots[i].Touches > hotspots[j].Touches
	})
	if len(hotspots) > 10 {
		hotspots = hotspots[:10]
	}

	var directories []DirectorySummary
	for _, entry := range dirMap {
		directories = append(directories, *entry)
	}
	sort.Slice(directories, func(i, j int) bool {
		if directories[i].Churn == directories[j].Churn {
			return directories[i].Path < directories[j].Path
		}
		return directories[i].Churn > directories[j].Churn
	})
	if len(directories) > 8 {
		directories = directories[:8]
	}

	return FileActivity{
		Hotspots:    hotspots,
		Directories: directories,
	}
}

func buildBranchActivity(data git.RepositoryData, now time.Time) BranchActivity {
	var active []BranchSummary
	var stale []BranchSummary
	for _, branch := range data.Branches {
		ageDays := int(now.Sub(branch.LastCommitAt).Hours() / 24)
		summary := BranchSummary{
			Name:         branch.Name,
			LastCommitAt: branch.LastCommitAt,
			AgeDays:      ageDays,
		}
		if ageDays <= 14 {
			active = append(active, summary)
		} else {
			stale = append(stale, summary)
		}
	}

	sort.Slice(active, func(i, j int) bool { return active[i].LastCommitAt.After(active[j].LastCommitAt) })
	sort.Slice(stale, func(i, j int) bool { return stale[i].LastCommitAt.Before(stale[j].LastCommitAt) })

	lastTag := ""
	var cadence float64
	if len(data.Tags) > 0 {
		sort.Slice(data.Tags, func(i, j int) bool { return data.Tags[i].When.Before(data.Tags[j].When) })
		lastTag = data.Tags[len(data.Tags)-1].Name
		if len(data.Tags) > 1 {
			totalGap := 0.0
			for idx := 1; idx < len(data.Tags); idx++ {
				totalGap += data.Tags[idx].When.Sub(data.Tags[idx-1].When).Hours() / 24
			}
			cadence = totalGap / float64(len(data.Tags)-1)
		}
	}

	return BranchActivity{
		ActiveBranches:     active,
		StaleBranches:      stale,
		ReleaseCadenceDays: cadence,
		LastTag:            lastTag,
	}
}

func sortedDateValues(values map[string]int) []DateValue {
	out := make([]DateValue, 0, len(values))
	for key, value := range values {
		date, err := time.Parse("2006-01-02", key)
		if err != nil {
			continue
		}
		out = append(out, DateValue{Date: date, Value: value})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

func weekdayIndex(day time.Weekday) int {
	if day == time.Sunday {
		return 6
	}
	return int(day) - 1
}

func startOfWeek(ts time.Time) time.Time {
	offset := weekdayIndex(ts.Weekday())
	return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, ts.Location()).AddDate(0, 0, -offset)
}

func topLevelDir(path string) string {
	if path == "" {
		return "."
	}
	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		return "."
	}
	return parts[0]
}

func percentage(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
