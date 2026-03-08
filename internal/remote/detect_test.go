package remote

import (
	"testing"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/stretchr/testify/require"
)

func TestDetectFromURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		urls []string
		want RepositoryRef
		ok   bool
	}{
		{
			name: "github ssh",
			urls: []string{"git@github.com:acme/git-pulse.git"},
			want: RepositoryRef{Provider: ProviderGitHub, Owner: "acme", Name: "git-pulse"},
			ok:   true,
		},
		{
			name: "github https",
			urls: []string{"https://github.com/acme/git-pulse.git"},
			want: RepositoryRef{Provider: ProviderGitHub, Owner: "acme", Name: "git-pulse"},
			ok:   true,
		},
		{
			name: "gitlab https",
			urls: []string{"https://gitlab.com/acme/git-pulse.git"},
			want: RepositoryRef{Provider: ProviderGitLab, Owner: "acme", Name: "git-pulse"},
			ok:   true,
		},
		{
			name: "unsupported host",
			urls: []string{"https://example.com/acme/git-pulse.git"},
			ok:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := DetectFromURLs(tt.urls)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestDetectPrefersOrigin(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	_, err = repo.CreateRemote(&gitconfig.RemoteConfig{Name: "upstream", URLs: []string{"https://gitlab.com/acme/other.git"}})
	require.NoError(t, err)
	_, err = repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{"git@github.com:acme/git-pulse.git"}})
	require.NoError(t, err)

	ref, err := Detect(dir)
	require.NoError(t, err)
	require.Equal(t, RepositoryRef{Provider: ProviderGitHub, Owner: "acme", Name: "git-pulse"}, ref)
}

func TestRepositoryRefFullName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "acme/git-pulse", RepositoryRef{Owner: "acme", Name: "git-pulse"}.FullName())
	require.Equal(t, "", RepositoryRef{}.FullName())
}

func TestDetectEmptyRepoWithoutRemote(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	ref, err := Detect(dir)
	require.NoError(t, err)
	require.Equal(t, RepositoryRef{}, ref)
}
