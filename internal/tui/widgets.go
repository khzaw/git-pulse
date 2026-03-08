package tui

import (
	"fmt"
	"math"
	"strings"
	"time"
)

var sparklineBlocks = []rune("▁▂▃▄▅▆▇█")

func sparkline(values []int, width int) string {
	if len(values) == 0 || width <= 0 {
		return ""
	}

	sampled := sampleInts(values, width)
	maxValue := 0
	for _, value := range sampled {
		if value > maxValue {
			maxValue = value
		}
	}
	if maxValue == 0 {
		return strings.Repeat(string(sparklineBlocks[0]), len(sampled))
	}

	var out strings.Builder
	for _, value := range sampled {
		idx := int(math.Round(float64(value) / float64(maxValue) * float64(len(sparklineBlocks)-1)))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparklineBlocks) {
			idx = len(sparklineBlocks) - 1
		}
		out.WriteRune(sparklineBlocks[idx])
	}
	return out.String()
}

func progressBar(value, total, width int) string {
	if width <= 0 {
		return ""
	}
	if total <= 0 {
		return strings.Repeat("░", width)
	}

	filled := int(math.Round(float64(value) / float64(total) * float64(width)))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func sampleInts(values []int, width int) []int {
	if len(values) <= width {
		return append([]int(nil), values...)
	}

	out := make([]int, width)
	for idx := range out {
		start := idx * len(values) / width
		end := (idx + 1) * len(values) / width
		if end <= start {
			end = start + 1
		}

		sum := 0
		for _, value := range values[start:end] {
			sum += value
		}
		out[idx] = int(math.Round(float64(sum) / float64(end-start)))
	}
	return out
}

func compactDuration(duration time.Duration) string {
	if duration <= 0 {
		return "n/a"
	}

	hours := duration.Hours()
	switch {
	case hours < 24:
		return fmt.Sprintf("%.0fh", hours)
	case hours < 24*14:
		return fmt.Sprintf("%.1fd", hours/24)
	default:
		return fmt.Sprintf("%.1fw", hours/24/7)
	}
}

func compactPercent(value float64) string {
	return fmt.Sprintf("%.0f%%", value*100)
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}
