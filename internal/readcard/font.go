package readcard

import (
	"image"
	"image/color"
	"unicode/utf8"
)

// MonospaceFont defines a bitmap font interface for rasterizing glyphs.
type MonospaceFont struct {
	Name       string
	CharWidth  int
	CharHeight int
	Ascent     int
	Glyphs     map[rune][]byte
}

// DrawRune draws a single rune onto img at pixel coordinate (x, y) using col.
func (f *MonospaceFont) DrawRune(img *image.RGBA, r rune, x, y int, col color.RGBA) {
	glyph, ok := f.Glyphs[r]
	if !ok {
		// Fallback for unknown characters: box or question mark
		if glyph, ok = f.Glyphs['?']; !ok {
			return
		}
	}

	bounds := img.Bounds()
	bytesPerRow := (f.CharWidth + 7) / 8

	for row := 0; row < f.CharHeight; row++ {
		py := y + row
		if py < bounds.Min.Y || py >= bounds.Max.Y {
			continue
		}
		if row >= len(glyph)/bytesPerRow {
			break
		}

		for c := 0; c < f.CharWidth; c++ {
			px := x + c
			if px < bounds.Min.X || px >= bounds.Max.X {
				continue
			}

			byteIdx := row*bytesPerRow + (c / 8)
			bitIdx := 7 - (c % 8)
			if byteIdx < len(glyph) && (glyph[byteIdx]&(1<<bitIdx)) != 0 {
				img.SetRGBA(px, py, col)
			}
		}
	}
}

// DrawString draws a single-line string of runes onto img at (x, y).
func (f *MonospaceFont) DrawString(img *image.RGBA, s string, x, y int, col color.RGBA) int {
	curX := x
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		if r == '\t' {
			// Tab expands to 4 spaces
			curX += f.CharWidth * 4
			continue
		}
		f.DrawRune(img, r, curX, y, col)
		curX += f.CharWidth
	}
	return curX - x
}

// DefaultFont8x16 provides a crisp 8x16 monospace font.
var DefaultFont8x16 = buildFont8x16()

// DefaultFont7x13 provides a dense 7x13 monospace font.
var DefaultFont7x13 = buildFont7x13()

// GetFont returns the best font matching the target font size (e.g. 10-16px).
func GetFont(fontSize int) *MonospaceFont {
	if fontSize <= 11 {
		return DefaultFont7x13
	}
	return DefaultFont8x16
}
