package rograph

import (
	"fmt"
	"math"
	"strings"
)

// BarOptions configures RenderBar. The zero value renders a 10-character
// 0-100 bar with the package's standard filled and empty glyphs.
type BarOptions struct {
	// Width is the number of graph glyphs inside the optional wrappers.
	// Zero uses MaxWidth; negative values clamp to 1.
	Width int
	// Min and Max define the numeric range. The zero value means 0-100.
	Min float64
	Max float64
	// Fill and Empty override the default terminal bar glyphs.
	Fill  rune
	Empty rune
	// Left and Right wrap the graph. Empty uses the default "[" and "]"
	// unless NoWrapper is true.
	Left  string
	Right string
	// NoWrapper renders only the graph glyphs, useful for stripe-style bars.
	NoWrapper bool
	// IncludePercent appends a formatted percent label after the bar.
	IncludePercent bool
	// PercentPrecision controls the optional percent label. Negative values
	// clamp to 0.
	PercentPrecision int
	// SubChar renders the single boundary character (where fill transitions
	// from filled to empty) using a horizontal eighth-block glyph
	// (▏▎▍▌▋▊▉█) instead of snapping it to fully filled or fully empty.
	// Only applies when Fill and Empty are left at their defaults ('█' and
	// '░'); custom glyphs fall back to whole-character snapping since
	// eighth-block glyphs only exist for the default block characters.
	SubChar bool
	// ANSI wraps the rendered bar (including any percent label) in a
	// background ANSI sequence, matching RenderSparkline's convention.
	// Plain output is the default so callers can opt in only for terminal
	// contexts.
	ANSI bool
	// BackgroundANSI is the SGR code used when ANSI is true. Empty uses
	// "100" (bright-black), the same default as RenderSparkline.
	BackgroundANSI string
}

// eighthBlockGlyphs are the horizontal eighth-block glyphs used for a
// sub-character fill boundary, 1/8 through 8/8 width.
var eighthBlockGlyphs = []rune("▏▎▍▌▋▊▉█")

// eighthBlockFill renders width default-glyph characters ('█'/'░') with the
// single boundary character rendered at eighth-block precision rather than
// snapped to fully filled or fully empty. pct must already be clamped to
// [0, 100].
func eighthBlockFill(pct float64, width int) string {
	totalEighths := int(float64(width*8) * (pct / 100))
	if totalEighths < 0 {
		totalEighths = 0
	}
	maxEighths := width * 8
	if totalEighths > maxEighths {
		totalEighths = maxEighths
	}

	fullChars := totalEighths / 8
	remainder := totalEighths % 8
	if fullChars >= width {
		fullChars = width
		remainder = 0
	}

	var b strings.Builder
	b.WriteString(strings.Repeat("█", fullChars))
	if fullChars < width {
		if remainder == 0 {
			b.WriteRune('░')
		} else {
			b.WriteRune(eighthBlockGlyphs[remainder-1])
		}
		b.WriteString(strings.Repeat("░", width-fullChars-1))
	}
	return b.String()
}

// SparklineOptions configures RenderSparkline. The zero value renders a
// no-ANSI, relative-scale sparkline capped at MaxWidth glyphs.
type SparklineOptions struct {
	// Width is the maximum number of recent values to render. Zero uses
	// MaxWidth; negative values clamp to 1.
	Width int
	// Min and Max define the scale when FixedRange is true.
	Min float64
	Max float64
	// FixedRange uses Min and Max instead of deriving a range from the
	// rendered values. Use this for percent or other absolute-scale charts.
	FixedRange bool
	// ANSI wraps the sparkline in a background ANSI sequence. Plain output is
	// the default so callers can opt in only for terminal contexts.
	ANSI bool
	// BackgroundANSI is the SGR code used when ANSI is true. Empty uses "100".
	BackgroundANSI string
}

// RenderBar renders a single-value terminal bar or stripe. Values outside the
// configured range are clamped, NaN values render as the minimum, and inverted
// or empty ranges fall back to the zero-value 0-100 range.
func RenderBar(value float64, opts BarOptions) string {
	width := optionWidth(opts.Width)
	minimum, maximum := barRange(opts.Min, opts.Max)
	pct := normalizedPercent(value, minimum, maximum)

	fill := opts.Fill
	if fill == 0 {
		fill = '█'
	}
	empty := opts.Empty
	if empty == 0 {
		empty = '░'
	}
	left, right := opts.Left, opts.Right
	if !opts.NoWrapper && left == "" && right == "" {
		left = "["
		right = "]"
	}

	var glyphs string
	if opts.SubChar && fill == '█' && empty == '░' {
		glyphs = eighthBlockFill(pct, width)
	} else {
		filledCount := int(float64(width) * (pct / 100))
		if filledCount < 0 {
			filledCount = 0
		}
		if filledCount > width {
			filledCount = width
		}
		glyphs = strings.Repeat(string(fill), filledCount) + strings.Repeat(string(empty), width-filledCount)
	}

	var b strings.Builder
	b.WriteString(left)
	b.WriteString(glyphs)
	b.WriteString(right)
	if opts.IncludePercent {
		b.WriteByte(' ')
		b.WriteString(FormatPercent(pct, opts.PercentPrecision))
	}

	out := b.String()
	if opts.ANSI {
		code := opts.BackgroundANSI
		if code == "" {
			code = "100"
		}
		out = "\x1b[" + code + "m" + out + "\x1b[0m"
	}
	return out
}

// RenderSparkline renders the most recent values as one terminal glyph per
// value. Relative mode derives the range from the rendered values, while
// FixedRange mode clamps values to opts.Min and opts.Max.
func RenderSparkline(values []float64, opts SparklineOptions) string {
	width := optionWidth(opts.Width)
	if len(values) == 0 {
		return ""
	}
	if len(values) > width {
		values = values[len(values)-width:]
	}

	minimum, maximum, flat := sparkRange(values, opts)
	spark := make([]rune, len(values))
	for i, value := range values {
		spark[i] = sparkGlyph(value, minimum, maximum, flat)
	}

	out := string(spark)
	if !opts.ANSI {
		return out
	}
	code := opts.BackgroundANSI
	if code == "" {
		code = "100"
	}
	return "\x1b[" + code + "m" + out + "\x1b[0m"
}

// RenderPercentSparkline renders values on a fixed 0-100 scale. It is the
// options-based successor to PercentSparkline for future call sites.
func RenderPercentSparkline(values []float64, opts SparklineOptions) string {
	opts.FixedRange = true
	opts.Min = 0
	opts.Max = 100
	return RenderSparkline(values, opts)
}

// FormatPercent formats a percent value with clamping and a trailing percent
// sign. It is useful for callers that want a label next to RenderBar output.
func FormatPercent(percent float64, precision int) string {
	if precision < 0 {
		precision = 0
	}
	if math.IsNaN(percent) || percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return fmt.Sprintf("%.*f%%", precision, percent)
}

func optionWidth(width int) int {
	switch {
	case width == 0:
		return MaxWidth
	case width < 0:
		return 1
	default:
		return width
	}
}

func barRange(minimum, maximum float64) (float64, float64) {
	if minimum == 0 && maximum == 0 {
		return 0, 100
	}
	if !isFinite(minimum) || !isFinite(maximum) || maximum <= minimum {
		return 0, 100
	}
	return minimum, maximum
}

func normalizedPercent(value, minimum, maximum float64) float64 {
	if math.IsNaN(value) {
		value = minimum
	}
	if value < minimum {
		value = minimum
	}
	if value > maximum {
		value = maximum
	}
	return ((value - minimum) / (maximum - minimum)) * 100
}

func sparkRange(values []float64, opts SparklineOptions) (float64, float64, bool) {
	if opts.FixedRange {
		if isFinite(opts.Min) && isFinite(opts.Max) && opts.Max > opts.Min {
			return opts.Min, opts.Max, false
		}
		return 0, 0, true
	}

	var minimum, maximum float64
	haveFinite := false
	for _, value := range values {
		if !isFinite(value) {
			continue
		}
		if !haveFinite {
			minimum = value
			maximum = value
			haveFinite = true
			continue
		}
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	if !haveFinite || maximum == minimum {
		return 0, 0, true
	}
	return minimum, maximum, false
}

func sparkGlyph(value, minimum, maximum float64, flat bool) rune {
	if flat {
		return percentSparkChars[(len(percentSparkChars)-1)/2]
	}
	if math.IsNaN(value) || value < minimum {
		value = minimum
	}
	if value > maximum {
		value = maximum
	}
	idx := int(((value - minimum) / (maximum - minimum)) * float64(len(percentSparkChars)-1))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(percentSparkChars) {
		idx = len(percentSparkChars) - 1
	}
	return percentSparkChars[idx]
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
