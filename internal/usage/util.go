package usage

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// MaskAccount obscures account identity (emails or usernames) for privacy in logs and terminal outputs.
func MaskAccount(account string) string {
	account = strings.TrimSpace(account)
	if account == "" {
		return ""
	}
	parts := strings.Split(account, "@")
	if len(parts) == 2 {
		user := parts[0]
		domain := parts[1]
		if len(user) <= 2 {
			return user[:1] + "***@" + domain
		}
		return user[:1] + "***" + user[len(user)-1:] + "@" + domain
	}
	if len(account) <= 4 {
		return "***"
	}
	return account[:2] + "***" + account[len(account)-2:]
}

// FormatDuration formats a time.Duration into human readable string like
// "4h 52m" or "32m". Durations of 48h or more switch to "<days>d <hours>h"
// since minute precision stops being useful that far out.
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}
	if d >= 48*time.Hour {
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

// RenderProgressBar generates an ANSI/Unicode progress bar of the given character width.
func RenderProgressBar(usedPercent float64, width int) string {
	if width <= 0 {
		width = 20
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

// FormatNumber formats an integer with thousand commas.
func FormatNumber(n int64) string {
	if n < 0 {
		return "-" + FormatNumber(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var res []string
	for len(s) > 3 {
		res = append([]string{s[len(s)-3:]}, res...)
		s = s[:len(s)-3]
	}
	if len(s) > 0 {
		res = append([]string{s}, res...)
	}
	return strings.Join(res, ",")
}

// TerminalWidth returns the detected terminal column width for out, or 90 if undetermined.
func TerminalWidth(out io.Writer) int {
	cols, _ := terminalSize(out)
	return cols
}

// SparkRunes are Unicode block runes (' ' through '█') for rendering sparklines.
var SparkRunes = []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// resampleValues downsamples a slice of float64 to target width using max aggregation in each bucket.
func resampleValues(values []float64, width int) []float64 {
	n := len(values)
	if width <= 0 || n <= width {
		return values
	}
	res := make([]float64, width)
	for i := 0; i < width; i++ {
		start := i * n / width
		end := (i + 1) * n / width
		if end <= start {
			end = start + 1
		}
		if end > n {
			end = n
		}
		maxV := values[start]
		for j := start + 1; j < end; j++ {
			if values[j] > maxV {
				maxV = values[j]
			}
		}
		res[i] = maxV
	}
	return res
}

// RenderSparklineWidth formats a slice of float64 time-series data into a Unicode sparkline.
// If width > 0 and len(values) > width, values are downsampled to width runes.
// Baseline is relative to the initial value (v0 = values[0]) so trajectory deltas are visible.
func RenderSparklineWidth(values []float64, width int) string {
	if len(values) == 0 {
		return ""
	}
	if width > 0 && len(values) > width {
		values = resampleValues(values, width)
	}

	minVal := values[0]
	maxVal := values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	var sb strings.Builder
	span := maxVal - minVal
	if span <= 0 {
		// All values identical:
		// If positive, render full block '█' (or baseline ' ' if 0 or negative)
		runeChar := SparkRunes[0]
		if maxVal > 0 {
			runeChar = SparkRunes[len(SparkRunes)-1]
		}
		for range values {
			sb.WriteRune(runeChar)
		}
		return sb.String()
	}

	for _, v := range values {
		delta := v - minVal
		if delta <= 0 {
			sb.WriteRune(SparkRunes[0])
			continue
		}
		idx := int((delta / span) * float64(len(SparkRunes)-1))
		if idx < 1 {
			idx = 1
		}
		if idx >= len(SparkRunes) {
			idx = len(SparkRunes) - 1
		}
		sb.WriteRune(SparkRunes[idx])
	}
	return sb.String()
}

// RenderSparkline formats a slice of float64 time-series data into an unconstrained Unicode sparkline.
func RenderSparkline(values []float64) string {
	return RenderSparklineWidth(values, 0)
}

// RenderSparklineInt64Width formats a slice of int64 time-series data into a Unicode sparkline of limited width.
func RenderSparklineInt64Width(values []int64, width int) string {
	if len(values) == 0 {
		return ""
	}
	f := make([]float64, len(values))
	for i, v := range values {
		f[i] = float64(v)
	}
	return RenderSparklineWidth(f, width)
}

// RenderSparklineInt64 formats a slice of int64 time-series data into a Unicode sparkline.
func RenderSparklineInt64(values []int64) string {
	return RenderSparklineInt64Width(values, 0)
}

