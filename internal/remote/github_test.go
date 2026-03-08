package remote

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-github/v68/github"
	"github.com/stretchr/testify/require"
)

func TestSummarizePullRequests(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	prs := []PullRequest{
		{
			Number:          1,
			Title:           "Ship dashboard",
			State:           "closed",
			CreatedAt:       now.AddDate(0, 0, -10),
			MergedAt:        now.AddDate(0, 0, -6),
			FirstReviewedAt: now.AddDate(0, 0, -9),
			Additions:       20,
			Deletions:       10,
		},
		{
			Number:          2,
			Title:           "Fix branch view",
			State:           "closed",
			CreatedAt:       now.AddDate(0, 0, -20),
			MergedAt:        now.AddDate(0, 0, -15),
			FirstReviewedAt: now.AddDate(0, 0, -18),
			Additions:       120,
			Deletions:       30,
		},
		{
			Number:    3,
			Title:     "Open cleanup",
			State:     "open",
			URL:       "https://github.com/acme/git-pulse/pull/3",
			CreatedAt: now.AddDate(0, 0, -16),
			Additions: 700,
			Deletions: 80,
		},
	}

	snapshot := SummarizePullRequests("acme/git-pulse", prs, now)

	require.Equal(t, "acme/git-pulse", snapshot.Repository)
	require.Len(t, snapshot.Windows, 3)
	require.Equal(t, 1, snapshot.Windows[0].MergedCount)
	require.Equal(t, 2, snapshot.Windows[1].MergedCount)
	require.Equal(t, 2, snapshot.MergedPullRequests)
	require.Equal(t, 2, snapshot.ReviewCoverage)
	require.True(t, snapshot.HasReviewData)
	require.Len(t, snapshot.OpenPullRequests, 1)
	require.Equal(t, 16, snapshot.OpenPullRequests[0].AgeDays)
	require.Equal(t, 1, snapshot.SizeDistribution[0].Count)
	require.Equal(t, 0, snapshot.SizeDistribution[1].Count)
	require.Equal(t, 1, snapshot.SizeDistribution[2].Count)
	require.Equal(t, 1, snapshot.SizeDistribution[3].Count)
}

func TestMapPullRequest(t *testing.T) {
	t.Parallel()

	created := github.Timestamp{Time: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)}
	merged := github.Timestamp{Time: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)}
	pr := &github.PullRequest{
		Number:    github.Ptr(7),
		Title:     github.Ptr("feat: add widget"),
		State:     github.Ptr("closed"),
		HTMLURL:   github.Ptr("https://github.com/acme/git-pulse/pull/7"),
		CreatedAt: &created,
		MergedAt:  &merged,
		Additions: github.Ptr(42),
		Deletions: github.Ptr(5),
	}

	mapped := mapPullRequest(pr)
	require.Equal(t, 7, mapped.Number)
	require.Equal(t, "feat: add widget", mapped.Title)
	require.Equal(t, 42, mapped.Additions)
	require.Equal(t, 5, mapped.Deletions)
	require.Equal(t, created.Time, mapped.CreatedAt)
	require.Equal(t, merged.Time, mapped.MergedAt)
}

func TestPopulateReviewTimes(t *testing.T) {
	t.Parallel()

	reviewedAt := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	client := &GitHubClient{
		service: fakePullRequestService{
			reviews: map[int][]*github.PullRequestReview{
				9: {
					{SubmittedAt: &github.Timestamp{Time: reviewedAt.Add(2 * time.Hour)}},
					{SubmittedAt: &github.Timestamp{Time: reviewedAt}},
				},
			},
		},
		now: func() time.Time { return reviewedAt },
	}

	prs := []PullRequest{{Number: 9}}
	err := client.populateReviewTimes(context.Background(), "acme", "git-pulse", prs)
	require.NoError(t, err)
	require.Equal(t, reviewedAt, prs[0].FirstReviewedAt)
}

type fakePullRequestService struct {
	pullRequests []*github.PullRequest
	reviews      map[int][]*github.PullRequestReview
}

func (f fakePullRequestService) List(_ context.Context, _, _ string, _ *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error) {
	return f.pullRequests, &github.Response{}, nil
}

func (f fakePullRequestService) ListReviews(_ context.Context, _, _ string, number int, _ *github.ListOptions) ([]*github.PullRequestReview, *github.Response, error) {
	return f.reviews[number], &github.Response{}, nil
}
