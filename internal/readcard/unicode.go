package readcard

import "image/color"

func drawUnicodePattern(img interface{ SetRGBA(x, y int, c color.RGBA) }, pattern []string, x, y int, col color.RGBA, width, height int) {
	drawGlyphPattern(img, pattern, x, y, col, width, height)
}
