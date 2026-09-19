package readcard

import (
	"image"
	"image/color"
)

// GoldenFontImage renders the stable character matrix used by issue 431.
func GoldenFontImage(font *MonospaceFont) *image.RGBA {
	const columns = 16
	charset := supportedGlyphCharset()
	rows := (len([]rune(charset)) + columns - 1) / columns
	img := image.NewRGBA(image.Rect(0, 0, columns*font.CharWidth, rows*font.CharHeight))
	dot := color.RGBA{R: 190, G: 70, B: 220, A: 255}
	for i, r := range []rune(charset) {
		x := (i % columns) * font.CharWidth
		y := (i / columns) * font.CharHeight
		img.SetRGBA(x, y, dot)
		font.DrawRune(img, r, x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	}
	return img
}
