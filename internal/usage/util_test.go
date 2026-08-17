package usage

import (
	"testing"
	"time"
)

func TestMaskAccount(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"a@b.com", "a***@b.com"},
		{"user@example.com", "u***r@example.com"},
		{"john.doe@company.org", "j***e@company.org"},
		{"admin", "ad***in"},
		{"ab", "***"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := MaskAccount(tt.input)
			if got != tt.want {
				t.Errorf("MaskAccount(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0m"},
		{-5 * time.Minute, "0m"},
		{45 * time.Minute, "45m"},
		{4*time.Hour + 52*time.Minute, "4h 52m"},
		{165*time.Hour + 5*time.Minute, "165h 5m"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatDuration(tt.d)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestRenderProgressBar(t *testing.T) {
	tests := []struct {
		pct   float64
		width int
		want  string
	}{
		{0, 10, "[░░░░░░░░░░]"},
		{50, 10, "[█████░░░░░]"},
		{100, 10, "[██████████]"},
		{120, 10, "[██████████]"},
		{-10, 10, "[░░░░░░░░░░]"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RenderProgressBar(tt.pct, tt.width)
			if got != tt.want {
				t.Errorf("RenderProgressBar(%v, %d) = %q, want %q", tt.pct, tt.width, got, tt.want)
			}
		})
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
		{-9876543, "-9,876,543"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatNumber(tt.n)
			if got != tt.want {
				t.Errorf("FormatNumber(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}
