package rograph

import "testing"

func TestPadLabel(t *testing.T) {
	tests := []struct {
		name  string
		label string
		width int
		want  string
	}{
		{"short label pads with spaces", "cpu (4 cores)", 16, "cpu (4 cores)   "},
		{"exact width unchanged", "1234567890123456", 16, "1234567890123456"},
		{"over width truncates with ellipsis", "12345678901234567", 16, "123456789012345…"},
		{"far over width truncates with ellipsis", "a very long window label name", 16, "a very long win…"},
		{"empty label pads fully", "", 16, "                "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadLabel(tt.label, tt.width)
			if got != tt.want {
				t.Errorf("PadLabel(%q, %d) = %q, want %q", tt.label, tt.width, got, tt.want)
			}
			if len([]rune(got)) != tt.width {
				t.Errorf("PadLabel(%q, %d) produced %d runes, want %d", tt.label, tt.width, len([]rune(got)), tt.width)
			}
		})
	}
}

// TestRowLayout locks in the agent-box row-layout contract established by
// issue 079: label(16) + " " + bar(barW+2) + " " + percent(6) + trailing,
// i.e. a fixed overhead of 26 for the current buildAgentBox call site, with
// the bar shrinking down to 1 and the trailing text dropped first when both
// don't fit.
func TestRowLayout(t *testing.T) {
	tests := []struct {
		name          string
		totalWidth    int
		trailingWidth int
		wantBarWidth  int
		wantKeep      bool
	}{
		{"ample room, bar clamps to MaxWidth", 100, 0, 10, true},
		{"ample room with trailing, bar clamps to MaxWidth", 100, 7, 10, true},
		{"bar shrinks below MaxWidth to fit trailing", 40, 7, 7, true},
		// At exactly the fixed overhead, the bar's own natural width is already
		// 0 (<1), so the same "not enough room" branch that drops trailing text
		// fires here too — harmless since there's no trailing text to drop, but
		// it does mean keep=false even though trailingWidth was already 0. This
		// matches the pre-extraction buildAgentBox logic exactly.
		{"exact fixed overhead, no trailing, bar floors at 1", 26, 0, 1, false},
		{"too tight for trailing, trailing dropped, stub bar", 20, 7, 1, false},
		{"negative headroom, trailing dropped, stub bar", 10, 7, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			barW, keep := RowLayout(tt.totalWidth, 16, 6, MaxWidth, tt.trailingWidth)
			if barW != tt.wantBarWidth || keep != tt.wantKeep {
				t.Errorf("RowLayout(%d, 16, 6, %d, %d) = (%d, %v), want (%d, %v)",
					tt.totalWidth, MaxWidth, tt.trailingWidth, barW, keep, tt.wantBarWidth, tt.wantKeep)
			}
		})
	}
}

func TestRowLayoutNeverPanicsOrGoesOutOfBounds(t *testing.T) {
	for _, total := range []int{-100, -1, 0, 1, 10, 26, 40, 100} {
		for _, trailing := range []int{-5, 0, 1, 7, 50} {
			barW, _ := RowLayout(total, 16, 6, MaxWidth, trailing)
			if barW < 1 || barW > MaxWidth {
				t.Errorf("RowLayout(%d, 16, 6, %d, %d) barWidth = %d, want in [1, %d]", total, MaxWidth, trailing, barW, MaxWidth)
			}
		}
	}
}
