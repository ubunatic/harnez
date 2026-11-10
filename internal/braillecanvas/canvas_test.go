package braillecanvas

import (
	"strings"
	"testing"
)

func TestCanvasDimensions(t *testing.T) {
	c := New(10, 5)
	if c.WidthCells() != 10 || c.HeightCells() != 5 {
		t.Fatalf("expected 10x5 cells, got %dx%d", c.WidthCells(), c.HeightCells())
	}
	if c.WidthDots() != 20 || c.HeightDots() != 20 {
		t.Fatalf("expected 20x20 dots, got %dx%d", c.WidthDots(), c.HeightDots())
	}
}

func TestCanvasBrailleDots(t *testing.T) {
	// A 1x1 canvas:
	c := New(1, 1)

	// In 1x1 cell:
	// px in 0..1, py in 0..3
	// (0, 3) is top-left dot (dot 1: 0x01)
	c.Set(0, 3)
	got := c.Render(false)
	wantRune := rune(0x2800 + 0x01) // ⠁
	if got != string(wantRune) {
		t.Fatalf("expected %q, got %q", string(wantRune), got)
	}

	// Add bottom-right dot (dot 8: 0x80) -> px=1, py=0
	c.Set(1, 0)
	got = c.Render(false)
	wantRune = rune(0x2800 + 0x01 + 0x80) // ⢁
	if got != string(wantRune) {
		t.Fatalf("expected %q, got %q", string(wantRune), got)
	}

	// Add all remaining dots
	c.Set(0, 2) // dot 2 (0x02)
	c.Set(0, 1) // dot 3 (0x04)
	c.Set(0, 0) // dot 7 (0x40)
	c.Set(1, 3) // dot 4 (0x08)
	c.Set(1, 2) // dot 5 (0x10)
	c.Set(1, 1) // dot 6 (0x20)
	got = c.Render(false)
	wantRune = rune(0x2800 + 0xFF) // ⣿
	if got != string(wantRune) {
		t.Fatalf("expected full Braille %q, got %q", string(wantRune), got)
	}
}

func TestCanvasBresenhamLine(t *testing.T) {
	c := New(5, 5)
	c.Line(0, 0, 9, 19)
	lines := c.RenderLines(false)
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
	foundDots := 0
	for _, l := range lines {
		for _, r := range l {
			if r != ' ' {
				foundDots++
			}
		}
	}
	if foundDots == 0 {
		t.Fatal("expected line dots to be drawn")
	}
}

func TestCanvasTextAndColor(t *testing.T) {
	c := New(10, 2)
	c.Text(2, 0, "test", "32")
	outColor := c.Render(true)
	if !strings.Contains(outColor, "\x1b[32mtest\x1b[0m") {
		t.Fatalf("expected colored text, got %q", outColor)
	}
	outNoColor := c.Render(false)
	if strings.Contains(outNoColor, "\x1b") {
		t.Fatalf("expected no escape codes, got %q", outNoColor)
	}
	if !strings.Contains(outNoColor, "  test    ") {
		t.Fatalf("expected text in rendered output, got %q", outNoColor)
	}
}

func TestCanvasOutOfBounds(t *testing.T) {
	c := New(2, 2)
	// Should not panic on out-of-bounds coordinates
	c.Set(-1, -1)
	c.Set(100, 100)
	c.Line(-10, -10, 100, 100)
	c.Text(-5, 0, "hello")
	c.Text(10, 0, "world")
}

func TestCanvasIsOccupied(t *testing.T) {
	c := New(10, 5)

	// Empty canvas
	if c.IsOccupied(0, 0, 4) {
		t.Fatal("expected empty canvas cells not to be occupied")
	}

	// Place text at (2, 1) length 4
	c.Text(2, 1, "test")
	if !c.IsOccupied(2, 1, 4) {
		t.Fatal("expected text cell to be occupied")
	}
	if !c.IsOccupied(1, 1, 3) { // overlapping cell (2, 1)
		t.Fatal("expected overlapping text cell to be occupied")
	}
	if c.IsOccupied(2, 0, 4) { // different row
		t.Fatal("expected different row not to be occupied by text")
	}

	// Set Braille dot in cell (5, 3) -> cellY = 3, so cellYFromBottom = (5-1) - 3 = 1 -> py = 1*4 = 4
	// cellX = 5 -> px = 5*2 = 10
	c.Set(10, 4)
	if !c.IsOccupied(5, 3, 1) {
		t.Fatal("expected cell with Braille dot to be occupied")
	}
	if !c.IsOccupied(4, 3, 2) {
		t.Fatal("expected range overlapping Braille dot to be occupied")
	}

	// Boundary check
	if !c.IsOccupied(8, 0, 5) {
		t.Fatal("expected out-of-bounds range to be reported as occupied")
	}
}
