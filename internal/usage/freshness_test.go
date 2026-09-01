package usage

import (
	"testing"
	"time"
)

// TestFreshnessFraction covers issue 131's elapsed/remaining -> fraction
// mapping at ~100%, ~50%, ~0%, and overdue/negative remaining, which must
// clamp to 0 rather than going negative or wrapping.
func TestFreshnessFraction(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		lastRefreshed time.Time
		wantMin       float64
		wantMax       float64
	}{
		{
			name:          "just refreshed: full gauge",
			lastRefreshed: now,
			wantMin:       0.99,
			wantMax:       1.0,
		},
		{
			name:          "halfway through the interval",
			lastRefreshed: now.Add(-DefaultCollectorInterval / 2),
			wantMin:       0.49,
			wantMax:       0.51,
		},
		{
			name:          "right at the interval boundary: empty gauge",
			lastRefreshed: now.Add(-DefaultCollectorInterval),
			wantMin:       0,
			wantMax:       0,
		},
		{
			name:          "overdue: elapsed well past the interval clamps to empty, not negative",
			lastRefreshed: now.Add(-2 * DefaultCollectorInterval),
			wantMin:       0,
			wantMax:       0,
		},
		{
			name:          "zero LastRefreshed (never refreshed) clamps to empty",
			lastRefreshed: time.Time{},
			wantMin:       0,
			wantMax:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := freshnessFraction(tt.lastRefreshed, now)
			if got < 0 || got > 1 {
				t.Fatalf("freshnessFraction() = %v, want a value clamped to [0,1]", got)
			}
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("freshnessFraction() = %v, want in [%v, %v]", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestFreshnessGauge covers the fraction -> glyph mapping at the same
// sample points, including out-of-range fractions clamping instead of
// panicking, and asserts the result is always exactly 3 visible runes so a
// label loses exactly 3 characters, never more or fewer.
func TestFreshnessGauge(t *testing.T) {
	tests := []struct {
		name     string
		fraction float64
		want     string
	}{
		{"full", 1.0, "[█]"},
		{"half", 0.5, "[▄]"},
		{"empty", 0.0, "[▁]"},
		{"clamps above 1", 1.5, "[█]"},
		{"clamps below 0 (overdue)", -0.5, "[▁]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := freshnessGauge(tt.fraction)
			if got != tt.want {
				t.Errorf("freshnessGauge(%v) = %q, want %q", tt.fraction, got, tt.want)
			}
			if n := len([]rune(got)); n != 3 {
				t.Errorf("freshnessGauge(%v) = %q, want exactly 3 runes, got %d", tt.fraction, got, n)
			}
		})
	}
}

// TestFreshnessOverlayLabel covers issue 131's acceptance criterion that a
// row label loses exactly 3 characters of its own text — not padding, and
// with no change in overall visible width — to the countdown gauge.
func TestFreshnessOverlayLabel(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)

	t.Run("puts the glyph first and keeps the label width intact", func(t *testing.T) {
		got := freshnessOverlayLabel("Gemini", now, now)
		want := "█ Gem…"
		if got != want {
			t.Errorf("freshnessOverlayLabel() = %q, want %q", got, want)
		}
		if n := len([]rune(got)); n != len([]rune("Gemini")) {
			t.Errorf("freshnessOverlayLabel() changed label width: got %d runes, want %d", n, len([]rune("Gemini")))
		}
	})

	t.Run("overdue agent drains to the empty glyph", func(t *testing.T) {
		got := freshnessOverlayLabel("Weekly", now.Add(-2*DefaultCollectorInterval), now)
		want := "▁ Wee…"
		if got != want {
			t.Errorf("freshnessOverlayLabel() = %q, want %q", got, want)
		}
	})

	t.Run("label with 3 or fewer runes is replaced in full", func(t *testing.T) {
		got := freshnessOverlayLabel("5h", now, now)
		if got != "█" {
			t.Errorf("freshnessOverlayLabel() = %q, want %q", got, "█")
		}
	})
}
