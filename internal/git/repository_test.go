package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
)

func TestScanCollectsRepositoryMetrics(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo := initTestRepo(t, dir)

	initial := commitFile(t, repo, dir, "README.md", "# git-pulse\n", object.Signature{
		Name:  "Ada",
		Email: "ada@example.com",
		When:  time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC),
	}, "docs: initial readme")

	_, err := repo.CreateTag("v0.1.0", initial, nil)
	require.NoError(t, err)

	second := commitFile(t, repo, dir, "internal/app.go", "package main\n\nfunc main() {}\n", object.Signature{
		Name:  "Bob",
		Email: "bob@example.com",
		When:  time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC),
	}, "feat: add entrypoint")

	worktree, err := repo.Worktree()
	require.NoError(t, err)
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("feature/ui"),
		Create: true,
	})
	require.NoError(t, err)

	commitFile(t, repo, dir, "internal/tui.go", "package main\n", object.Signature{
		Name:  "Cara",
		Email: "cara@example.com",
		When:  time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC),
	}, "feat: add branch-only view")

	err = worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("master")})
	require.NoError(t, err)
	_, err = repo.CreateTag("v0.2.0", second, nil)
	require.NoError(t, err)

	data, err := Scan(context.Background(), dir)
	require.NoError(t, err)

	require.Equal(t, dir, data.Path)
	require.Equal(t, "master", data.DefaultBranch)
	require.NotEmpty(t, data.Head)
	require.Len(t, data.Commits, 2)
	require.Equal(t, "docs", data.Commits[0].ConventionalType)
	require.Equal(t, "feat", data.Commits[1].ConventionalType)
	require.Len(t, data.Branches, 2)
	require.Len(t, data.Tags, 2)
	require.Equal(t, "v0.2.0", data.Tags[1].Name)
}

func TestConventionalType(t *testing.T) {
	t.Parallel()

	require.Equal(t, "feat", conventionalType("feat(ui): render dashboard"))
	require.Equal(t, "fix", conventionalType("fix!: change refresh logic"))
	require.Empty(t, conventionalType("Ship it"))
}

func TestParseNumStat(t *testing.T) {
	t.Parallel()

	stats := parseNumStat([]byte("12\t3\tinternal/tui/app.go\n-\t-\tassets/logo.png\n1\t0\tREADME.md\n"))
	require.Len(t, stats, 3)
	require.Equal(t, "internal/tui/app.go", stats[0].Name)
	require.Equal(t, 12, stats[0].Addition)
	require.Equal(t, 3, stats[0].Deletion)
	require.Equal(t, "assets/logo.png", stats[1].Name)
	require.Equal(t, 0, stats[1].Addition)
	require.Equal(t, 0, stats[1].Deletion)
}

func TestParseGitLogOutput(t *testing.T) {
	t.Parallel()

	output := []byte("\x1eabc123\x1fAda\x1fada@example.com\x1f2026-03-08T12:00:00Z\x1ffeat: add dashboard\n12\t3\tinternal/tui/app.go\n1\t0\tREADME.md\n\x1edef456\x1fBob\x1fbob@example.com\x1f2026-03-09T01:00:00Z\x1ffix: adjust layout\n4\t2\tinternal/tui/widgets.go\n")
	commits, err := parseGitLogOutput(output)
	require.NoError(t, err)
	require.Len(t, commits, 2)
	require.Equal(t, "abc123", commits[0].Hash)
	require.Equal(t, "feat", commits[0].ConventionalType)
	require.Equal(t, 13, commits[0].Additions)
	require.Equal(t, 3, commits[0].Deletions)
	require.Len(t, commits[0].Files, 2)
	require.Equal(t, "fix", commits[1].ConventionalType)
}

func initTestRepo(t *testing.T, dir string) *git.Repository {
	t.Helper()

	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	return repo
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
