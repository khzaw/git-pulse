package dashboard

import (
	"context"
	"fmt"
	"time"

	"git-pulse/internal/aggregator"
	gitrepo "git-pulse/internal/git"
	"git-pulse/internal/remote"
)

type PRFetcher interface {
	FetchSnapshot(ctx context.Context, ref remote.RepositoryRef) (remote.PRSnapshot, error)
}

type Loader struct {
	FetchPRs PRFetcher
	Now      func() time.Time
}

type LoadOptions struct {
	IncludeRemote bool
}

type Result struct {
	Snapshot aggregator.Snapshot  `json:"snapshot"`
	PRs      remote.PRSnapshot    `json:"prs"`
	Remote   remote.RepositoryRef `json:"remote"`
	Warning  string               `json:"warning,omitempty"`
}

func NewLoader() Loader {
	return Loader{
		FetchPRs: remote.NewGitHubClient(nil),
		Now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (l Loader) Load(ctx context.Context, repoPath string, window aggregator.TimeWindow) (Result, error) {
	return l.LoadWithOptions(ctx, repoPath, window, LoadOptions{IncludeRemote: true})
}

func (l Loader) LoadLocal(ctx context.Context, repoPath string, window aggregator.TimeWindow) (Result, error) {
	return l.LoadWithOptions(ctx, repoPath, window, LoadOptions{})
}

func (l Loader) LoadRemote(ctx context.Context, repoPath string, window aggregator.TimeWindow) (Result, error) {
	return l.LoadWithOptions(ctx, repoPath, window, LoadOptions{IncludeRemote: true})
}

func (l Loader) LoadWithOptions(ctx context.Context, repoPath string, window aggregator.TimeWindow, opts LoadOptions) (Result, error) {
	now := time.Now().UTC()
	if l.Now != nil {
		now = l.Now().UTC()
	}

	data, err := gitrepo.Scan(ctx, repoPath)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Snapshot: aggregator.Aggregate(data, aggregator.Options{
			Now:    now,
			Window: window,
		}),
	}

	ref, err := remote.Detect(repoPath)
	if err != nil {
		return Result{}, fmt.Errorf("detect remote: %w", err)
	}
	result.Remote = ref

	if opts.IncludeRemote && ref.Provider == remote.ProviderGitHub && l.FetchPRs != nil {
		fetchCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()

		prs, err := l.FetchPRs.FetchSnapshot(fetchCtx, ref)
		if err != nil {
			result.Warning = fmt.Sprintf("remote metrics unavailable: %v", err)
			return result, nil
		}
		result.PRs = prs
	}

	return result, nil
}
