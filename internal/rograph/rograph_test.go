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
		{"width 0 clamps to 1", 50, 0, "[░]"},
		{"negative width clamps to 1", 50, -5, "[░]"},
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
