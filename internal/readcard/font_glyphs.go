package readcard

// buildFont5x8 builds the 5x8 retro pixel font (6x8 cell: 5x7 glyph + 1px spacing).
func buildFont5x8() *MonospaceFont {
	m := make(map[rune][]byte, 128)
	for _, table := range []map[rune][]uint8{specFont5x8, specFont5x8Extensions} {
		for r, rows := range table {
			glyph := make([]byte, 8)
			for i, rowBits := range rows {
				if i < 8 {
					glyph[i] = byte(rowBits << 2)
				}
			}
			m[r] = glyph
		}
	}
	return &MonospaceFont{
		Name:       "RetroPixel5x8",
		CharWidth:  6,
		CharHeight: 8,
		Ascent:     7,
		Glyphs:     m,
	}
}

// buildFont3x5 builds the 3x5 micro pixel font (4x6 cell: 3x5 glyph + 1px spacing).
func buildFont3x5() *MonospaceFont {
	m := make(map[rune][]byte, 128)
	for r, rows := range specFontTables["micro_pixel_3x5"] {
		glyph := make([]byte, 6)
		for i, rowBits := range rows {
			if i < 5 {
				glyph[i] = byte(rowBitsToByte(rowBits) << 5)
			}
		}
		m[r] = glyph
	}
	// Box drawing and symbol extensions

	for r, glyph := range specFontExtensions["micro_pixel_3x5"] {
		m[r] = glyph
	}
	return &MonospaceFont{
		Name:       "MicroPixel3x5",
		CharWidth:  4,
		CharHeight: 6,
		Ascent:     5,
		Glyphs:     m,
	}
}

func rowBitsToByte(row string) uint8 {
	var bits uint8
	for _, bit := range row {
		bits <<= 1
		if bit == '1' {
			bits |= 1
		}
	}
	return bits
}

func rowsToBytes(rows []string) []byte {
	result := make([]byte, len(rows))
	for i, row := range rows {
		result[i] = rowBitsToByte(row)
	}
	return result
}

// buildFont6x12 builds the 6x12 retro console font (7x12 cell).
func buildFont6x12() *MonospaceFont {
	m := make(map[rune][]byte, 128)
	for r, rows := range specFont5x8 {
		glyph := make([]byte, 12)
		for i, rowBits := range rows {
			if i < 7 {
				glyph[i+2] = byte(rowBits << 2)
			}
		}
		m[r] = glyph
	}

	for r, glyph := range specFontExtensions["retro_pixel_6x12"] {
		m[r] = glyph
	}
	return &MonospaceFont{
		Name:       "RetroPixel6x12",
		CharWidth:  7,
		CharHeight: 12,
		Ascent:     9,
		Glyphs:     m,
	}
}

// buildFont8x16 builds the 8x16 monospace font glyph table.
func buildFont8x16() *MonospaceFont {
	m := make(map[rune][]byte, 128)
	// Base standard ASCII 32..126 in 8x16 format (16 bytes per glyph)
	for r, rows := range specFontTables["monospace_8x16"] {
		m[r] = rowsToBytes(rows)
	}
	// Common box-drawing / symbols

	for r, glyph := range specFontExtensions["monospace_8x16"] {
		m[r] = glyph
	}
	return &MonospaceFont{
		Name:       "Monospace8x16",
		CharWidth:  8,
		CharHeight: 16,
		Ascent:     12,
		Glyphs:     m,
	}
}

// buildFont7x13 builds the 7x13 monospace font glyph table.
func buildFont7x13() *MonospaceFont {
	m := make(map[rune][]byte, 128)
	// Derive 7x13 from 8x16 by cropping/sampling 13 rows and shifting bits
	for r, rows := range specFontTables["monospace_8x16"] {
		raw := rowsToBytes(rows)
		scaled := make([]byte, 13)
		rowMap := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
		for i, srcRow := range rowMap {
			if srcRow < len(raw) {
				scaled[i] = raw[srcRow]
			}
		}
		m[r] = scaled
	}

	for r, glyph := range specFontExtensions["monospace_7x13"] {
		m[r] = glyph
	}
	return &MonospaceFont{
		Name:       "Monospace7x13",
		CharWidth:  7,
		CharHeight: 13,
		Ascent:     10,
		Glyphs:     m,
	}
}
