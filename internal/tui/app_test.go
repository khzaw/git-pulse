package tui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git-pulse/internal/config"
)

func TestResolveTheme(t *testing.T) {
	t.Parallel()

	theme, err := ResolveTheme("tokyo-night")
	require.NoError(t, err)
	require.Equal(t, "tokyo-night", theme.Name)
}

func TestResolveThemeRejectsUnknownTheme(t *testing.T) {
	t.Parallel()

	_, err := ResolveTheme("missing")
	require.Error(t, err)
}

func TestViewIncludesKeyPanels(t *testing.T) {
	t.Parallel()

	model, err := NewModel(config.Default())
	require.NoError(t, err)

	view := model.View()
	require.Contains(t, view, "git-pulse")
	require.Contains(t, view, "Commit Velocity")
	require.Contains(t, view, "Author Activity")
	require.Contains(t, view, "File Hotspots")
	require.Contains(t, view, "Branch & PR Health")
}
