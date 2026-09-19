//go:build ignore

package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/readcard"
)

func main() {
	outDir := filepath.Join("docs", "data")
	fonts := map[string]*readcard.MonospaceFont{
		"3x5": readcard.Font3x5, "5x8": readcard.Font5x8,
		"6x12": readcard.Font6x12, "7x13": readcard.DefaultFont7x13,
		"8x16": readcard.DefaultFont8x16,
	}
	for name, font := range fonts {
		path := filepath.Join(outDir, "golden-font-"+name+".png")
		want := readcard.GoldenFontImage(font)
		file, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		if err := png.Encode(file, want); err != nil {
			panic(err)
		}
		if err := file.Close(); err != nil {
			panic(err)
		}
		if err := verifyGoldenPNG(path, want); err != nil {
			panic(fmt.Sprintf("%s: %v", path, err))
		}
		if name == "5x8" {
			if err := verifyImported5x8(path); err != nil {
				panic(fmt.Sprintf("imported 5x8 mismatch: %v", err))
			}
		}
		fmt.Println(path, "OK")
	}
	if err := writeGoldenText(outDir, fonts); err != nil {
		panic(err)
	}
}

func verifyImported5x8(path string) error {
	font := readcard.Font5x8
	generatedFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer generatedFile.Close()
	generated, err := png.Decode(generatedFile)
	if err != nil {
		return err
	}
	importedFile, err := os.Open("docs/data/golden-font-5x8-import.png")
	if err != nil {
		return err
	}
	defer importedFile.Close()
	imported, err := png.Decode(importedFile)
	if err != nil {
		return err
	}
	if generated.Bounds().Dx() < imported.Bounds().Dx() || generated.Bounds().Dy() < imported.Bounds().Dy() {
		return fmt.Errorf("generated dimensions %v do not cover imported dimensions %v", generated.Bounds(), imported.Bounds())
	}
	for cellY := 0; cellY < imported.Bounds().Dy(); cellY += font.CharHeight {
		for cellX := 0; cellX < imported.Bounds().Dx(); cellX += font.CharWidth {
			occupied := false
			for y := cellY; y < cellY+font.CharHeight && y < imported.Bounds().Dy(); y++ {
				for x := cellX; x < cellX+font.CharWidth && x < imported.Bounds().Dx(); x++ {
					if x == cellX && y == cellY {
						continue
					}
					_, _, _, alpha := imported.At(x, y).RGBA()
					occupied = occupied || alpha != 0
				}
			}
			if !occupied {
				continue
			}
			for y := cellY; y < cellY+font.CharHeight && y < imported.Bounds().Dy(); y++ {
				for x := cellX; x < cellX+font.CharWidth && x < imported.Bounds().Dx(); x++ {
					if x == cellX && y == cellY {
						continue
					}
					gr, gg, gb, ga := generated.At(x, y).RGBA()
					ir, ig, ib, ia := imported.At(x, y).RGBA()
					if gr != ir || gg != ig || gb != ib || ga != ia {
						return fmt.Errorf("pixel mismatch at (%d,%d)", x, y)
					}
				}
			}
		}
	}
	return nil
}

func verifyGoldenPNG(path string, want image.Image) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("reopen: %w", err)
	}
	defer file.Close()
	got, err := png.Decode(file)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if !got.Bounds().Eq(want.Bounds()) {
		return fmt.Errorf("dimensions %v, want %v", got.Bounds(), want.Bounds())
	}
	for y := want.Bounds().Min.Y; y < want.Bounds().Max.Y; y++ {
		for x := want.Bounds().Min.X; x < want.Bounds().Max.X; x++ {
			gr, gg, gb, ga := got.At(x, y).RGBA()
			wr, wg, wb, wa := want.At(x, y).RGBA()
			if gr != wr || gg != wg || gb != wb || ga != wa {
				return fmt.Errorf("pixel mismatch at (%d,%d)", x, y)
			}
		}
	}
	return nil
}

func writeGoldenText(outDir string, fonts map[string]*readcard.MonospaceFont) error {
	var b strings.Builder
	_ = fonts
	runes := []rune(readcard.SupportedGlyphCharset())
	for start := 0; start < len(runes); start += 16 {
		end := start + 16
		if end > len(runes) {
			end = len(runes)
		}
		b.WriteString(string(runes[start:end]))
		b.WriteByte('\n')
	}
	return os.WriteFile(filepath.Join(outDir, "golden.txt"), []byte(b.String()), 0o644)
}
