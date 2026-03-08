package remote

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/google/go-github/v68/github"
	"golang.org/x/sync/errgroup"
)

type PullRequest struct {
	Number          int
	Title           string
	State           string
	URL             string
	CreatedAt       time.Time
	MergedAt        time.Time
	ClosedAt        time.Time
	FirstReviewedAt time.Time
	Additions       int
	Deletions       int
}

type PRSnapshot struct {
	Repository         string
	Windows            []WindowMetric
	WeeklyCycle        []WeeklyCycle
	WeeklyThroughput   []WeeklyCount
	MedianReviewTime   time.Duration
	OpenPullRequests   []OpenPullRequest
	SizeDistribution   []Bucket
	HasReviewData      bool
	CollectedAt        time.Time
	ReviewCoverage     int
	MergedPullRequests int
}

type WindowMetric struct {
	Label            string
	MergedCount      int
	MedianCycleTime  time.Duration
	MedianReviewTime time.Duration
}

type WeeklyCycle struct {
	WeekStart time.Time
	Median    time.Duration
}

type WeeklyCount struct {
	WeekStart time.Time
	Count     int
}

type OpenPullRequest struct {
	Number  int
	Title   string
	URL     string
	AgeDays int
}

type Bucket struct {
	Label string
	Count int
}

type githubPullRequestService interface {
	List(ctx context.Context, owner string, repo string, opts *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error)
	ListReviews(ctx context.Context, owner string, repo string, number int, opts *github.ListOptions) ([]*github.PullRequestReview, *github.Response, error)
}

type GitHubClient struct {
	service githubPullRequestService
	now     func() time.Time
}

func NewGitHubClient(httpClient *http.Client) *GitHubClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	client := github.NewClient(httpClient)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		client = client.WithAuthToken(token)
	}

	return &GitHubClient{
		service: client.PullRequests,
		now:     time.Now,
	}
}

func (c *GitHubClient) FetchSnapshot(ctx context.Context, ref RepositoryRef) (PRSnapshot, error) {
	if ref.Provider != ProviderGitHub {
		return PRSnapshot{}, nil
	}

	prs, err := c.fetchPullRequests(ctx, ref.Owner, ref.Name)
	if err != nil {
		return PRSnapshot{}, err
	}

	return SummarizePullRequests(ref.FullName(), prs, c.now()), nil
}

func (c *GitHubClient) fetchPullRequests(ctx context.Context, owner, repo string) ([]PullRequest, error) {
	openPRs, err := c.listPullRequests(ctx, owner, repo, "open", 1, 20)
	if err != nil {
		return nil, err
	}

	closedPRs, err := c.listPullRequests(ctx, owner, repo, "closed", 1, 50)
	if err != nil {
		return nil, err
	}

	pullRequests := append(openPRs, filterRecentClosed(closedPRs, c.now().AddDate(0, 0, -120))...)
	if err := c.populateReviewTimes(ctx, owner, repo, pullRequests); err != nil {
		return nil, err
	}

	return pullRequests, nil
}

func (c *GitHubClient) listPullRequests(ctx context.Context, owner, repo, state string, pages int, perPage int) ([]PullRequest, error) {
	opts := &github.PullRequestListOptions{
		State:     state,
		Sort:      "updated",
		Direction: "desc",
		ListOptions: github.ListOptions{
			PerPage: perPage,
		},
	}

	var pullRequests []PullRequest
	for page := 0; page < pages; page++ {
		ghPRs, response, err := c.service.List(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("list %s pull requests: %w", state, err)
		}

		for _, pr := range ghPRs {
			pullRequests = append(pullRequests, mapPullRequest(pr))
		}

		if response.NextPage == 0 {
			break
		}
		opts.Page = response.NextPage
	}
	return pullRequests, nil
}

func (c *GitHubClient) populateReviewTimes(ctx context.Context, owner, repo string, prs []PullRequest) error {
	sem := make(chan struct{}, 6)
	group, groupCtx := errgroup.WithContext(ctx)

	for idx := range prs {
		if prs[idx].Number == 0 || prs[idx].MergedAt.IsZero() {
			continue
		}
		idx := idx
		group.Go(func() error {
			select {
			case <-groupCtx.Done():
				return groupCtx.Err()
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			reviews, _, err := c.service.ListReviews(groupCtx, owner, repo, prs[idx].Number, &github.ListOptions{PerPage: 100})
			if err != nil {
				return fmt.Errorf("list reviews for #%d: %w", prs[idx].Number, err)
			}

			var first time.Time
			for _, review := range reviews {
				if review.SubmittedAt == nil {
					continue
				}
				submittedAt := review.SubmittedAt.UTC()
				if first.IsZero() || submittedAt.Before(first) {
					first = submittedAt
				}
			}
			prs[idx].FirstReviewedAt = first
			return nil
		})
	}

	if err := group.Wait(); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

func mapPullRequest(pr *github.PullRequest) PullRequest {
	mapped := PullRequest{}
	if pr == nil {
		return mapped
	}

	mapped.Number = pr.GetNumber()
	mapped.Title = pr.GetTitle()
	mapped.State = pr.GetState()
	mapped.URL = pr.GetHTMLURL()
	if pr.CreatedAt != nil {
		mapped.CreatedAt = pr.CreatedAt.UTC()
	}
	if pr.MergedAt != nil {
		mapped.MergedAt = pr.MergedAt.UTC()
	}
	if pr.ClosedAt != nil {
		mapped.ClosedAt = pr.ClosedAt.UTC()
	}
	mapped.Additions = pr.GetAdditions()
	mapped.Deletions = pr.GetDeletions()
	return mapped
}

func SummarizePullRequests(repository string, prs []PullRequest, now time.Time) PRSnapshot {
	snapshot := PRSnapshot{
		Repository:  repository,
		CollectedAt: now,
		Windows: []WindowMetric{
			summarizeWindow("7d", prs, now.AddDate(0, 0, -7)),
			summarizeWindow("30d", prs, now.AddDate(0, 0, -30)),
			summarizeWindow("90d", prs, now.AddDate(0, 0, -90)),
		},
		WeeklyCycle:      weeklyCycle(prs),
		WeeklyThroughput: weeklyThroughput(prs),
		OpenPullRequests: summarizeOpen(prs, now),
		SizeDistribution: sizeDistribution(prs),
	}

	var reviewDurations []time.Duration
	for _, pr := range prs {
		if pr.MergedAt.IsZero() {
			continue
		}
		snapshot.MergedPullRequests++
		if !pr.FirstReviewedAt.IsZero() && !pr.FirstReviewedAt.Before(pr.CreatedAt) {
			reviewDurations = append(reviewDurations, pr.FirstReviewedAt.Sub(pr.CreatedAt))
			snapshot.ReviewCoverage++
		}
	}
	snapshot.MedianReviewTime = medianDuration(reviewDurations)
	snapshot.HasReviewData = len(reviewDurations) > 0

	return snapshot
}

func summarizeWindow(label string, prs []PullRequest, cutoff time.Time) WindowMetric {
	var cycleDurations []time.Duration
	var reviewDurations []time.Duration

	for _, pr := range prs {
		if pr.MergedAt.IsZero() || pr.MergedAt.Before(cutoff) {
			continue
		}
		cycleDurations = append(cycleDurations, pr.MergedAt.Sub(pr.CreatedAt))
		if !pr.FirstReviewedAt.IsZero() && !pr.FirstReviewedAt.Before(pr.CreatedAt) {
			reviewDurations = append(reviewDurations, pr.FirstReviewedAt.Sub(pr.CreatedAt))
		}
	}

	return WindowMetric{
		Label:            label,
		MergedCount:      len(cycleDurations),
		MedianCycleTime:  medianDuration(cycleDurations),
		MedianReviewTime: medianDuration(reviewDurations),
	}
}

func filterRecentClosed(prs []PullRequest, cutoff time.Time) []PullRequest {
	filtered := make([]PullRequest, 0, len(prs))
	for _, pr := range prs {
		if !pr.MergedAt.IsZero() {
			if pr.MergedAt.Before(cutoff) {
				continue
			}
			filtered = append(filtered, pr)
			continue
		}
		if !pr.ClosedAt.IsZero() && pr.ClosedAt.Before(cutoff) {
			continue
		}
		filtered = append(filtered, pr)
	}
	return filtered
}

func weeklyCycle(prs []PullRequest) []WeeklyCycle {
	series := map[time.Time][]time.Duration{}
	for _, pr := range prs {
		if pr.MergedAt.IsZero() {
			continue
		}
		week := weekStart(pr.MergedAt)
		series[week] = append(series[week], pr.MergedAt.Sub(pr.CreatedAt))
	}

	var weeks []time.Time
	for week := range series {
		weeks = append(weeks, week)
	}
	sort.Slice(weeks, func(i, j int) bool { return weeks[i].Before(weeks[j]) })
	if len(weeks) > 12 {
		weeks = weeks[len(weeks)-12:]
	}

	out := make([]WeeklyCycle, 0, len(weeks))
	for _, week := range weeks {
		out = append(out, WeeklyCycle{
			WeekStart: week,
			Median:    medianDuration(series[week]),
		})
	}
	return out
}

func weeklyThroughput(prs []PullRequest) []WeeklyCount {
	series := map[time.Time]int{}
	for _, pr := range prs {
		if pr.MergedAt.IsZero() {
			continue
		}
		series[weekStart(pr.MergedAt)]++
	}

	var weeks []time.Time
	for week := range series {
		weeks = append(weeks, week)
	}
	sort.Slice(weeks, func(i, j int) bool { return weeks[i].Before(weeks[j]) })
	if len(weeks) > 12 {
		weeks = weeks[len(weeks)-12:]
	}

	out := make([]WeeklyCount, 0, len(weeks))
	for _, week := range weeks {
		out = append(out, WeeklyCount{
			WeekStart: week,
			Count:     series[week],
		})
	}
	return out
}

func summarizeOpen(prs []PullRequest, now time.Time) []OpenPullRequest {
	var open []OpenPullRequest
	for _, pr := range prs {
		if pr.State != "open" {
			continue
		}
		open = append(open, OpenPullRequest{
			Number:  pr.Number,
			Title:   pr.Title,
			URL:     pr.URL,
			AgeDays: int(now.Sub(pr.CreatedAt).Hours() / 24),
		})
	}
	sort.Slice(open, func(i, j int) bool { return open[i].AgeDays > open[j].AgeDays })
	if len(open) > 8 {
		open = open[:8]
	}
	return open
}

func sizeDistribution(prs []PullRequest) []Bucket {
	buckets := []Bucket{
		{Label: "XS"},
		{Label: "S"},
		{Label: "M"},
		{Label: "L"},
		{Label: "XL"},
	}

	for _, pr := range prs {
		size := pr.Additions + pr.Deletions
		switch {
		case size < 50:
			buckets[0].Count++
		case size < 150:
			buckets[1].Count++
		case size < 400:
			buckets[2].Count++
		case size < 800:
			buckets[3].Count++
		default:
			buckets[4].Count++
		}
	}
	return buckets
}

func medianDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}

	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return values[mid]
	}
	return (values[mid-1] + values[mid]) / 2
}

func weekStart(ts time.Time) time.Time {
	weekday := int(ts.Weekday())
	if ts.Weekday() == time.Sunday {
		weekday = 7
	}
	offset := weekday - 1
	return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, ts.Location()).AddDate(0, 0, -offset)
}
