package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

func columnChart(values []int, width, height int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	sampled := sampleInts(values, width)
	if len(sampled) == 0 {
		sampled = make([]int, width)
	}

	maxValue := 0
	for _, value := range sampled {
		if value > maxValue {
			maxValue = value
		}
	}

	rows := make([]strings.Builder, height)
	for idx := 0; idx < height; idx++ {
		rows[idx].Grow(len(sampled))
	}

	for _, value := range sampled {
		level := 0
		if maxValue > 0 {
			level = int(math.Round(float64(value) / float64(maxValue) * float64(height)))
		}
		if level > height {
			level = height
		}
		for row := 0; row < height; row++ {
			threshold := height - row
			switch {
			case level >= threshold:
				rows[row].WriteRune('█')
			case level == threshold-1:
				rows[row].WriteRune('▄')
			default:
				rows[row].WriteRune(' ')
			}
		}
	}

	out := make([]string, 0, height)
	for _, row := range rows {
		out = append(out, strings.TrimRight(row.String(), " "))
	}
	return out
}

func renderAxisColumnChart(values []int, width, height int) []string {
	if width <= 6 || height <= 1 {
		return []string{}
	}
	chart := columnChart(values, width-5, height)
	maxValue := 0
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}

	// Only show labels on ~5-8 rows to avoid a wall of numbers
	labelInterval := 1
	if height > 10 {
		labelInterval = height / 6
		if labelInterval < 2 {
			labelInterval = 2
		}
	}

	lines := make([]string, 0, height+1)
	for row := 0; row < height; row++ {
		level := 0
		if height > 1 {
			level = int(math.Round(float64(maxValue) * float64(height-row-1) / float64(height-1)))
		}
		if row%labelInterval == 0 {
			lines = append(lines, fmt.Sprintf("%3d│%s", level, padRight(chart[row], width-4)))
		} else {
			lines = append(lines, fmt.Sprintf("   │%s", padRight(chart[row], width-4)))
		}
	}
	lines = append(lines, "   0└"+strings.Repeat("─", width-5))
	return lines
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

func meterBar(value, total, width int, fill, empty lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	if total <= 0 {
		return empty.Render(strings.Repeat("░", width))
	}

	filled := int(math.Round(float64(value) / float64(total) * float64(width)))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	return fill.Render(strings.Repeat("█", filled)) + empty.Render(strings.Repeat("░", width-filled))
}

func sampleInts(values []int, width int) []int {
	if len(values) == 0 || width <= 0 {
		return nil
	}
	if len(values) == width {
		return append([]int(nil), values...)
	}
	if len(values) < width {
		// Upsample: stretch values to fill available width
		out := make([]int, width)
		for idx := range out {
			src := idx * len(values) / width
			if src >= len(values) {
				src = len(values) - 1
			}
			out[idx] = values[src]
		}
		return out
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

// brailleColumnChart renders a column chart using braille Unicode characters (U+2800 block).
// Each character cell encodes a 2-wide × 4-tall dot grid, giving 2× horizontal and 4× vertical
// resolution compared to block characters.
//
// Braille dot positions and their bit values:
//
//	Left  Right
//	(1) 0x01  (4) 0x08
//	(2) 0x02  (5) 0x10
//	(3) 0x04  (6) 0x20
//	(7) 0x40  (8) 0x80
func brailleColumnChart(values []int, width, height int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}

	// Each char cell is 2 data columns wide and 4 dot rows tall
	dataCols := width * 2
	dotRows := height * 4

	sampled := sampleInts(values, dataCols)
	if len(sampled) == 0 {
		sampled = make([]int, dataCols)
	}

	maxValue := 0
	for _, v := range sampled {
		if v > maxValue {
			maxValue = v
		}
	}

	// Scale each value to dot-row height
	scaled := make([]int, len(sampled))
	if maxValue > 0 {
		for i, v := range sampled {
			scaled[i] = int(math.Round(float64(v) / float64(maxValue) * float64(dotRows)))
			if scaled[i] > dotRows {
				scaled[i] = dotRows
			}
		}
	}

	// Left-column dot bits (rows 0-3 from top within a cell)
	leftBits := [4]rune{0x40, 0x04, 0x02, 0x01}
	// Right-column dot bits
	rightBits := [4]rune{0x80, 0x20, 0x10, 0x08}

	rows := make([]string, height)
	for cellRow := 0; cellRow < height; cellRow++ {
		var out strings.Builder
		out.Grow(width * 3) // braille chars are 3 bytes in UTF-8
		for cellCol := 0; cellCol < width; cellCol++ {
			var ch rune = 0x2800 // empty braille
			leftIdx := cellCol * 2
			rightIdx := leftIdx + 1

			for dotRow := 0; dotRow < 4; dotRow++ {
				// Global dot row from top: cellRow*4 + dotRow
				// A dot is filled if the column's scaled height reaches this row from the bottom
				globalDotRow := cellRow*4 + dotRow
				threshold := dotRows - globalDotRow // dots needed to reach this row

				if leftIdx < len(scaled) && scaled[leftIdx] >= threshold {
					ch |= leftBits[dotRow]
				}
				if rightIdx < len(scaled) && scaled[rightIdx] >= threshold {
					ch |= rightBits[dotRow]
				}
			}
			out.WriteRune(ch)
		}
		rows[cellRow] = out.String()
	}
	return rows
}

func renderBrailleAxisChart(values []int, width, height int) []string {
	if width <= 6 || height <= 1 {
		return []string{}
	}
	chartWidth := width - 5 // 4 chars for label + 1 for │
	chart := brailleColumnChart(values, chartWidth, height)

	maxValue := 0
	for _, v := range values {
		if v > maxValue {
			maxValue = v
		}
	}

	labelInterval := 1
	if height > 10 {
		labelInterval = height / 6
		if labelInterval < 2 {
			labelInterval = 2
		}
	}

	lines := make([]string, 0, height+1)
	for row := 0; row < height; row++ {
		level := 0
		if height > 1 {
			level = int(math.Round(float64(maxValue) * float64(height-row-1) / float64(height-1)))
		}
		chartLine := ""
		if row < len(chart) {
			chartLine = chart[row]
		}
		if row%labelInterval == 0 {
			lines = append(lines, fmt.Sprintf("%3d│%s", level, padRight(chartLine, chartWidth)))
		} else {
			lines = append(lines, fmt.Sprintf("   │%s", padRight(chartLine, chartWidth)))
		}
	}
	lines = append(lines, "   0└"+strings.Repeat("─", chartWidth))
	return lines
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
	if width == 1 {
		return "…"
	}
	return ansi.Truncate(value, width, "…")
}
