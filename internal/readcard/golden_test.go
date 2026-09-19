package readcard

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestGoldenFontAssets(t *testing.T) {
	fonts := map[string]*MonospaceFont{
		"3x5": Font3x5, "5x8": Font5x8, "6x12": Font6x12,
		"7x13": DefaultFont7x13, "8x16": DefaultFont8x16,
	}
	for name, font := range fonts {
		path := filepath.Join("..", "..", "docs", "data", "golden-font-"+name+".png")
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var got bytes.Buffer
		if err := png.Encode(&got, GoldenFontImage(font)); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Bytes(), want) {
			t.Errorf("%s golden asset is stale; run go run ./scripts/generate-golden-fonts.go", name)
		}
	}
}
