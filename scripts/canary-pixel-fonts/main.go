// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package main implements a canary pixel font rasterizer and ViT token compression probe.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// FontDef defines dimensions and raw bit patterns for a pixel font.
type FontDef struct {
	Name    string
	CharW   int
	CharH   int
	CellW   int
	CellH   int
	Bitmaps map[rune][]uint8
}

// 3x5 Micro Font glyph bit patterns (5 rows per char, 3 bits wide).
var font3x5Bitmaps = map[rune][]uint8{
	' ':  {0, 0, 0, 0, 0},
	'!':  {2, 2, 2, 0, 2},
	'"':  {5, 5, 0, 0, 0},
	'#':  {5, 7, 5, 7, 5},
	'$':  {7, 6, 7, 3, 7},
	'%':  {5, 1, 2, 4, 5},
	'&':  {2, 5, 2, 5, 3},
	'\'': {2, 2, 0, 0, 0},
	'(':  {2, 4, 4, 4, 2},
	')':  {2, 1, 1, 1, 2},
	'*':  {0, 5, 2, 5, 0},
	'+':  {0, 2, 7, 2, 0},
	',':  {0, 0, 0, 2, 4},
	'-':  {0, 0, 7, 0, 0},
	'.':  {0, 0, 0, 0, 2},
	'/':  {1, 1, 2, 4, 4},
	'0':  {7, 5, 5, 5, 7},
	'1':  {2, 6, 2, 2, 7},
	'2':  {7, 1, 7, 4, 7},
	'3':  {7, 1, 7, 1, 7},
	'4':  {5, 5, 7, 1, 1},
	'5':  {7, 4, 7, 1, 7},
	'6':  {7, 4, 7, 5, 7},
	'7':  {7, 1, 2, 2, 2},
	'8':  {7, 5, 7, 5, 7},
	'9':  {7, 5, 7, 1, 7},
	':':  {0, 2, 0, 2, 0},
	';':  {0, 2, 0, 2, 4},
	'<':  {1, 2, 4, 2, 1},
	'=':  {0, 7, 0, 7, 0},
	'>':  {4, 2, 1, 2, 4},
	'?':  {7, 1, 3, 0, 2},
	'@':  {7, 5, 7, 4, 7},
	'A':  {2, 5, 7, 5, 5},
	'B':  {6, 5, 6, 5, 6},
	'C':  {7, 4, 4, 4, 7},
	'D':  {6, 5, 5, 5, 6},
	'E':  {7, 4, 6, 4, 7},
	'F':  {7, 4, 6, 4, 4},
	'G':  {7, 4, 5, 5, 7},
	'H':  {5, 5, 7, 5, 5},
	'I':  {7, 2, 2, 2, 7},
	'J':  {1, 1, 1, 5, 2},
	'K':  {5, 5, 6, 5, 5},
	'L':  {4, 4, 4, 4, 7},
	'M':  {5, 7, 5, 5, 5},
	'N':  {5, 7, 7, 5, 5},
	'O':  {7, 5, 5, 5, 7},
	'P':  {7, 5, 7, 4, 4},
	'Q':  {7, 5, 5, 7, 1},
	'R':  {7, 5, 6, 5, 5},
	'S':  {7, 4, 7, 1, 7},
	'T':  {7, 2, 2, 2, 2},
	'U':  {5, 5, 5, 5, 7},
	'V':  {5, 5, 5, 5, 2},
	'W':  {5, 5, 5, 7, 5},
	'X':  {5, 5, 2, 5, 5},
	'Y':  {5, 5, 2, 2, 2},
	'Z':  {7, 1, 2, 4, 7},
	'[':  {3, 2, 2, 2, 3},
	'\\': {4, 4, 2, 1, 1},
	']':  {6, 2, 2, 2, 6},
	'^':  {2, 5, 0, 0, 0},
	'_':  {0, 0, 0, 0, 7},
	'`':  {4, 2, 0, 0, 0},
	'a':  {0, 6, 7, 5, 7},
	'b':  {4, 6, 5, 5, 6},
	'c':  {0, 7, 4, 4, 7},
	'd':  {1, 3, 5, 5, 3},
	'e':  {0, 7, 7, 4, 7},
	'f':  {3, 4, 6, 4, 4},
	'g':  {0, 7, 5, 7, 1},
	'h':  {4, 6, 5, 5, 5},
	'i':  {2, 0, 2, 2, 2},
	'j':  {1, 0, 1, 5, 2},
	'k':  {4, 5, 6, 5, 5},
	'l':  {6, 2, 2, 2, 3},
	'm':  {0, 5, 7, 5, 5},
	'n':  {0, 6, 5, 5, 5},
	'o':  {0, 2, 5, 5, 2},
	'p':  {0, 6, 5, 6, 4},
	'q':  {0, 3, 5, 3, 1},
	'r':  {0, 5, 6, 4, 4},
	's':  {0, 3, 6, 1, 6},
	't':  {2, 7, 2, 2, 1},
	'u':  {0, 5, 5, 5, 3},
	'v':  {0, 5, 5, 5, 2},
	'w':  {0, 5, 5, 7, 5},
	'x':  {0, 5, 2, 5, 5},
	'y':  {0, 5, 5, 3, 6},
	'z':  {0, 7, 3, 6, 7},
	'{':  {3, 2, 6, 2, 3},
	'|':  {2, 2, 2, 2, 2},
	'}':  {6, 2, 3, 2, 6},
	'~':  {0, 5, 2, 0, 0},
}

// 5x7 Standard Console Font (7 rows per char, 5 bits wide).
var font5x7Bitmaps = map[rune][]uint8{
	' ':  {0, 0, 0, 0, 0, 0, 0},
	'!':  {4, 4, 4, 4, 0, 0, 4},
	'"':  {10, 10, 10, 0, 0, 0, 0},
	'#':  {10, 10, 31, 10, 31, 10, 10},
	'$':  {4, 15, 20, 14, 5, 30, 4},
	'%':  {25, 26, 4, 8, 16, 19, 25},
	'&':  {12, 18, 20, 8, 21, 18, 13},
	'\'': {12, 4, 8, 0, 0, 0, 0},
	'(':  {2, 4, 8, 8, 8, 4, 2},
	')':  {8, 4, 2, 2, 2, 4, 8},
	'*':  {0, 4, 21, 14, 21, 4, 0},
	'+':  {0, 4, 4, 31, 4, 4, 0},
	',':  {0, 0, 0, 0, 12, 4, 8},
	'-':  {0, 0, 0, 31, 0, 0, 0},
	'.':  {0, 0, 0, 0, 0, 12, 12},
	'/':  {1, 2, 4, 8, 16, 0, 0},
	'0':  {14, 17, 19, 21, 25, 17, 14},
	'1':  {4, 12, 4, 4, 4, 4, 14},
	'2':  {14, 17, 1, 2, 4, 8, 31},
	'3':  {31, 2, 4, 2, 1, 17, 14},
	'4':  {2, 6, 10, 18, 31, 2, 2},
	'5':  {31, 16, 30, 1, 1, 17, 14},
	'6':  {6, 8, 16, 30, 17, 17, 14},
	'7':  {31, 1, 2, 4, 8, 8, 8},
	'8':  {14, 17, 17, 14, 17, 17, 14},
	'9':  {14, 17, 17, 15, 1, 2, 12},
	':':  {0, 12, 12, 0, 12, 12, 0},
	';':  {0, 12, 12, 0, 12, 4, 8},
	'<':  {2, 4, 8, 16, 8, 4, 2},
	'=':  {0, 31, 0, 31, 0, 0, 0},
	'>':  {8, 4, 2, 1, 2, 4, 8},
	'?':  {14, 17, 1, 2, 4, 0, 4},
	'@':  {14, 17, 1, 13, 21, 21, 14},
	'A':  {14, 17, 17, 31, 17, 17, 17},
	'B':  {30, 17, 17, 30, 17, 17, 30},
	'C':  {14, 17, 16, 16, 16, 17, 14},
	'D':  {28, 18, 17, 17, 17, 18, 28},
	'E':  {31, 16, 16, 30, 16, 16, 31},
	'F':  {31, 16, 16, 30, 16, 16, 16},
	'G':  {14, 17, 16, 23, 17, 17, 14},
	'H':  {17, 17, 17, 31, 17, 17, 17},
	'I':  {14, 4, 4, 4, 4, 4, 14},
	'J':  {7, 2, 2, 2, 2, 18, 12},
	'K':  {17, 18, 20, 24, 20, 18, 17},
	'L':  {16, 16, 16, 16, 16, 16, 31},
	'M':  {17, 27, 21, 21, 17, 17, 17},
	'N':  {17, 17, 25, 21, 19, 17, 17},
	'O':  {14, 17, 17, 17, 17, 17, 14},
	'P':  {30, 17, 17, 30, 16, 16, 16},
	'Q':  {14, 17, 17, 17, 21, 18, 13},
	'R':  {30, 17, 17, 30, 20, 18, 17},
	'S':  {14, 17, 16, 14, 1, 17, 14},
	'T':  {31, 4, 4, 4, 4, 4, 4},
	'U':  {17, 17, 17, 17, 17, 17, 14},
	'V':  {17, 17, 17, 17, 17, 10, 4},
	'W':  {17, 17, 17, 21, 21, 27, 17},
	'X':  {17, 17, 10, 4, 10, 17, 17},
	'Y':  {17, 17, 10, 4, 4, 4, 4},
	'Z':  {31, 1, 2, 4, 8, 16, 31},
	'[':  {14, 8, 8, 8, 8, 8, 14},
	'\\': {16, 8, 4, 2, 1, 0, 0},
	']':  {14, 2, 2, 2, 2, 2, 14},
	'^':  {4, 10, 17, 0, 0, 0, 0},
	'_':  {0, 0, 0, 0, 0, 0, 31},
	'`':  {8, 4, 2, 0, 0, 0, 0},
	'a':  {0, 0, 14, 1, 15, 17, 15},
	'b':  {16, 16, 22, 25, 17, 17, 30},
	'c':  {0, 0, 14, 17, 16, 17, 14},
	'd':  {1, 1, 13, 19, 17, 17, 15},
	'e':  {0, 0, 14, 17, 31, 16, 14},
	'f':  {6, 9, 8, 28, 8, 8, 8},
	'g':  {0, 0, 15, 17, 17, 15, 1, 14},
	'h':  {16, 16, 22, 25, 17, 17, 17},
	'i':  {4, 0, 12, 4, 4, 4, 14},
	'j':  {2, 0, 6, 2, 2, 2, 18, 12},
	'k':  {16, 16, 18, 20, 24, 20, 18},
	'l':  {12, 4, 4, 4, 4, 4, 14},
	'm':  {0, 0, 26, 21, 21, 17, 17},
	'n':  {0, 0, 22, 25, 17, 17, 17},
	'o':  {0, 0, 14, 17, 17, 17, 14},
	'p':  {0, 0, 30, 17, 17, 30, 16, 16},
	'q':  {0, 0, 15, 17, 17, 15, 1, 1},
	'r':  {0, 0, 22, 25, 16, 16, 16},
	's':  {0, 0, 15, 16, 14, 1, 30},
	't':  {8, 8, 28, 8, 8, 9, 6},
	'u':  {0, 0, 17, 17, 17, 19, 13},
	'v':  {0, 0, 17, 17, 17, 10, 4},
	'w':  {0, 0, 17, 17, 21, 21, 10},
	'x':  {0, 0, 17, 10, 4, 10, 17},
	'y':  {0, 0, 17, 17, 15, 1, 14},
	'z':  {0, 0, 31, 2, 4, 8, 31},
	'{':  {6, 8, 8, 16, 8, 8, 6},
	'|':  {4, 4, 4, 4, 4, 4, 4},
	'}':  {12, 2, 2, 1, 2, 2, 12},
	'~':  {0, 0, 13, 22, 0, 0, 0},
}

// GetFont returns the font definition by name.
func GetFont(name string) (*FontDef, error) {
	switch name {
	case "3x5":
		return &FontDef{
			Name:    "3x5",
			CharW:   3,
			CharH:   5,
			CellW:   4,
			CellH:   6,
			Bitmaps: font3x5Bitmaps,
		}, nil
	case "5x8":
		return &FontDef{
			Name:    "5x8",
			CharW:   5,
			CharH:   7,
			CellW:   6,
			CellH:   8,
			Bitmaps: font5x7Bitmaps,
		}, nil
	case "6x12":
		return &FontDef{
			Name:    "6x12",
			CharW:   5,
			CharH:   7,
			CellW:   7,
			CellH:   12,
			Bitmaps: font5x7Bitmaps,
		}, nil
	default:
		return nil, fmt.Errorf("unknown font: %s", name)
	}
}

// RenderText renders lines of text into an image with integer nearest neighbor scaling.
func RenderText(font *FontDef, text string, width, height, scale int) (*image.RGBA, int, int) {
	if scale < 1 {
		scale = 1
	}
	baseW := width / scale
	baseH := height / scale

	img := image.NewRGBA(image.Rect(0, 0, baseW, baseH))
	bg := color.RGBA{R: 13, G: 17, B: 23, A: 255}
	fg := color.RGBA{R: 230, G: 237, B: 243, A: 255}
	accent := color.RGBA{R: 121, G: 192, B: 255, A: 255}
	comment := color.RGBA{R: 139, G: 148, B: 158, A: 255}

	// Fill background
	for y := 0; y < baseH; y++ {
		for x := 0; x < baseW; x++ {
			img.Set(x, y, bg)
		}
	}

	xMargin := 2
	yMargin := 2
	maxCols := (baseW - 2*xMargin) / font.CellW
	maxRows := (baseH - 2*yMargin) / font.CellH

	lines := strings.Split(text, "\n")
	curY := yMargin

	for rIdx, line := range lines {
		if rIdx >= maxRows {
			break
		}
		curX := xMargin
		colCount := 0

		lineColor := fg
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			lineColor = comment
		} else if strings.HasPrefix(line, "===") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "PROBE") {
			lineColor = accent
		}

		for _, ch := range line {
			if colCount >= maxCols {
				break
			}
			rows, ok := font.Bitmaps[ch]
			if !ok {
				rows = font.Bitmaps['?']
			}
			for rowIdx, rowBits := range rows {
				if rowIdx >= font.CharH {
					break
				}
				for colIdx := 0; colIdx < font.CharW; colIdx++ {
					shift := font.CharW - 1 - colIdx
					if (rowBits & (1 << shift)) != 0 {
						px := curX + colIdx
						py := curY + rowIdx
						if px >= 0 && px < baseW && py >= 0 && py < baseH {
							img.Set(px, py, lineColor)
						}
					}
				}
			}
			curX += font.CellW
			colCount++
		}
		curY += font.CellH
	}

	if scale == 1 {
		return img, maxCols, maxRows
	}

	// Nearest-neighbor upscale
	scaledImg := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcY := y / scale
		for x := 0; x < width; x++ {
			srcX := x / scale
			scaledImg.Set(x, y, img.At(srcX, srcY))
		}
	}

	return scaledImg, maxCols, maxRows
}

func main() {
	fontFlag := flag.String("font", "5x8", "Font name: 3x5, 5x8, 6x12")
	scaleFlag := flag.Int("scale", 1, "Integer scale factor (1, 2, 3)")
	widthFlag := flag.Int("width", 256, "Canvas width")
	heightFlag := flag.Int("height", 256, "Canvas height")
	outFlag := flag.String("out", "/home/uwe/projects/harnez/scratch/pixel-fonts/go_probe.png", "Output PNG path")
	benchFlag := flag.Bool("benchmark", false, "Run full benchmark table")
	flag.Parse()

	if *benchFlag {
		runBenchmark()
		return
	}

	fontDef, err := GetFont(*fontFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	sampleText := `=== CANARY PIXEL PROBE (GO) ===
if test "$x" != "$y" && test -f "${PATH:?}"; then local val=$(cmd); fi
Glyphs: { [ ( < > == != := && || ! ~ * ; : / \ # @ $ % ^ ) ] }
Rules : Invariant 3: Zero Zombie Guarantee (pid=9482, exit=0)
Token : 15x-30x Multimodal ViT Compression`

	img, cols, rows := RenderText(fontDef, sampleText, *widthFlag, *heightFlag, *scaleFlag)

	if err := os.MkdirAll(filepath.Dir(*outFlag), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Mkdir error: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Create(*outFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Create error: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		fmt.Fprintf(os.Stderr, "PNG encode error: %v\n", err)
		os.Exit(1)
	}

	totalChars := cols * rows
	fmt.Printf("[OK] Rendered %s (scale %dx) on %dx%d canvas -> %s (Grid: %dx%d, Chars: %d)\n",
		*fontFlag, *scaleFlag, *widthFlag, *heightFlag, *outFlag, cols, rows, totalChars)
}

func runBenchmark() {
	fmt.Println("=== GO PIXEL FONT BENCHMARK ===")
	resolutions := [][2]int{{256, 256}, {384, 384}, {512, 512}}
	fonts := []string{"3x5", "5x8", "6x12"}

	fmt.Printf("%-10s | %-6s | %-6s | %-10s | %-8s | %-12s | %-12s | %-12s\n",
		"Canvas", "Font", "Scale", "Grid", "Chars", "OpenAI Ratio", "Gemini Ratio", "Claude Ratio")
	fmt.Println(strings.Repeat("-", 90))

	for _, res := range resolutions {
		w, h := res[0], res[1]
		for _, fontName := range fonts {
			for _, scale := range []int{1, 2} {
				fontDef, _ := GetFont(fontName)
				baseW := w / scale
				baseH := h / scale
				cols := (baseW - 4) / fontDef.CellW
				rows := (baseH - 4) / fontDef.CellH
				chars := cols * rows
				textTokens := float64(chars) / 4.0

				openAITiles := math.Ceil(float64(w)/512.0) * math.Ceil(float64(h)/512.0)
				openAITokens := 85.0 + 170.0*openAITiles
				geminiTokens := 258.0
				claudeTokens := float64(w*h) / 750.0

				fmt.Printf("%-10s | %-6s | %-6dx | %-10s | %-8d | %-12.2fx | %-12.2fx | %-12.2fx\n",
					fmt.Sprintf("%dx%d", w, h),
					fontName,
					scale,
					fmt.Sprintf("%dx%d", cols, rows),
					chars,
					textTokens/openAITokens,
					textTokens/geminiTokens,
					textTokens/claudeTokens,
				)
			}
		}
	}
}
