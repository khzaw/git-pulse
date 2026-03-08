package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHelpHidesRepoFlag(t *testing.T) {
	t.Parallel()

	help, err := HelpText()
	require.NoError(t, err)
	require.Contains(t, help, "Run git-pulse from inside a git repository")
	require.NotContains(t, help, "--repo")
	require.Contains(t, help, "--json")
	require.Contains(t, help, "--markdown")
	require.Contains(t, help, "--remote")
}

func TestRepoFlagRemainsAvailableButHidden(t *testing.T) {
	t.Parallel()

	opts := options{}
	cmd := newRootCmd(&opts)

	flag := cmd.Flags().Lookup("repo")
	require.NotNil(t, flag)
	require.True(t, flag.Hidden)
}
