package dashboard

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"

	"git-pulse/internal/aggregator"
	"git-pulse/internal/remote"
)

func TestLoaderBuildsDashboardResult(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	_, err = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{"git@github.com:acme/git-pulse.git"}})
	require.NoError(t, err)

	commitFile(t, repo, dir, "README.md", "hello\n", object.Signature{
		Name:  "Ada",
		Email: "ada@example.com",
		When:  time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC),
	}, "feat: bootstrap")

	loader := Loader{
		FetchPRs: fakeFetcher{snapshot: remote.PRSnapshot{Repository: "acme/git-pulse"}},
		Now: func() time.Time {
			return time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
		},
	}

	result, err := loader.Load(context.Background(), dir, aggregator.Window30Days)
	require.NoError(t, err)
	require.Equal(t, "acme/git-pulse", result.PRs.Repository)
	require.Equal(t, remote.ProviderGitHub, result.Remote.Provider)
	require.Equal(t, 1, result.Snapshot.Overview.CommitCount)
}

func TestLoaderKeepsLocalSnapshotWhenPRFetchFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	_, err = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{"git@github.com:acme/git-pulse.git"}})
	require.NoError(t, err)

	commitFile(t, repo, dir, "README.md", "hello\n", object.Signature{
		Name:  "Ada",
		Email: "ada@example.com",
		When:  time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC),
	}, "feat: bootstrap")

	loader := Loader{
		FetchPRs: fakeFetcher{err: context.DeadlineExceeded},
		Now: func() time.Time {
			return time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
		},
	}

	result, err := loader.Load(context.Background(), dir, aggregator.Window30Days)
	require.NoError(t, err)
	require.Equal(t, 1, result.Snapshot.Overview.CommitCount)
	require.Contains(t, result.Warning, "remote metrics unavailable")
}

type fakeFetcher struct {
	snapshot remote.PRSnapshot
	err      error
}

func (f fakeFetcher) FetchSnapshot(_ context.Context, _ remote.RepositoryRef) (remote.PRSnapshot, error) {
	return f.snapshot, f.err
}

func commitFile(t *testing.T, repo *git.Repository, dir, name, contents string, signature object.Signature, message string) plumbing.Hash {
	t.Helper()

	fullPath := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
	require.NoError(t, os.WriteFile(fullPath, []byte(contents), 0o600))

	worktree, err := repo.Worktree()
	require.NoError(t, err)
	_, err = worktree.Add(name)
	require.NoError(t, err)

	hash, err := worktree.Commit(message, &git.CommitOptions{
		Author:    &signature,
		Committer: &signature,
	})
	require.NoError(t, err)
	return hash
}
