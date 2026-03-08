package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSparklineRespectsWidth(t *testing.T) {
	t.Parallel()

	got := sparkline([]int{1, 2, 3, 4, 5, 6}, 4)
	require.Len(t, []rune(got), 4)
}

func TestProgressBarUsesExpectedWidth(t *testing.T) {
	t.Parallel()

	got := progressBar(3, 6, 10)
	require.Len(t, got, 30)
	require.Contains(t, got, "█")
}

func TestCompactDuration(t *testing.T) {
	t.Parallel()

	require.Equal(t, "4h", compactDuration(4*time.Hour))
	require.Equal(t, "2.0d", compactDuration(48*time.Hour))
	require.Equal(t, "n/a", compactDuration(0))
}
