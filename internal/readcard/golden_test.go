package readcard

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestGoldenFontAssets compares decoded pixels, not PNG bytes: the bytes
// change with the Go image/png encoder even when the render does not (1004).
func TestGoldenFontAssets(t *testing.T) {
	fonts := map[string]*MonospaceFont{
		"3x5": Font3x5, "5x8": Font5x8, "6x12": Font6x12,
		"7x13": DefaultFont7x13, "8x16": DefaultFont8x16,
	}
	for name, font := range fonts {
		path := filepath.Join("..", "..", "docs", "data", "golden-font-"+name+".png")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		decoded, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		want := image.NewRGBA(decoded.Bounds())
		draw.Draw(want, want.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
		got := GoldenFontImage(font)
		if got.Bounds() != want.Bounds() || !bytes.Equal(got.Pix, want.Pix) {
			t.Errorf("%s golden asset is stale; run go run ./scripts/generate-golden-fonts.go", name)
		}
	}
}
