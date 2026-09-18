package readcard

import "image/color"

// unicodeGlyphs groups compact, high-value symbols commonly emitted by
// terminal UIs, dashboards, code comments, and command output. Patterns use a
// 5x7 logical grid and are scaled to the active bitmap font cell.
var unicodeGlyphs = map[rune][]string{
	// Terminal and box-drawing companions.
	'┈': {"     ", "     ", "     ", "10101", "     ", "     ", "     "},
	'┄': {"     ", "     ", "     ", "10101", "     ", "     ", "     "},
	'╭': {" 111 ", " 1   ", " 1   ", "     ", "     ", "     ", "     "},
	'╮': {" 111 ", "   1 ", "   1 ", "     ", "     ", "     ", "     "},
	'╰': {"     ", "     ", "     ", " 1   ", " 1   ", " 111 ", "     "},
	'╯': {"     ", "     ", "     ", "   1 ", "   1 ", " 111 ", "     "},
	// Arrows and flow indicators.
	'↑': {"  1  ", " 111 ", " 1 1 ", "   1 ", "   1 ", "   1 ", "     "},
	'↓': {"     ", "   1 ", "   1 ", "   1 ", " 1 1 ", " 111 ", "  1  "},
	'↔': {"     ", "     ", "1 1 1", " 111 ", "1 1 1", "     ", "     "},
	'⇒': {"    1", "   11", "11111", "   11", "    1", "     ", "     "},
	'⇐': {"1    ", "11   ", "11111", "11   ", "1    ", "     ", "     "},
	// Math and measurement.
	'±': {"  1  ", " 111 ", "  1  ", "     ", " 111 ", "     ", "     "},
	'×': {"     ", "1   1", " 1 1 ", "  1  ", " 1 1 ", "1   1", "     "},
	'÷': {"     ", "  1  ", "     ", "11111", "     ", "  1  ", "     "},
	'≠': {"1   1", " 111 ", "11111", " 111 ", "1   1", "     ", "     "},
	'≤': {"   1 ", "  1  ", " 1   ", "  1  ", "   1 ", "11111", "     "},
	'≥': {" 1   ", "  1  ", "   1 ", "  1  ", " 1   ", "11111", "     "},
	// Typography and status.
	'–': {"     ", "     ", "     ", "11111", "     ", "     ", "     "},
	'—': {"     ", "     ", "     ", "11111", "11111", "     ", "     "},
	'“': {"1 1  ", "1 1  ", "     ", "     ", "     ", "     ", "     "},
	'”': {"  1 1", "  1 1", "     ", "     ", "     ", "     ", "     "},
	'⚠': {"  1  ", " 1 1 ", " 111 ", "11111", "1 1 1", "1   1", "     "},
	'✦': {"  1  ", "1 1 1", " 111 ", "11111", " 111 ", "1 1 1", "  1  "},
}

func drawUnicodePattern(img interface{ SetRGBA(x, y int, c color.RGBA) }, pattern []string, x, y int, col color.RGBA, width, height int) {
	if len(pattern) == 0 {
		return
	}
	for py, row := range pattern {
		for px, bit := range row {
			if bit != '1' {
				continue
			}
			dx := x + px*width/5
			dy := y + py*height/7
			img.SetRGBA(dx, dy, col)
		}
	}
}
