package readcard

func newSpecFont(name, size string, width, height, ascent int) *MonospaceFont {
	return &MonospaceFont{
		Name:       name,
		CharWidth:  width,
		CharHeight: height,
		Ascent:     ascent,
		loadGlyphs: func() map[rune][]byte {
			return glyphBitmaps(size, width, height)
		},
	}
}

// buildFont5x8 builds the 5x8 retro pixel font (6x8 cell: 5x7 glyph + 1px spacing).
func buildFont5x8() *MonospaceFont { return newSpecFont("RetroPixel5x8", "5x8", 6, 8, 7) }

// buildFont3x5 builds the 3x5 micro pixel font (4x6 cell: 3x5 glyph + 1px spacing).
func buildFont3x5() *MonospaceFont { return newSpecFont("MicroPixel3x5", "3x5", 4, 6, 5) }

// buildFontDot8 builds the dedicated 3x6 Dot8 font for 3px cell rendering.
func buildFontDot8() *MonospaceFont { return newSpecFont("Dot8_3x6", "dot8", 3, 6, 5) }

// buildFont6x12 builds the 6x12 retro console font (7x12 cell).
func buildFont6x12() *MonospaceFont { return newSpecFont("RetroPixel6x12", "6x12", 7, 12, 9) }

// buildFont8x16 builds the 8x16 monospace font.
func buildFont8x16() *MonospaceFont { return newSpecFont("Monospace8x16", "8x16", 8, 16, 12) }

// buildFont7x13 builds the 7x13 monospace font.
func buildFont7x13() *MonospaceFont { return newSpecFont("Monospace7x13", "7x13", 7, 13, 10) }
