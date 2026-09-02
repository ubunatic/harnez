package rograph

import (
	"fmt"
	"math"
	"testing"
)

func TestRenderBarOptions(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		opts  BarOptions
		want  string
	}{
		{"zero options use default bar", 50, BarOptions{}, "[█████░░░░░]"},
		{"width clamps up from negative", 100, BarOptions{Width: -4}, "[█]"},
		{"custom range supports negatives", 0, BarOptions{Width: 4, Min: -10, Max: 10}, "[██░░]"},
		{"value clamps above range", 20, BarOptions{Width: 4, Min: -10, Max: 10}, "[████]"},
		{"value clamps below range", -20, BarOptions{Width: 4, Min: -10, Max: 10}, "[░░░░]"},
		{"nan renders as minimum", math.NaN(), BarOptions{Width: 4}, "[░░░░]"},
		{"infinity clamps above range", math.Inf(1), BarOptions{Width: 4}, "[████]"},
		{"invalid range falls back to percent scale", 50, BarOptions{Width: 4, Min: 10, Max: 10}, "[██░░]"},
		{"stripe omits wrappers", 50, BarOptions{Width: 4, NoWrapper: true}, "██░░"},
		{"custom glyphs and wrappers", 50, BarOptions{Width: 4, Fill: '#', Empty: '-', Left: "{", Right: "}"}, "{##--}"},
		{"percent label defaults to whole percent", 12.5, BarOptions{Width: 4, IncludePercent: true}, "[░░░░] 12%"},
		{"percent label honors precision", 12.5, BarOptions{Width: 4, IncludePercent: true, PercentPrecision: 1}, "[░░░░] 12.5%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderBar(tt.value, tt.opts)
			if got != tt.want {
				t.Errorf("RenderBar(%v, %+v) = %q, want %q", tt.value, tt.opts, got, tt.want)
			}
		})
	}
}

func TestRenderSparklineOptions(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		opts   SparklineOptions
		want   string
	}{
		{"nil renders empty", nil, SparklineOptions{}, ""},
		{"empty renders empty", []float64{}, SparklineOptions{}, ""},
		{"zero options pair into cells", []float64{0, 25, 50, 75, 100}, SparklineOptions{}, "▁▄█"},
		{"width keeps most recent doubled resolution", []float64{0, 25, 50, 75, 100}, SparklineOptions{Width: 2}, "▃█"},
		{"negative width clamps to one recent cell", []float64{0, 100}, SparklineOptions{Width: -1}, "█"},
		{"flat relative range uses middle glyph", []float64{7, 7, 7}, SparklineOptions{}, "▄▄"},
		{"relative range supports negative values", []float64{-10, 0, 10}, SparklineOptions{}, "▁█"},
		{"non-finite relative values clamp to visible range", []float64{0, math.NaN(), math.Inf(1), 100}, SparklineOptions{}, "▁█"},
		{"fixed range clamps below and above", []float64{-10, 0, 50, 100, 110}, SparklineOptions{FixedRange: true, Min: 0, Max: 100}, "▁▄█"},
		{"all non-finite values use middle glyph", []float64{math.NaN(), math.Inf(-1)}, SparklineOptions{}, "▄"},
		{"invalid fixed range uses middle glyph", []float64{1, 2}, SparklineOptions{FixedRange: true, Min: 5, Max: 5}, "▄"},
		{"percent convenience uses absolute scale", []float64{5, 5, 5}, SparklineOptions{}, "▁▁"},
		{"ansi wraps output", []float64{0, 100}, SparklineOptions{ANSI: true, BackgroundANSI: "44"}, "\x1b[44m█\x1b[0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			if tt.name == "percent convenience uses absolute scale" {
				got = RenderPercentSparkline(tt.values, tt.opts)
			} else {
				got = RenderSparkline(tt.values, tt.opts)
			}
			if got != tt.want {
				t.Errorf("sparkline render = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestANSIChartsOmitWrapperWhenResolvedBackgroundIsAbsent(t *testing.T) {
	old := DefaultBackgroundANSI
	DefaultBackgroundANSI = ""
	t.Cleanup(func() { DefaultBackgroundANSI = old })
	if got, want := RenderBar(50, BarOptions{Width: 2, ANSI: true}), "[█░]"; got != want {
		t.Fatalf("bar without background = %q, want %q", got, want)
	}
	if got, want := RenderPercentSparkline([]float64{0, 100}, SparklineOptions{ANSI: true}), "█"; got != want {
		t.Fatalf("sparkline without background = %q, want %q", got, want)
	}
}

func TestRenderBrailleSparkline(t *testing.T) {
	fixed := SparklineOptions{Presentation: SparklineBraille, FixedRange: true, Min: 0, Max: 100}
	tests := []struct {
		name string
		in   []float64
		opts SparklineOptions
		want string
	}{
		{"pair orientation", []float64{0, 100, 100, 0}, fixed, "⢸⡇"},
		{"odd leading singleton is duplicated", []float64{10, 20, 30}, fixed, "⠀⢀"},
		{"latest doubled window", []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100}, SparklineOptions{Presentation: SparklineBraille, FixedRange: true, Min: 0, Max: 100, Width: 10}, "⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RenderSparkline(tt.in, tt.opts); got != tt.want {
				t.Fatalf("RenderSparkline() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderBrailleSparklineIsSafeForAllInputs(t *testing.T) {
	values := []float64{math.NaN(), math.Inf(-1), -10, 0, 50, 100, 110, math.Inf(1)}
	got := []rune(RenderPercentSparkline(values, SparklineOptions{Presentation: SparklineBraille, Width: 4}))
	if len(got) != 4 {
		t.Fatalf("Braille width = %d, want 4", len(got))
	}
	for _, glyph := range got {
		if glyph < 0x2800 || glyph > 0x28ff {
			t.Errorf("glyph %U is not a Braille pattern", glyph)
		}
	}
}

func TestFormatPercent(t *testing.T) {
	tests := []struct {
		name      string
		percent   float64
		precision int
		want      string
	}{
		{"whole percent", 12.5, 0, "12%"},
		{"one decimal", 12.5, 1, "12.5%"},
		{"negative clamps to zero", -1, 1, "0.0%"},
		{"over one hundred clamps", 101, 0, "100%"},
		{"negative precision clamps", 12.5, -1, "12%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPercent(tt.percent, tt.precision)
			if got != tt.want {
				t.Errorf("FormatPercent(%v, %d) = %q, want %q", tt.percent, tt.precision, got, tt.want)
			}
		})
	}
}

func ExampleRenderBar() {
	fmt.Println(RenderBar(75, BarOptions{Width: 4, IncludePercent: true}))
	// Output:
	// [███░] 75%
}

func ExampleRenderSparkline() {
	fmt.Println(RenderSparkline([]float64{-2, -1, 0, 1, 2}, SparklineOptions{}))
	// Output:
	// ▁▄█
}
