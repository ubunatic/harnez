package usage

import (
	"strings"
	"testing"
)

// TestGridColumnsFitsWithGutters guards the layout bug that broke `--watch`:
// boxWidth was computed as total/columns, ignoring the boxGap columns that
// combineRow inserts between panels. At 100 columns that produced two 50-wide
// boxes plus a gutter = 101 cells, wrapping every box line onto a second
// physical row and corrupting the whole frame.
func TestGridColumnsFitsWithGutters(t *testing.T) {
	for usable := minTerminalWidth; usable <= 200; usable++ {
		for panels := 1; panels <= 6; panels++ {
			columns, boxWidth := gridColumns(usable, panels)
			if columns < 1 || columns > panels {
				t.Fatalf("usable=%d panels=%d: columns=%d out of range", usable, panels, columns)
			}
			if columns == 1 {
				continue
			}
			total := columns*boxWidth + (columns-1)*boxGap
			if total > usable {
				t.Errorf("usable=%d panels=%d: row width %d exceeds usable (columns=%d boxWidth=%d)",
					usable, panels, total, columns, boxWidth)
			}
		}
	}
}

// TestRenderWBoxWidth checks a rendered panel occupies exactly its declared
// width on every line, so side-by-side rows line up.
func TestRenderWBoxWidth(t *testing.T) {
	b := wbox{
		title: "\x1b[1m[C]\x1b[0m Claude Code",
		lines: []string{"short", strings.Repeat("x", 200), ""},
		width: 48,
	}
	for i, l := range renderWBox(b) {
		if got := visLen(l); got != b.width {
			t.Errorf("line %d: visible width %d, want %d (%q)", i, got, b.width, stripANSI(l))
		}
	}
}

// TestFitClipsToViewport ensures a frame can never exceed the terminal box.
// An overflowing frame scrolls the terminal, after which the `\x1b[H` starting
// each redraw no longer addresses the row the program assumes.
func TestFitClipsToViewport(t *testing.T) {
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = strings.Repeat("y", 300)
	}
	f := fit(lines, 60, 10)
	if len(f.lines) != 10 {
		t.Fatalf("got %d lines, want 10", len(f.lines))
	}
	for i, l := range f.lines {
		if visLen(l) > 60 {
			t.Errorf("line %d: visible width %d exceeds 60", i, visLen(l))
		}
	}
}

// TestPaintClearsEachLine guards the stale-content bug: `\x1b[H` + content +
// `\x1b[J` clears only *below* the new frame, so shrinking the frame left the
// previous, larger frame's tails on every shortened or blank line. Every line
// must therefore carry its own erase-to-end-of-line.
func TestPaintClearsEachLine(t *testing.T) {
	var sb strings.Builder
	fit([]string{"a", "", "c"}, 20, 5).paint(&sb)
	out := sb.String()

	if !strings.HasPrefix(out, "\x1b[H") {
		t.Errorf("frame must start by homing the cursor, got %q", out)
	}
	if n := strings.Count(out, "\x1b[K"); n != 3 {
		t.Errorf("got %d erase-to-end-of-line sequences, want one per line (3): %q", n, out)
	}
	if !strings.HasSuffix(out, "\x1b[J") {
		t.Errorf("frame must end by erasing below the content, got %q", out)
	}
	// A trailing newline after the final line would scroll a full-height frame.
	if strings.HasSuffix(strings.TrimSuffix(out, "\x1b[J"), "\n") {
		t.Errorf("frame must not end with a newline: %q", out)
	}
}
