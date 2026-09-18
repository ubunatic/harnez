package readcard

import (
	"image"
	"image/color"
	"testing"
)

func TestDrawRuneDegree(t *testing.T) {
	for _, f := range []*MonospaceFont{Font3x5, Font5x8, Font6x12, DefaultFont7x13, DefaultFont8x16} {
		img := image.NewRGBA(image.Rect(0, 0, f.CharWidth, f.CharHeight))
		f.DrawRune(img, '°', 0, 0, color.RGBA{R: 255, A: 255})
		found := false
		for _, p := range img.Pix {
			if p != 0 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s did not render degree symbol", f.Name)
		}
	}
}

func TestDrawRuneTypicalUnicodeSymbols(t *testing.T) {
	for _, r := range []rune{'┈', '↑', '↔', '±', '×', '≤', '–', '⚠'} {
		img := image.NewRGBA(image.Rect(0, 0, DefaultFont8x16.CharWidth, DefaultFont8x16.CharHeight))
		DefaultFont8x16.DrawRune(img, r, 0, 0, color.RGBA{R: 255, A: 255})
		found := false
		for _, p := range img.Pix {
			if p != 0 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("symbol %q did not render", r)
		}
	}
}
