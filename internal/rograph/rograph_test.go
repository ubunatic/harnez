package rograph

import (
	"strings"
	"testing"
)

func TestRenderProgressBar(t *testing.T) {
	tests := []struct {
		name string
		pct  float64
		w    int
		want string
	}{
		{"normal width 0%", 0, 10, "[░░░░░░░░░░]"},
		{"normal width 50%", 50, 10, "[█████░░░░░]"},
		{"normal width 100%", 100, 10, "[██████████]"},
		{"clamp over 100%", 120, 10, "[██████████]"},
		{"clamp under 0%", -10, 10, "[░░░░░░░░░░]"},
		{"shrink to 1 char, filled", 100, 1, "[█]"},
		{"shrink to 1 char, empty", 0, 1, "[░]"},
		{"width 0 clamps to 1, sub-char boundary", 50, 0, "[▌]"},
		{"negative width clamps to 1, sub-char boundary", 50, -5, "[▌]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderProgressBar(tt.pct, tt.w)
			if got != tt.want {
				t.Errorf("RenderProgressBar(%v, %d) = %q, want %q", tt.pct, tt.w, got, tt.want)
			}
		})
	}
}

func TestMaxWidthCapsOutput(t *testing.T) {
	if MaxWidth != 10 {
		t.Fatalf("MaxWidth = %d, want 10", MaxWidth)
	}

	// The primitives themselves render exactly the width they're asked
	// for (that's the shrink-to-fit contract); callers are responsible
	// for requesting min(MaxWidth, availableWidth). Assert that
	// requesting MaxWidth produces output within the documented bracket
	// budget, i.e. that MaxWidth is a sane, honored request size.
	bar := RenderProgressBar(50, MaxWidth)
	if got := len([]rune(bar)); got != MaxWidth+2 {
		t.Errorf("RenderProgressBar(50, MaxWidth) produced %d runes, want %d", got, MaxWidth+2)
	}

	// Sparkline output is at most MaxWidth glyphs (no brackets) when a
	// longer history is capped at MaxWidth by the caller, as every call
	// site in internal/usage now does via min(rograph.MaxWidth, ...).
	series := make([]float64, 50)
	for i := range series {
		series[i] = float64(i)
	}
	spark := stripAnsi(PercentSparkline(series, MaxWidth))
	if got := len([]rune(spark)); got != MaxWidth {
		t.Errorf("PercentSparkline(series, MaxWidth) produced %d glyphs, want %d", got, MaxWidth)
	}
}

func TestRenderProgressBarEighthBlockBoundary(t *testing.T) {
	// width=4 turns the bar's effective resolution from 5 discrete states
	// (0/4..4/4) to 32 (4 characters x 8 eighths) via the single boundary
	// character. Only the boundary character (where fill transitions from
	// filled to empty) should render at sub-character precision; fully
	// filled/empty characters elsewhere stay snapped.
	tests := []struct {
		name string
		pct  float64
		want string
	}{
		{"0%", 0, "[░░░░]"},
		{"12.5% boundary at half of char 0", 12.5, "[▌░░░]"},
		{"24% boundary at seven-eighths of char 0", 24, "[▉░░░]"},
		{"26% char 0 full, boundary empty at char 1", 26, "[█░░░]"},
		{"49% char 0 full, boundary at seven-eighths of char 1", 49, "[█▉░░]"},
		{"51% chars 0-1 full, boundary empty at char 2", 51, "[██░░]"},
		{"100% fully filled, no boundary character", 100, "[████]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderProgressBar(tt.pct, 4)
			if got != tt.want {
				t.Errorf("RenderProgressBar(%v, 4) = %q, want %q", tt.pct, got, tt.want)
			}
		})
	}
}

func TestRenderBarSubCharOptOut(t *testing.T) {
	// Without SubChar, RenderBar keeps the legacy whole-character snapping
	// behavior even at a percentage that would otherwise land mid-eighth.
	got := RenderBar(12.5, BarOptions{Width: 4})
	want := "[░░░░]"
	if got != want {
		t.Errorf("RenderBar(12.5, BarOptions{Width: 4}) = %q, want %q", got, want)
	}
}

func TestRenderBarSubCharIgnoresCustomGlyphs(t *testing.T) {
	// Eighth-block glyphs only exist for the default '█'/'░' pair, so a
	// custom Fill/Empty falls back to whole-character snapping even with
	// SubChar set.
	got := RenderBar(12.5, BarOptions{Width: 4, SubChar: true, Fill: '#', Empty: '-'})
	want := "[----]"
	if got != want {
		t.Errorf("RenderBar with custom glyphs = %q, want %q", got, want)
	}
}

// TestRenderBarANSIBackground guards issue 136's bracket-leak fix: the ANSI
// background wraps only the glyph portion, so "[" and "]" (and, in a
// separate test below, any IncludePercent label) stay outside the escape
// sequence -- matching RenderSparkline/PercentSparkline's existing
// glyph-only wrap convention rather than coloring the whole "[glyphs]"
// string.
func TestRenderBarANSIBackground(t *testing.T) {
	got := RenderBar(50, BarOptions{Width: 4, ANSI: true})
	want := "[" + "\x1b[100m" + "██░░" + "\x1b[0m" + "]"
	if got != want {
		t.Errorf("RenderBar ANSI default = %q, want %q", got, want)
	}

	got = RenderBar(50, BarOptions{Width: 4, ANSI: true, BackgroundANSI: "44"})
	want = "[" + "\x1b[44m" + "██░░" + "\x1b[0m" + "]"
	if got != want {
		t.Errorf("RenderBar ANSI custom code = %q, want %q", got, want)
	}
}

// TestRenderBarANSIKeepsPercentLabelOutsideWrap covers the IncludePercent
// case explicitly called out in issue 136's acceptance criteria: the label
// renders after the closing bracket, outside the ANSI escape.
func TestRenderBarANSIKeepsPercentLabelOutsideWrap(t *testing.T) {
	got := RenderBar(50, BarOptions{Width: 4, ANSI: true, IncludePercent: true})
	want := "[" + "\x1b[100m" + "██░░" + "\x1b[0m" + "]" + " 50%"
	if got != want {
		t.Errorf("RenderBar ANSI with percent label = %q, want %q", got, want)
	}
}

func TestRenderProgressBarNeverPanics(t *testing.T) {
	for _, w := range []int{-100, -1, 0, 1, 2, 100} {
		for _, pct := range []float64{-1000, -1, 0, 50, 100, 1000} {
			_ = RenderProgressBar(pct, w)
		}
	}
}

// stripAnsi removes the sparkline's fixed grey-background wrapper so tests
// can assert on the glyphs alone.
func stripAnsi(s string) string {
	s = strings.TrimPrefix(s, "\x1b[100m")
	s = strings.TrimSuffix(s, "\x1b[0m")
	return s
}

func TestPercentSparkline(t *testing.T) {
	tests := []struct {
		name     string
		pcts     []float64
		maxWidth int
		want     string
	}{
		{"single value", []float64{0}, 10, "▁"},
		{"full value", []float64{100}, 10, "█"},
		{"fewer values than maxWidth", []float64{0, 50, 100}, 10, "▁▅█"},
		{"more values than maxWidth uses most recent", []float64{0, 25, 50, 75, 100}, 2, "▇█"},
		{"maxWidth 0 clamps to 1, keeps most recent", []float64{0, 100}, 0, "█"},
		{"negative maxWidth clamps to 1", []float64{0, 100}, -3, "█"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripAnsi(PercentSparkline(tt.pcts, tt.maxWidth))
			if got != tt.want {
				t.Errorf("PercentSparkline(%v, %d) = %q, want %q", tt.pcts, tt.maxWidth, got, tt.want)
			}
		})
	}
}

func TestPercentSparklineNeverPanics(t *testing.T) {
	series := []float64{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	for _, w := range []int{-100, -1, 0, 1, 2, 5, 100} {
		_ = PercentSparkline(series, w)
	}
	_ = PercentSparkline(nil, 5)
	_ = PercentSparkline([]float64{}, -1)
}
