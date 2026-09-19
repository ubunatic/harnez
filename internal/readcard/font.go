package readcard

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"sync"
	"unicode/utf8"
)

// MonospaceFont defines a bitmap font interface for rasterizing glyphs.
type MonospaceFont struct {
	Name       string
	CharWidth  int
	CharHeight int
	Ascent     int
	Glyphs     map[rune][]byte
	loadGlyphs func() map[rune][]byte
	glyphsOnce sync.Once
}

// DrawRune draws a single rune onto img at pixel coordinate (x, y) using col.
func (f *MonospaceFont) DrawRune(img *image.RGBA, r rune, x, y int, col color.RGBA) {
	if r >= 0x2800 && r <= 0x28ff {
		drawBrailleRune(img, r, x, y, col, f.CharWidth, f.CharHeight)
		return
	}
	f.glyphsOnce.Do(func() { f.Glyphs = f.loadGlyphs() })
	glyph, ok := f.Glyphs[r]
	if !ok {
		// Fallback for unknown characters: upstream or spec question mark.
		if glyph, ok = f.Glyphs['?']; !ok {
			return
		}
	}

	drawBitmapGlyph(img, glyph, x, y, col, f.CharWidth, f.CharHeight)
}

func drawBitmapGlyph(img *image.RGBA, glyph []byte, x, y int, col color.RGBA, width, height int) {
	bounds := img.Bounds()
	bytesPerRow := (width + 7) / 8

	for row := 0; row < height; row++ {
		py := y + row
		if py < bounds.Min.Y || py >= bounds.Max.Y {
			continue
		}
		if row >= len(glyph)/bytesPerRow {
			break
		}

		for c := 0; c < width; c++ {
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

// drawBrailleRune rasterizes a Unicode Braille cell directly. Bitmap fonts do
// not have room for the entire Unicode glyph block, but Braille's dot layout
// is simple enough to preserve at every card font size.
func drawBrailleRune(img *image.RGBA, r rune, x, y int, col color.RGBA, width, height int) {
	if r == 0x2800 {
		return
	}

	bits := byte(r - 0x2800)
	dotX := []int{0, 0, 0, 1, 1, 1, 0, 1}
	dotY := []int{0, 1, 2, 0, 1, 2, 3, 3}
	marginX := 0
	if width > 2 {
		marginX = 1
	}
	marginY := 0
	if height > 2 {
		marginY = 1
	}
	spanX := width - 1 - 2*marginX
	spanY := height - 1 - 2*marginY
	if height == 8 {
		// The imported 5x8 golden uses the full cell height: Braille rows
		// land at y=0,2,4,6 rather than being vertically inset.
		marginY = 0
		spanY = height - 2
	}
	for dot := 0; dot < 8; dot++ {
		if bits&(1<<dot) == 0 {
			continue
		}
		px := x + marginX + dotX[dot]*spanX
		py := y + marginY + dotY[dot]*spanY/3
		if px >= img.Bounds().Min.X && px < img.Bounds().Max.X && py >= img.Bounds().Min.Y && py < img.Bounds().Max.Y {
			img.SetRGBA(px, py, col)
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

// DrawStringBounded draws a single-line string of runes onto img at (x, y) stopping before maxX.
func (f *MonospaceFont) DrawStringBounded(img *image.RGBA, s string, x, y, maxX int, col color.RGBA) int {
	curX := x
	for len(s) > 0 {
		if curX+f.CharWidth > maxX {
			break
		}
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		if r == '\t' {
			// Tab expands to 4 spaces
			if curX+f.CharWidth*4 > maxX {
				break
			}
			curX += f.CharWidth * 4
			continue
		}
		f.DrawRune(img, r, curX, y, col)
		curX += f.CharWidth
	}
	return curX - x
}

// Font5x8 provides the canonical 5x8 retro pixel font (6x8 cell) for maximum density and 1-bit crisp contrast.
var Font5x8 *MonospaceFont

// Font3x5 provides the ultra-dense 3x5 micro pixel font (4x6 cell) for extreme token compression.
var Font3x5 *MonospaceFont

// Font6x12 provides the 6x12 retro console font (7x12 cell).
var Font6x12 *MonospaceFont

// DefaultFont8x16 provides a crisp 8x16 monospace font.
var DefaultFont8x16 *MonospaceFont

// DefaultFont7x13 provides a dense 7x13 monospace font.
var DefaultFont7x13 *MonospaceFont

// DefaultFont is the default font used across harnez visual cards (Font5x8 retro pixel font).
var DefaultFont *MonospaceFont

func init() {
	Font5x8 = buildFont5x8()
	Font3x5 = buildFont3x5()
	Font6x12 = buildFont6x12()
	DefaultFont8x16 = buildFont8x16()
	DefaultFont7x13 = buildFont7x13()
	DefaultFont = Font5x8
}

// ParseFont resolves a font by name (e.g. "pixel", "retro", "5x8", "3x5", "micro", "6x12", "standard", "8x16", "7x13").
func ParseFont(name string) (*MonospaceFont, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "pixel", "retro", "5x8", "default":
		return Font5x8, nil
	case "3x5", "micro", "thumb":
		return Font3x5, nil
	case "6x12":
		return Font6x12, nil
	case "7x13":
		return DefaultFont7x13, nil
	case "8x16", "standard", "vga":
		return DefaultFont8x16, nil
	default:
		return nil, fmt.Errorf("unknown font %q: valid options are pixel, retro, 5x8, 3x5, micro, 6x12, standard, 8x16, 7x13", name)
	}
}

// ParseFontName is an alias for ParseFont.
func ParseFontName(name string) (*MonospaceFont, error) {
	return ParseFont(name)
}

// GetFont returns the best font matching the target font size (defaulting to Font5x8 retro pixel font).
func GetFont(fontSize ...int) *MonospaceFont {
	if len(fontSize) > 0 && fontSize[0] > 0 {
		switch {
		case fontSize[0] <= 6:
			return Font3x5
		case fontSize[0] <= 11:
			return Font5x8
		case fontSize[0] <= 13:
			return DefaultFont7x13
		default:
			return DefaultFont8x16
		}
	}
	return Font5x8
}

// ResolveFont resolves a font from name and fontSize, defaulting to Font5x8.
func ResolveFont(name string, fontSize int) *MonospaceFont {
	if name != "" {
		if f, err := ParseFont(name); err == nil {
			return f
		}
	}
	if fontSize > 0 && fontSize != 11 {
		return GetFont(fontSize)
	}
	return Font5x8
}
