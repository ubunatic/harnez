package rograph

// percentSparkChars are the 8 sparkline glyphs used for a 0-100% value's
// height, low to high (▁ = idle/0%, █ = 100%). Unlike a relative (per-series
// min/max) sparkline, this is an absolute scale: a flat history at 5% should
// look idle, not maxed out.
var percentSparkChars = []rune("▁▂▃▄▅▆▇█")

// PercentSparkline renders one glyph per value in pcts (each 0-100), one
// glyph per recent sample in a rolling timeline. A single-element slice
// degenerates to one glyph (the current value, no history yet). The glyphs
// keep normal foreground color but sit on a muted grey background (ANSI
// 100, "bright black") so the graph reads as its own panel instead of
// full-brightness "█" blocks fighting the terminal's own background.
//
// maxWidth is the max-width for this sparkline: if len(pcts) > maxWidth,
// only the most recent maxWidth values are rendered. maxWidth < 1 clamps up
// to 1 so the function never panics or renders a negative-length sparkline.
func PercentSparkline(pcts []float64, maxWidth int) string {
	if maxWidth < 1 {
		maxWidth = 1
	}
	if len(pcts) > maxWidth {
		pcts = pcts[len(pcts)-maxWidth:]
	}
	spark := make([]rune, len(pcts))
	for i, p := range pcts {
		idx := int(p / 100 * float64(len(percentSparkChars)))
		if idx >= len(percentSparkChars) {
			idx = len(percentSparkChars) - 1
		}
		if idx < 0 {
			idx = 0
		}
		spark[i] = percentSparkChars[idx]
	}
	return "\x1b[" + DefaultBackgroundANSI + "m" + string(spark) + "\x1b[0m"
}
