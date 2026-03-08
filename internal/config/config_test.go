package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	t.Parallel()

	cfg := Default()
	require.Equal(t, ".", cfg.RepoPath)
	require.Equal(t, "tokyo-night", cfg.Theme)
	require.Equal(t, 60, cfg.RefreshSeconds)
	require.Equal(t, "30d", cfg.DefaultWindow)
}

func TestLoadOverridesValues(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "git-pulse.yml")
	err := os.WriteFile(path, []byte("repo_path: /tmp/repo\ntheme: gruvbox\nrefresh_seconds: 15\ndefault_window: 90d\n"), 0o600)
	require.NoError(t, err)

	cfg, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, "/tmp/repo", cfg.RepoPath)
	require.Equal(t, "gruvbox", cfg.Theme)
	require.Equal(t, 15, cfg.RefreshSeconds)
	require.Equal(t, "90d", cfg.DefaultWindow)
}
