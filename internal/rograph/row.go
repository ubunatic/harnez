package rograph

import (
	"fmt"
	"unicode/utf8"
)

// PadLabel truncates label to width runes (replacing the last rune with an
// ellipsis "…" when it's too long), then left-pads the result to exactly
// width columns with spaces. Used for the fixed-width leading label in a
// "label [graph] trailing" row, so every row's graph starts at the same
// column regardless of the label's natural length.
func PadLabel(label string, width int) string {
	if utf8.RuneCountInString(label) > width {
		label = string([]rune(label)[:width-1]) + "…"
	}
	return fmt.Sprintf("%-*s", width, label)
}

// RowLayout computes how many characters a shrinkable graph (a bar or
// sparkline) gets inside a fixed-width row shaped
// "<label> [<graph>] <percent><trailing>", and whether the trailing text
// still fits.
//
// The row reserves labelWidth for the label, one space, two characters for
// the graph's own brackets, one space, percentWidth for the percent field,
// and trailingWidth for optional trailing text (e.g. " · 3h2m"). The graph
// gets up to graphMaxWidth characters; if there isn't room for the graph at
// its target size *and* the trailing text, the trailing text is dropped
// first (keepTrailing = false) and the graph is recomputed against the
// freed space. Only once the trailing text is already dropped does the
// graph itself shrink, down to a floor of 1 character — it never goes
// below that or above graphMaxWidth.
func RowLayout(totalWidth, labelWidth, percentWidth, graphMaxWidth, trailingWidth int) (barWidth int, keepTrailing bool) {
	fixed := labelWidth + 1 + 2 + 1 + percentWidth
	barWidth = totalWidth - fixed - trailingWidth
	keepTrailing = true
	if barWidth < 1 {
		keepTrailing = false
		barWidth = totalWidth - fixed
	}
	if barWidth < 1 {
		barWidth = 1
	}
	if barWidth > graphMaxWidth {
		barWidth = graphMaxWidth
	}
	return barWidth, keepTrailing
}
