package assess

import (
	"math"
	"strings"
)

// BrailleOptions configures RenderBrailleSparkline.
type BrailleOptions struct {
	// Width is the exact number of terminal cells to render (default 10).
	// With 2 samples per cell, this consumes/resamples to Width * 2 samples.
	Width int

	// Color enables ANSI delta tinting:
	// Green (\x1b[32m) for growth (v_{i+1} > v_i)
	// Red (\x1b[31m) for pruning/removals (v_{i+1} < v_i)
	// Default/no color for flat (v_{i+1} == v_i)
	Color bool

	// FixedRange uses Min and Max instead of calculating min/max from values.
	FixedRange bool
	Min        float64
	Max        float64
}

// ResampleSeries resamples or pads/truncates a series of float64/int values to exactly targetCount points.
// If input is empty, returns targetCount zeros.
// If input has 1 element, returns targetCount identical elements.
// If input length == targetCount, returns a copy of input.
// If input length > 0, performs linear interpolation / index mapping across targetCount points.
func ResampleSeries(values []float64, targetCount int) []float64 {
	if targetCount <= 0 {
		return nil
	}
	out := make([]float64, targetCount)
	if len(values) == 0 {
		return out
	}
	if len(values) == 1 {
		for i := 0; i < targetCount; i++ {
			out[i] = values[0]
		}
		return out
	}
	if len(values) == targetCount {
		copy(out, values)
		return out
	}

	// Linear interpolation across the dataset
	n := len(values)
	for i := 0; i < targetCount; i++ {
		// Map i in [0, targetCount-1] to [0, n-1]
		t := float64(i) * float64(n-1) / float64(targetCount-1)
		low := int(math.Floor(t))
		high := int(math.Ceil(t))
		if low >= n {
			low = n - 1
		}
		if high >= n {
			high = n - 1
		}
		if low == high {
			out[i] = values[low]
		} else {
			frac := t - float64(low)
			out[i] = values[low]*(1.0-frac) + values[high]*frac
		}
	}
	return out
}

// BrailleGlyph maps left and right vertical levels (0..4) to a Unicode Braille rune (0x2800 - 0x28FF).
// Left column dots: 1, 2, 3, 7 (values 0..4 map to 0, dot 7, 7+3, 7+3+2, 7+3+2+1).
// Right column dots: 4, 5, 6, 8 (values 0..4 map to 0, dot 8, 8+6, 8+6+5, 8+6+5+4).
// Level 0 gives 0 dots.
// Baseline at level 1 for both gives ⣀ (0x28C0, dots 7 and 8).
// Max height at level 4 for both gives ⣿ (0x28FF, all 8 dots).
func BrailleGlyph(leftLevel, rightLevel int) rune {
	if leftLevel < 0 {
		leftLevel = 0
	}
	if leftLevel > 4 {
		leftLevel = 4
	}
	if rightLevel < 0 {
		rightLevel = 0
	}
	if rightLevel > 4 {
		rightLevel = 4
	}

	leftBits := [...]rune{
		0,
		0x40,               // dot 7 (1<<6)
		0x40 | 0x04,        // dots 7, 3 (1<<6 | 1<<2)
		0x40 | 0x04 | 0x02, // dots 7, 3, 2 (1<<6 | 1<<2 | 1<<1)
		0x40 | 0x04 | 0x02 | 0x01, // dots 7, 3, 2, 1 (1<<6 | 1<<2 | 1<<1 | 1<<0)
	}

	rightBits := [...]rune{
		0,
		0x80,               // dot 8 (1<<7)
		0x80 | 0x20,        // dots 8, 6 (1<<7 | 1<<5)
		0x80 | 0x20 | 0x10, // dots 8, 6, 5 (1<<7 | 1<<5 | 1<<4)
		0x80 | 0x20 | 0x10 | 0x08, // dots 8, 6, 5, 4 (1<<7 | 1<<5 | 1<<4 | 1<<3)
	}

	bits := leftBits[leftLevel] | rightBits[rightLevel]
	return 0x2800 + bits
}

// BrailleColumnLevel maps a continuous value within [minimum, maximum] to a level 1..4.
// When flat or all zeros/minimum, returns baseline level 1.
func BrailleColumnLevel(value, minimum, maximum float64, flat bool) int {
	if flat || maximum <= minimum {
		return 1
	}
	if math.IsNaN(value) || math.IsInf(value, -1) || value <= minimum {
		return 1
	}
	if math.IsInf(value, 1) || value >= maximum {
		return 4
	}

	// Normalizes (minimum, maximum] into (0, 0.25] -> 1, (0.25, 0.5] -> 2, (0.5, 0.75] -> 3, (0.75, 1.0] -> 4
	// Notice: value == minimum already returned 1.
	// For value > minimum:
	level := int(math.Ceil(((value - minimum) / (maximum - minimum)) * 4))
	if level < 1 {
		return 1
	}
	if level > 4 {
		return 4
	}
	return level
}

// RenderBrailleSparkline renders a double-resolution 2-samples-per-cell Braille sparkline.
func RenderBrailleSparkline(values []float64, opts BrailleOptions) string {
	width := opts.Width
	if width <= 0 {
		width = 10
	}

	targetSamples := width * 2
	samples := ResampleSeries(values, targetSamples)

	// Determine min and max
	var minVal, maxVal float64
	flat := false

	if opts.FixedRange && opts.Max > opts.Min {
		minVal = opts.Min
		maxVal = opts.Max
	} else {
		if len(samples) == 0 {
			minVal = 0
			maxVal = 0
			flat = true
		} else {
			minVal = samples[0]
			maxVal = samples[0]
			for _, v := range samples {
				if v < minVal {
					minVal = v
				}
				if v > maxVal {
					maxVal = v
				}
			}
			if maxVal == minVal {
				flat = true
			}
		}
	}

	var sb strings.Builder
	for cell := 0; cell < width; cell++ {
		vLeft := samples[cell*2]
		vRight := samples[cell*2+1]

		lvlLeft := BrailleColumnLevel(vLeft, minVal, maxVal, flat)
		lvlRight := BrailleColumnLevel(vRight, minVal, maxVal, flat)

		glyph := BrailleGlyph(lvlLeft, lvlRight)

		if opts.Color {
			if vRight > vLeft {
				// Growth / additions -> Green (\x1b[32m)
				sb.WriteString("\x1b[32m")
				sb.WriteRune(glyph)
				sb.WriteString("\x1b[0m")
			} else if vRight < vLeft {
				// Removals / reduction -> Red (\x1b[31m)
				sb.WriteString("\x1b[31m")
				sb.WriteRune(glyph)
				sb.WriteString("\x1b[0m")
			} else {
				// Flat -> muted / default
				sb.WriteRune(glyph)
			}
		} else {
			sb.WriteRune(glyph)
		}
	}

	return sb.String()
}
