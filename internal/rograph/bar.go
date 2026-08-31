// Package rograph ("read-only graph") holds small, dependency-free
// renderers for single-row, fixed-max-width terminal graphs: a filled/empty
// usage bar and an absolute-scale sparkline. Both share one shrink-to-fit
// contract: render at min(maxWidth, availableWidth), shrinking down to a
// minimum of 1 character when space is tight, never panicking or going
// negative.
package rograph

// MaxWidth is the shared maximum render width, in characters/glyphs, for
// every single-row rograph graph (bars and sparklines alike). Call sites
// should request min(MaxWidth, availableWidth) rather than inventing their
// own clamp.
const MaxWidth = 10

// RenderProgressBar generates an ANSI/Unicode progress bar of the given
// character width: a "[filled empty]" bar using █ for the filled portion
// and ░ for the empty portion, scaled to usedPercent (clamped to [0, 100]).
// The single boundary character where fill transitions from filled to
// empty renders at eighth-block precision (▏▎▍▌▋▊▉█) rather than snapping
// to fully filled or fully empty; characters fully before or after the
// boundary are unaffected. width is the max-width for this bar; width < 1
// clamps up to 1 so the function never panics or renders a negative-length
// bar. This is a thin convenience wrapper around RenderBar's options-based
// path; callers that want the ANSI-background option should call RenderBar
// directly with BarOptions{Width: width, SubChar: true, ANSI: true}.
func RenderProgressBar(usedPercent float64, width int) string {
	if width < 1 {
		width = 1
	}
	return RenderBar(usedPercent, BarOptions{Width: width, SubChar: true})
}
