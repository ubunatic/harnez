package usage

import (
	"time"

	"ubunatic.com/harnez/internal/rograph"
)

// freshnessFraction returns how much of DefaultCollectorInterval remains
// before the next background-collector tick (issue 082) for an agent whose
// data was last refreshed at lastRefreshed, as a 0.0-1.0 fraction of the
// interval: 1.0 right when lastRefreshed updates, draining toward 0.0 as
// the next tick approaches.
//
// A zero lastRefreshed (never refreshed) and an overdue/negative remaining
// duration (elapsed has already exceeded the interval — a missed tick, or a
// slow collector) both clamp to 0.0 rather than going negative or wrapping.
// This is issue 131's pure elapsed/remaining -> fraction mapping, kept
// separate from freshnessGauge so it's testable without any rendering.
func freshnessFraction(lastRefreshed, now time.Time) float64 {
	return freshnessFractionForInterval(lastRefreshed, now, DefaultCollectorInterval)
}

// freshnessFractionForInterval is freshnessFraction for a particular refresh
// schedule. The watch view fetches on its configured interval, which is
// normally much shorter than the collector's cadence.
func freshnessFractionForInterval(lastRefreshed, now time.Time, interval time.Duration) float64 {
	if lastRefreshed.IsZero() {
		return 0
	}
	if interval <= 0 {
		return 0
	}
	elapsed := now.Sub(lastRefreshed)
	remaining := interval - elapsed
	if remaining <= 0 {
		return 0
	}
	frac := float64(remaining) / float64(interval)
	if frac > 1 {
		frac = 1
	}
	return frac
}

// freshnessGauge renders fraction (0.0-1.0) as a 3-character countdown
// gauge, e.g. "[█]" (full/just refreshed) draining to "[▁]" (overdue/empty).
// It reuses rograph's shared percent-sparkline glyph set (issue 078) rather
// than a one-off bar implementation. It remains plain so callers outside the
// debug overlay retain its exactly 3-rune output.
func freshnessGauge(fraction float64) string {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	glyph := rograph.RenderPercentSparkline([]float64{fraction * 100}, rograph.SparklineOptions{Width: 1})
	return "[" + glyph + "]"
}

// freshnessOverlayLabel returns the countdown glyph before a shortened label
// (issue 131's `!` debug overlay). It replaces — never appends or pads — so
// the label's total visible width is unchanged from non-overlay rendering.
// Labels with 3 or fewer runes are replaced in full, since there is nothing
// left to keep.
func freshnessOverlayLabel(label string, lastRefreshed, now time.Time) string {
	return freshnessOverlayLabelForInterval(label, lastRefreshed, now, DefaultCollectorInterval)
}

// freshnessOverlayLabelForInterval renders the watch debug gauge against the
// watch's next scheduled fetch rather than the unrelated collector schedule.
func freshnessOverlayLabelForInterval(label string, lastRefreshed, now time.Time, interval time.Duration) string {
	fraction := freshnessFractionForInterval(lastRefreshed, now, interval)
	glyph := timeoutSnakeGlyph(fraction)
	r := []rune(label)
	if len(r) <= 3 {
		return glyph
	}
	return glyph + " " + string(r[:len(r)-3]) + "…"
}

// styleTimeGaugeGlyph gives the compact debug overlay's first (gauge) glyph
// its independently specified graph colors after the label has been padded.
// Styling earlier would make rograph.PadLabel count invisible ANSI runes.
func styleTimeGaugeGlyph(line string) string {
	r := []rune(line)
	if len(r) == 0 {
		return line
	}
	return ansiOpen("time-gauge-fg") + ansiOpen("time-gauge-bg") + string(r[0]) + "\x1b[0m" + string(r[1:])
}
