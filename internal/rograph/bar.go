// Package rograph ("read-only graph") holds small, dependency-free
// renderers for single-row, fixed-max-width terminal graphs: a filled/empty
// usage bar and an absolute-scale sparkline. Both share one shrink-to-fit
// contract: render at min(maxWidth, availableWidth), shrinking down to a
// minimum of 1 character when space is tight, never panicking or going
// negative.
package rograph

import "strings"

// RenderProgressBar generates an ANSI/Unicode progress bar of the given
// character width: a "[filled empty]" bar using █ for the filled portion
// and ░ for the empty portion, scaled to usedPercent (clamped to [0, 100]).
// width is the max-width for this bar; width < 1 clamps up to 1 so the
// function never panics or renders a negative-length bar.
func RenderProgressBar(usedPercent float64, width int) string {
	if width < 1 {
		width = 1
	}
	if usedPercent < 0 {
		usedPercent = 0
	}
	if usedPercent > 100 {
		usedPercent = 100
	}
	filledCount := int(float64(width) * (usedPercent / 100.0))
	if filledCount > width {
		filledCount = width
	}
	emptyCount := width - filledCount

	filled := strings.Repeat("█", filledCount)
	empty := strings.Repeat("░", emptyCount)
	return "[" + filled + empty + "]"
}
