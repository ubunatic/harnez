package braillecanvas

import (
	"strings"
	"unicode/utf8"
)

// Canvas represents a 2D Braille character-grid canvas.
// Each character cell represents a 2x4 dot matrix (2 horizontal x 4 vertical dots).
// (px, py) dot coordinates have (0, 0) at the bottom-left.
type Canvas struct {
	widthCells  int
	heightCells int
	dots        [][]uint8
	cellColors  [][]string
	textCells   [][]rune
	textColors  [][]string
}

// New creates a new Braille canvas of the given cell dimensions.
func New(widthCells, heightCells int) *Canvas {
	if widthCells < 1 {
		widthCells = 1
	}
	if heightCells < 1 {
		heightCells = 1
	}
	dots := make([][]uint8, heightCells)
	cellColors := make([][]string, heightCells)
	textCells := make([][]rune, heightCells)
	textColors := make([][]string, heightCells)
	for y := 0; y < heightCells; y++ {
		dots[y] = make([]uint8, widthCells)
		cellColors[y] = make([]string, widthCells)
		textCells[y] = make([]rune, widthCells)
		textColors[y] = make([]string, widthCells)
	}
	return &Canvas{
		widthCells:  widthCells,
		heightCells: heightCells,
		dots:        dots,
		cellColors:  cellColors,
		textCells:   textCells,
		textColors:  textColors,
	}
}

// WidthCells returns the width in character cells.
func (c *Canvas) WidthCells() int { return c.widthCells }

// HeightCells returns the height in character cells.
func (c *Canvas) HeightCells() int { return c.heightCells }

// WidthDots returns the pixel/dot width (2 dots per cell).
func (c *Canvas) WidthDots() int { return c.widthCells * 2 }

// HeightDots returns the pixel/dot height (4 dots per cell).
func (c *Canvas) HeightDots() int { return c.heightCells * 4 }

func dotMask(dx, dy int) uint8 {
	// dx: 0=left, 1=right
	// dy: 0=bottom, 1=mid-down, 2=mid-up, 3=top
	switch dy {
	case 3: // top
		if dx == 0 {
			return 0x01 // dot 1
		}
		return 0x08 // dot 4
	case 2: // mid-up
		if dx == 0 {
			return 0x02 // dot 2
		}
		return 0x10 // dot 5
	case 1: // mid-down
		if dx == 0 {
			return 0x04 // dot 3
		}
		return 0x20 // dot 6
	case 0: // bottom
		if dx == 0 {
			return 0x40 // dot 7
		}
		return 0x80 // dot 8
	default:
		return 0
	}
}

// Set activates the dot at (px, py).
// (0, 0) is bottom-left; (WidthDots()-1, HeightDots()-1) is top-right.
func (c *Canvas) Set(px, py int, colorCode ...string) {
	if px < 0 || px >= c.WidthDots() || py < 0 || py >= c.HeightDots() {
		return
	}
	cellX := px / 2
	dx := px % 2
	cellYFromBottom := py / 4
	cellY := (c.heightCells - 1) - cellYFromBottom
	dy := py % 4

	c.dots[cellY][cellX] |= dotMask(dx, dy)
	if len(colorCode) > 0 && colorCode[0] != "" {
		c.cellColors[cellY][cellX] = colorCode[0]
	}
}

// Line draws a 1-pixel Braille line from (x0, y0) to (x1, y1) using Bresenham's algorithm.
func (c *Canvas) Line(x0, y0, x1, y1 int, colorCode ...string) {
	dx := x1 - x0
	if dx < 0 {
		dx = -dx
	}
	dy := y1 - y0
	if dy < 0 {
		dy = -dy
	}
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy

	for {
		c.Set(x0, y0, colorCode...)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// IsOccupied checks if any character cell from (cellX, cellY) to (cellX+length-1, cellY) contains text or Braille dots.
func (c *Canvas) IsOccupied(cellX, cellY, length int) bool {
	if cellY < 0 || cellY >= c.heightCells {
		return true
	}
	for i := 0; i < length; i++ {
		x := cellX + i
		if x < 0 || x >= c.widthCells {
			return true
		}
		if c.textCells[cellY][x] != 0 || c.dots[cellY][x] != 0 {
			return true
		}
	}
	return false
}

// HasText reports whether the character cell at (cellX, cellY) contains text.
func (c *Canvas) HasText(cellX, cellY int) bool {
	if cellX < 0 || cellX >= c.widthCells || cellY < 0 || cellY >= c.heightCells {
		return false
	}
	return c.textCells[cellY][cellX] != 0
}

// Text places string text starting at character cell (cellX, cellY), where cellY=0 is top row.
func (c *Canvas) Text(cellX, cellY int, text string, colorCode ...string) {
	if cellY < 0 || cellY >= c.heightCells {
		return
	}
	var color string
	if len(colorCode) > 0 {
		color = colorCode[0]
	}
	for _, r := range text {
		if cellX < 0 || cellX >= c.widthCells {
			cellX++
			continue
		}
		c.textCells[cellY][cellX] = r
		c.textColors[cellY][cellX] = color
		cellX++
	}
}

// RenderLines returns each row of the canvas as a string.
func (c *Canvas) RenderLines(colorEnabled bool) []string {
	lines := make([]string, c.heightCells)
	for y := 0; y < c.heightCells; y++ {
		var b strings.Builder
		currentColor := ""
		for x := 0; x < c.widthCells; x++ {
			var r rune
			var color string
			if c.textCells[y][x] != 0 {
				r = c.textCells[y][x]
				color = c.textColors[y][x]
			} else if c.dots[y][x] != 0 {
				r = rune(0x2800 + uint16(c.dots[y][x]))
				color = c.cellColors[y][x]
			} else {
				r = ' '
				color = ""
			}

			if colorEnabled {
				if color != currentColor {
					if currentColor != "" {
						b.WriteString("\x1b[0m")
					}
					if color != "" {
						b.WriteString("\x1b[" + color + "m")
					}
					currentColor = color
				}
			}
			b.WriteRune(r)
		}
		if colorEnabled && currentColor != "" {
			b.WriteString("\x1b[0m")
		}
		lines[y] = b.String()
	}
	return lines
}

// Render renders the canvas as a single multiline string.
func (c *Canvas) Render(colorEnabled bool) string {
	return strings.Join(c.RenderLines(colorEnabled), "\n")
}

// VisibleWidth returns the display column width of a string.
func VisibleWidth(s string) int {
	return utf8.RuneCountInString(stripANSI(s))
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
