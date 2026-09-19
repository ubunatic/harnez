package readcard

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLineRange(t *testing.T) {
	tests := []struct {
		input     string
		total     int
		wantStart int
		wantEnd   int
		wantErr   bool
	}{
		{"", 100, 1, 100, false},
		{"10:50", 100, 10, 50, false},
		{"10-50", 100, 10, 50, false},
		{"10..50", 100, 10, 50, false},
		{":30", 100, 1, 30, false},
		{"70:", 100, 70, 100, false},
		{"42", 100, 42, 42, false},
		{"invalid", 100, 0, 0, true},
	}

	for _, tt := range tests {
		s, e, err := ParseLineRange(tt.input, tt.total)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseLineRange(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr {
			if s != tt.wantStart || e != tt.wantEnd {
				t.Errorf("ParseLineRange(%q) = (%d, %d), want (%d, %d)", tt.input, s, e, tt.wantStart, tt.wantEnd)
			}
		}
	}
}

func TestReadSourceTextAndFormatting(t *testing.T) {
	sample := `package main

import "fmt"

func main() {
	fmt.Println("Hello, Harnez!")
}
`
	res, err := ReadSource(strings.NewReader(sample), "sample.go", TextOptions{
		LineRange: "1:4",
	})
	if err != nil {
		t.Fatalf("ReadSource failed: %v", err)
	}

	if len(res.Lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(res.Lines))
	}
	if res.Lines[0] != "package main" {
		t.Errorf("line 0 = %q, want 'package main'", res.Lines[0])
	}

	formatted := FormatText(res, true)
	if !strings.Contains(formatted, "1 │ package main") {
		t.Errorf("expected formatted line numbers, got:\n%s", formatted)
	}
}

func TestHighlightLexer(t *testing.T) {
	line := `func helloWorld() string { return "harnez" } // comment`
	tokens := HighlightLine(line, ".go", nil)
	if len(tokens) == 0 {
		t.Fatalf("expected tokens for Go line")
	}

	foundKeyword := false
	foundString := false
	foundComment := false

	for _, tok := range tokens {
		if tok.Type == TokenKeyword && (tok.Text == "func" || tok.Text == "return") {
			foundKeyword = true
		}
		if tok.Type == TokenString && tok.Text == `"harnez"` {
			foundString = true
		}
		if tok.Type == TokenComment && strings.Contains(tok.Text, "// comment") {
			foundComment = true
		}
	}

	if !foundKeyword {
		t.Errorf("expected keyword token")
	}
	if !foundString {
		t.Errorf("expected string token")
	}
	if !foundComment {
		t.Errorf("expected comment token")
	}
}

func TestFontRasterizer(t *testing.T) {
	font := GetFont(11)
	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	w := font.DrawString(img, "harnez", 5, 5, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	if w <= 0 {
		t.Errorf("expected drawn width > 0, got %d", w)
	}
}

// TestPercentGlyphRasterAlignment pins the hand-tuned 5x8 percent exactly. The
// other sizes are upstream fonts, so only the layout is checked: ink in the
// top-left and bottom-right quadrants (the two counters) and in both other
// quadrants (the slash).
func TestPercentGlyphRasterAlignment(t *testing.T) {
	render := func(font *MonospaceFont) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, font.CharWidth, font.CharHeight))
		font.DrawRune(img, '%', 0, 0, color.RGBA{R: 255, A: 255})
		return img
	}
	t.Run("5x8", func(t *testing.T) {
		img := render(Font5x8)
		want := []int{2, 3, 1, 1, 1, 3, 2}
		rows := make([]int, 0, len(want))
		for y := 0; y < Font5x8.CharHeight; y++ {
			count := 0
			for x := 0; x < Font5x8.CharWidth; x++ {
				if img.RGBAAt(x, y).A != 0 {
					count++
				}
			}
			if count > 0 {
				rows = append(rows, count)
			}
		}
		if len(rows) != len(want) {
			t.Fatalf("non-empty row counts = %v, want %v", rows, want)
		}
		for i := range rows {
			if rows[i] != want[i] {
				t.Fatalf("row %d pixel count = %d, want %d; rows = %v", i, rows[i], want[i], rows)
			}
		}
	})
	for _, font := range []*MonospaceFont{Font3x5, Font6x12, DefaultFont7x13, DefaultFont8x16} {
		t.Run(font.Name, func(t *testing.T) {
			img := render(font)
			quadrant := func(qx, qy int) int {
				count := 0
				for y := qy * font.CharHeight / 2; y < (qy+1)*font.CharHeight/2; y++ {
					for x := qx * font.CharWidth / 2; x < (qx+1)*font.CharWidth/2; x++ {
						if img.RGBAAt(x, y).A != 0 {
							count++
						}
					}
				}
				return count
			}
			for qy := 0; qy < 2; qy++ {
				for qx := 0; qx < 2; qx++ {
					if quadrant(qx, qy) == 0 {
						t.Errorf("percent has no ink in quadrant (%d,%d)", qx, qy)
					}
				}
			}
		})
	}
}

func TestRenderFileToCards(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "rendered_card.png")

	lines := make([]string, 80)
	for i := 0; i < 80; i++ {
		lines[i] = "func processStep(n int) error { return nil } // step execution"
	}

	res, err := RenderFileToCards(lines, "step.go", RenderOptions{
		OutputPath:      outPath,
		Columns:         2,
		FontSize:        11,
		ShowLineNumbers: true,
	})
	if err != nil {
		t.Fatalf("RenderFileToCards failed: %v", err)
	}

	if len(res.Files) == 0 {
		t.Fatalf("expected generated file paths")
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("output file %s not found: %v", outPath, err)
	}
	if res.Width > 1568 || res.Height > 1568 {
		t.Errorf("dimensions (%dx%d) exceed 1568 bound", res.Width, res.Height)
	}
	if res.TokenStats.ClaudeTokens <= 0 || res.TokenStats.OpenAITokens <= 0 {
		t.Errorf("expected positive ViT token stats, got %+v", res.TokenStats)
	}
}

func TestTokenStatsFormulas(t *testing.T) {
	text := "Hello world from multimodal context"
	txtStats := ComputeTextTokens(text)
	if txtStats.TextTokens <= 0 {
		t.Errorf("expected text tokens > 0, got %d", txtStats.TextTokens)
	}

	imgStats := ComputeImageTokens(txtStats.TextTokens, txtStats.TextBytes, 1200, 800, 1)
	if imgStats.ClaudeTokens <= 0 || imgStats.OpenAITokens <= 0 || imgStats.GeminiTokens <= 0 {
		t.Errorf("invalid image token stats: %+v", imgStats)
	}
}

func TestEstimateSavingsLargePayload(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&b, "line %03d: useful source content for the native read\n", i)
	}
	estimate := EstimateSavings(b.String(), ProviderClaude)
	if estimate.SavingsTokens <= 0 || estimate.SavingsBytes <= 0 {
		t.Fatalf("expected positive savings for large payload, got %+v", estimate)
	}
}

func TestParseFont(t *testing.T) {
	tests := []struct {
		name      string
		wantFont  *MonospaceFont
		wantWidth int
		wantErr   bool
	}{
		{"", Font5x8, 6, false},
		{"pixel", Font5x8, 6, false},
		{"retro", Font5x8, 6, false},
		{"5x8", Font5x8, 6, false},
		{"3x5", Font3x5, 4, false},
		{"micro", Font3x5, 4, false},
		{"6x12", Font6x12, 7, false},
		{"standard", DefaultFont8x16, 8, false},
		{"8x16", DefaultFont8x16, 8, false},
		{"7x13", DefaultFont7x13, 7, false},
		{"unknown-font", nil, 0, true},
	}

	for _, tt := range tests {
		f, err := ParseFont(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseFont(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			continue
		}
		if !tt.wantErr {
			if f != tt.wantFont {
				t.Errorf("ParseFont(%q) = %v, want %v", tt.name, f.Name, tt.wantFont.Name)
			}
			if f.CharWidth != tt.wantWidth {
				t.Errorf("ParseFont(%q) width = %d, want %d", tt.name, f.CharWidth, tt.wantWidth)
			}
		}
	}
}

func TestGetFontDefaults(t *testing.T) {
	if GetFont() != Font5x8 {
		t.Errorf("GetFont() should default to Font5x8 retro pixel font, got %v", GetFont().Name)
	}
	if GetFont(11) != Font5x8 {
		t.Errorf("GetFont(11) should resolve to Font5x8 retro pixel font, got %v", GetFont(11).Name)
	}
	if GetFont(5) != Font3x5 {
		t.Errorf("GetFont(5) should resolve to Font3x5 micro font, got %v", GetFont(5).Name)
	}
	if GetFont(13) != DefaultFont7x13 {
		t.Errorf("GetFont(13) should resolve to DefaultFont7x13, got %v", GetFont(13).Name)
	}
	if GetFont(16) != DefaultFont8x16 {
		t.Errorf("GetFont(16) should resolve to DefaultFont8x16, got %v", GetFont(16).Name)
	}
}

func TestDrawRuneBrailleRasterizesDots(t *testing.T) {
	fonts := []*MonospaceFont{Font3x5, Font5x8, Font6x12, DefaultFont7x13, DefaultFont8x16}
	for _, font := range fonts {
		img := image.NewRGBA(image.Rect(0, 0, font.CharWidth, font.CharHeight))
		font.DrawRune(img, '⣿', 0, 0, color.RGBA{R: 255, A: 255})
		pixels := 0
		for y := 0; y < font.CharHeight; y++ {
			for x := 0; x < font.CharWidth; x++ {
				if img.RGBAAt(x, y).A != 0 {
					pixels++
				}
			}
		}
		if pixels != 8 {
			t.Errorf("%s rendered ⣿ with %d pixels, want 8", font.Name, pixels)
		}
		for _, x := range []int{0, font.CharWidth - 1} {
			for y := 0; y < font.CharHeight; y++ {
				if img.RGBAAt(x, y).A != 0 {
					t.Errorf("%s rendered a Braille dot on horizontal cell edge at (%d, %d)", font.Name, x, y)
				}
			}
		}
		for _, y := range []int{0, font.CharHeight - 1} {
			if font == Font5x8 && y == 0 {
				continue
			}
			for x := 0; x < font.CharWidth; x++ {
				if img.RGBAAt(x, y).A != 0 {
					t.Errorf("%s rendered a Braille dot on vertical cell edge at (%d, %d)", font.Name, x, y)
				}
			}
		}
	}
}

func TestRenderBundleCardAndCheckCard(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "dev_3in1.png")

	sections := []CardSection{
		{
			Title:    "Bash Rules",
			Filename: "Bash.md",
			Lines: []string{
				"# Bash Rules",
				"- No ';', break before then/else",
				"- Use if test, no [[ ]]",
			},
		},
		{
			Title:    "Make Rules",
			Filename: "Make.md",
			Lines: []string{
				"# Make Rules",
				"- Use phony sentinel",
				"- Self-documenting help",
			},
		},
		{
			Title:    "Git Rules",
			Filename: "Git.md",
			Lines: []string{
				"# Git Rules",
				"- Conventional commits",
				"- Work on default branch",
			},
		},
	}

	res, err := RenderBundleCard(sections, BundleOptions{
		Title:        "Harnez Core Developer Cheatsheet (3-in-1)",
		Columns:      3,
		FontName:     "pixel",
		OutputPath:   outPath,
		MaxDimension: 1568,
	})
	if err != nil {
		t.Fatalf("RenderBundleCard failed: %v", err)
	}

	if len(res.Files) == 0 || res.Files[0] != outPath {
		t.Errorf("expected output path %s, got %+v", outPath, res.Files)
	}
	if res.Width > 1568 || res.Height > 1568 {
		t.Errorf("bundle dimensions (%dx%d) exceed 1568px bound", res.Width, res.Height)
	}

	checkRes, err := CheckCard(outPath, 1568)
	if err != nil {
		t.Fatalf("CheckCard failed: %v", err)
	}
	if !checkRes.Passed || !checkRes.ValidBounds || !checkRes.ValidFont {
		t.Errorf("expected CheckCard to pass, got %+v", checkRes)
	}
}

func TestRenderFileToCards_FontVariants(t *testing.T) {
	tmpDir := t.TempDir()
	lines := []string{
		"package main",
		"func main() {",
		"    println(\"pixel font test\")",
		"}",
	}

	for _, fontName := range []string{"pixel", "3x5", "6x12", "8x16"} {
		outPath := filepath.Join(tmpDir, "card_"+fontName+".png")
		res, err := RenderFileToCards(lines, "main.go", RenderOptions{
			FontName:   fontName,
			OutputPath: outPath,
		})
		if err != nil {
			t.Fatalf("RenderFileToCards with font %s failed: %v", fontName, err)
		}
		if len(res.Files) == 0 {
			t.Fatalf("expected output file for font %s", fontName)
		}
		if _, err := os.Stat(outPath); err != nil {
			t.Errorf("file %s not created: %v", outPath, err)
		}
	}
}

func TestRenderIssuesMatrixCard(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "matrix_issues.png")

	items := []IssueCardItem{
		{Number: "100", RawStatus: "Open", PlainTitle: "VRAM Load Panel", Path: "issues/100-vram.md"},
		{Number: "101", RawStatus: "Closed", PlainTitle: "GTT Cleanup", Path: "issues/101-gtt.md"},
		{Number: "102", RawStatus: "Blocked", PlainTitle: "Upstream Dependency", Path: "issues/102-dep.md"},
	}

	res, err := RenderIssuesMatrixCard(items, IssueMatrixOptions{
		Title:      "Found 3 Issues",
		OutputPath: outPath,
		FontName:   "pixel",
	})
	if err != nil {
		t.Fatalf("RenderIssuesMatrixCard failed: %v", err)
	}
	if len(res.Files) == 0 || res.Files[0] != outPath {
		t.Errorf("expected outpath %s, got %+v", outPath, res.Files)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("expected matrix card on disk: %v", err)
	}

	checkRes, err := CheckCard(outPath, 1568)
	if err != nil {
		t.Fatalf("CheckCard failed: %v", err)
	}
	if !checkRes.Passed {
		t.Errorf("expected CheckCard to pass on issues matrix card: %v", checkRes.Error)
	}
}

func TestRenderFileToCards_MultiColumnDistribution(t *testing.T) {
	testCases := []struct {
		name      string
		lineCount int
		cols      int
	}{
		{"85 lines 2 cols", 85, 2},
		{"133 lines 2 cols", 133, 2},
		{"250 lines 3 cols", 250, 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			outPath := filepath.Join(tmpDir, "card.png")

			lines := make([]string, tc.lineCount)
			for i := 0; i < tc.lineCount; i++ {
				lines[i] = fmt.Sprintf("const Item%03d = %d // sample value", i+1, i+1)
			}

			res, err := RenderFileToCards(lines, "items.go", RenderOptions{
				OutputPath:      outPath,
				Columns:         tc.cols,
				FontSize:        11,
				ShowLineNumbers: true,
			})
			if err != nil {
				t.Fatalf("RenderFileToCards failed: %v", err)
			}

			if len(res.Files) == 0 {
				t.Fatalf("expected generated files")
			}

			// Open first page
			f, err := os.Open(res.Files[0])
			if err != nil {
				t.Fatalf("failed to open generated card %s: %v", res.Files[0], err)
			}
			defer f.Close()

			img, err := png.Decode(f)
			if err != nil {
				t.Fatalf("failed to decode png %s: %v", res.Files[0], err)
			}

			bounds := img.Bounds()
			if bounds.Dx() != res.Width || bounds.Dy() != res.Height {
				t.Errorf("decoded image bounds (%dx%d) != result (%dx%d)", bounds.Dx(), bounds.Dy(), res.Width, res.Height)
			}

			// Check column 1 (second column) contains non-background pixels (rendered text)
			col1StartX := (res.Width / tc.cols) + 10
			col1EndX := res.Width - 16
			col1StartY := 45
			col1EndY := res.Height - 10

			bg := DarkTheme.Bg
			foundTextInCol1 := false
			for y := col1StartY; y < col1EndY; y++ {
				for x := col1StartX; x < col1EndX; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)
					if r8 != bg.R || g8 != bg.G || b8 != bg.B {
						foundTextInCol1 = true
						break
					}
				}
				if foundTextInCol1 {
					break
				}
			}

			if !foundTextInCol1 {
				t.Errorf("expected rendered text in column 1, but area was completely empty/background")
			}
		})
	}
}

func TestRenderFileToCards_CroppedWidth(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "short.png")

	lines := []string{
		"a = 1",
		"b = 2",
		"c = 3",
	}

	res, err := RenderFileToCards(lines, "short.py", RenderOptions{
		OutputPath:      outPath,
		Columns:         1,
		FontSize:        11,
		ShowLineNumbers: true,
	})
	if err != nil {
		t.Fatalf("RenderFileToCards failed: %v", err)
	}

	// Should be tightly cropped, well under 600px, not 1568px
	if res.Width > 600 {
		t.Errorf("expected tightly cropped card width < 600px for short lines, got %d px", res.Width)
	}
	if res.Height > 300 {
		t.Errorf("expected tightly cropped card height < 300px for 3 lines, got %d px", res.Height)
	}
}

func TestDrawStringBounded(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 20))
	bg := color.RGBA{R: 10, G: 10, B: 10, A: 255}
	fg := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	for y := 0; y < 20; y++ {
		for x := 0; x < 100; x++ {
			img.SetRGBA(x, y, bg)
		}
	}

	font := Font5x8 // cw = 6, ch = 8
	startX := 10
	startY := 5
	// Allow exactly 3 characters = 3 * 6 = 18 px -> maxX = 10 + 18 = 28
	maxX := startX + font.CharWidth*3
	drawnWidth := font.DrawStringBounded(img, "ABCDEFGH", startX, startY, maxX, fg)

	if drawnWidth != font.CharWidth*3 {
		t.Errorf("expected drawnWidth %d, got %d", font.CharWidth*3, drawnWidth)
	}

	// Verify that at x >= maxX, no pixel was modified from bg
	for y := 0; y < 20; y++ {
		for x := maxX; x < 100; x++ {
			c := img.RGBAAt(x, y)
			if c != bg {
				t.Fatalf("pixel at (%d, %d) was modified to %+v, want %+v (bleed past maxX %d)", x, y, c, bg, maxX)
			}
		}
	}

	// Verify that at least some pixels in the bounded area were drawn
	var modifiedCount int
	for y := startY; y < startY+font.CharHeight; y++ {
		for x := startX; x < maxX; x++ {
			if img.RGBAAt(x, y) == fg {
				modifiedCount++
			}
		}
	}
	if modifiedCount == 0 {
		t.Fatalf("expected glyph pixels to be drawn in [%d, %d)", startX, maxX)
	}

	// Test boundary within a character cell: maxX = startX + font.CharWidth*3 + 2
	for y := 0; y < 20; y++ {
		for x := 0; x < 100; x++ {
			img.SetRGBA(x, y, bg)
		}
	}
	maxX2 := startX + font.CharWidth*3 + 2
	drawnWidth2 := font.DrawStringBounded(img, "ABCDEFGH", startX, startY, maxX2, fg)
	if drawnWidth2 != font.CharWidth*3 {
		t.Errorf("expected drawnWidth2 %d, got %d", font.CharWidth*3, drawnWidth2)
	}
	for y := 0; y < 20; y++ {
		for x := startX + font.CharWidth*3; x < 100; x++ {
			c := img.RGBAAt(x, y)
			if c != bg {
				t.Fatalf("pixel at (%d, %d) was modified when maxX was %d (partial glyph bleed)", x, y, maxX2)
			}
		}
	}
}

func TestRenderFileToCards_NoBleedAcrossColumns(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "long_lines_2col.png")

	// Create 100 lines with 200 characters each
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = fmt.Sprintf("// Line %03d: %s", i+1, strings.Repeat("A_very_long_identifier_sequence_that_exceeds_column_width_", 4))
	}

	res, err := RenderFileToCards(lines, "long.go", RenderOptions{
		OutputPath:      outPath,
		Columns:         2,
		FontSize:        11,
		ShowLineNumbers: true,
	})
	if err != nil {
		t.Fatalf("RenderFileToCards failed: %v", err)
	}

	f, err := os.Open(res.Files[0])
	if err != nil {
		t.Fatalf("failed to open generated png: %v", err)
	}
	defer f.Close()

	imgDecoded, err := png.Decode(f)
	if err != nil {
		t.Fatalf("failed to decode png: %v", err)
	}
	bounds := imgDecoded.Bounds()
	img := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.Set(x, y, imgDecoded.At(x, y))
		}
	}

	// Layout geometry matching render.go
	font := Font5x8
	cw := font.CharWidth
	headerHeight := 36
	paddingX := 16
	paddingY := 12
	colGap := 16

	maxLineNum := len(lines)
	digits := len(fmt.Sprintf("%d", maxLineNum))
	if digits < 3 {
		digits = 3
	}
	gutterWidth := (digits+1)*cw + 12

	maxLineLen := 120 // capped at 120
	colWidth := gutterWidth + (maxLineLen * cw) + 16
	if (2*colWidth)+colGap+(paddingX*2) > 1568 {
		colWidth = (1568 - (paddingX * 2) - colGap) / 2
	}

	col0X := paddingX
	col1X := paddingX + colWidth + colGap
	sepX := col1X - (colGap / 2)
	maxCol0X := col0X + colWidth - 4

	bg := DarkTheme.Bg
	colSep := DarkTheme.ColumnSep

	// Verify that between maxCol0X and sepX (excluding sepX), all pixels are bg
	for y := headerHeight + paddingY; y < res.Height-paddingY; y++ {
		for x := maxCol0X; x < sepX; x++ {
			c := img.RGBAAt(x, y)
			if c != bg {
				t.Fatalf("pixel at (%d, %d) between col0 and sep was modified to %+v, want bg %+v", x, y, c, bg)
			}
		}
	}

	// Verify that at sepX the column separator is drawn
	sepFound := false
	for y := headerHeight + paddingY; y < res.Height-paddingY; y++ {
		if img.RGBAAt(sepX, y) == colSep {
			sepFound = true
			break
		}
	}
	if !sepFound {
		t.Errorf("expected column separator line at x=%d", sepX)
	}

	// Verify that between sepX+1 and col1X, all pixels are bg
	for y := headerHeight + paddingY; y < res.Height-paddingY; y++ {
		for x := sepX + 1; x < col1X; x++ {
			c := img.RGBAAt(x, y)
			if c != bg {
				t.Fatalf("pixel at (%d, %d) between sep and col1 was modified to %+v, want bg %+v", x, y, c, bg)
			}
		}
	}
}

func TestRenderFileToCards_AutoColumnLongLines(t *testing.T) {
	tmpDir := t.TempDir()

	// 100 lines of length 95 chars
	longLines := make([]string, 100)
	for i := range longLines {
		longLines[i] = fmt.Sprintf("const ConfigEntry%03d = \"value_%s\"", i+1, strings.Repeat("x", 65))
	}

	// Default three-column packing applies to long lines with soft wrapping.
	res1, err := RenderFileToCards(longLines, "config.go", RenderOptions{
		OutputPath: filepath.Join(tmpDir, "auto.png"),
		Columns:    0,
	})
	if err != nil {
		t.Fatalf("RenderFileToCards failed: %v", err)
	}
	if res1.Columns != 3 {
		t.Errorf("expected default 3 columns for wrapped long lines, got %d", res1.Columns)
	}

	// Explicit Columns: 2 should still be respected even for long lines
	res2, err := RenderFileToCards(longLines, "config.go", RenderOptions{
		OutputPath: filepath.Join(tmpDir, "explicit2.png"),
		Columns:    2,
	})
	if err != nil {
		t.Fatalf("RenderFileToCards failed: %v", err)
	}
	if res2.Columns != 2 {
		t.Errorf("expected explicit Columns=2 to be respected, got %d", res2.Columns)
	}

	// Moderate lines use the same default three-column packing.
	shortLines := make([]string, 100)
	for i := range shortLines {
		shortLines[i] = fmt.Sprintf("const Item%03d = %d", i+1, i+1)
	}
	res3, err := RenderFileToCards(shortLines, "short.go", RenderOptions{
		OutputPath: filepath.Join(tmpDir, "auto2.png"),
		Columns:    0,
	})
	if err != nil {
		t.Fatalf("RenderFileToCards failed: %v", err)
	}
	if res3.Columns != 3 {
		t.Errorf("expected default 3 columns for normal lines, got %d", res3.Columns)
	}
}

func TestRenderFileToCards_OneColumnWidthExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "wide_1col.png")

	// 10 lines with 200 characters
	longLine := "// " + strings.Repeat("A_very_long_code_identifier_sequence_that_spans_broadly_", 3) + "END"
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = fmt.Sprintf("%s_%02d", longLine, i+1)
	}

	res, err := RenderFileToCards(lines, "wide.go", RenderOptions{
		OutputPath:      outPath,
		Columns:         1,
		FontSize:        11,
		ShowLineNumbers: true,
	})
	if err != nil {
		t.Fatalf("RenderFileToCards failed: %v", err)
	}

	if res.Columns != 1 {
		t.Errorf("expected 1 column, got %d", res.Columns)
	}
	// With 200 chars and Font5x8 (cw=6), width should expand to > 1100px (not clamped to 120 chars ~800px)
	if res.Width < 1100 {
		t.Errorf("expected expanded width > 1100px for 200-char line, got %d px", res.Width)
	}
	if res.Width > 1568 {
		t.Errorf("expected width within 1568 bound, got %d px", res.Width)
	}
	if len(res.Files) == 0 {
		t.Fatalf("expected generated file")
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("expected output file on disk: %v", err)
	}
}

func TestRenderFileToCards_SoftWrappingAndContinuation(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "soft_wrapped.png")

	// Create a line of 220 chars in a 2-column layout (each column capacity ~110 chars)
	lines := []string{
		"const FirstLine = 1",
		"const VeryLongLine = \"" + strings.Repeat("ABCDEFGHIJ", 20) + "\"",
		"const ThirdLine = 3",
	}

	res, err := RenderFileToCards(lines, "wrap.go", RenderOptions{
		OutputPath:      outPath,
		Columns:         2,
		Wrap:            "soft",
		FontSize:        11,
		ShowLineNumbers: true,
	})
	if err != nil {
		t.Fatalf("RenderFileToCards with soft wrap failed: %v", err)
	}

	if res.TotalLines != 3 {
		t.Errorf("expected TotalLines to report 3 source lines, got %d", res.TotalLines)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("expected rendered card at %s: %v", outPath, err)
	}
}

func TestRenderFileToCards_TruncateEllipsis(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "truncated.png")

	lines := []string{
		"const FirstLine = 1",
		"const VeryLongLine = \"" + strings.Repeat("ABCDEFGHIJ", 20) + "\"",
		"const ThirdLine = 3",
	}

	res, err := RenderFileToCards(lines, "trunc.go", RenderOptions{
		OutputPath:      outPath,
		Columns:         2,
		Wrap:            "truncate",
		FontSize:        11,
		ShowLineNumbers: true,
	})
	if err != nil {
		t.Fatalf("RenderFileToCards with truncate failed: %v", err)
	}

	if res.TotalLines != 3 {
		t.Errorf("expected TotalLines = 3, got %d", res.TotalLines)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("expected file %s to exist: %v", outPath, err)
	}
}

func TestSplitTokensByLength(t *testing.T) {
	tokens := []Token{
		{Type: TokenKeyword, Text: "func"},
		{Type: TokenText, Text: " "},
		{Type: TokenTypeIdent, Text: "processLongIdentifier"},
	}

	// Split at 10 chars: "func " (5) + "proce" (5)
	head, tail := splitTokensByLength(tokens, 10)
	if len(head) != 3 {
		t.Fatalf("expected 3 head tokens, got %d: %+v", len(head), head)
	}
	if head[0].Text != "func" || head[1].Text != " " || head[2].Text != "proce" {
		t.Errorf("unexpected head tokens: %+v", head)
	}
	if head[2].Type != TokenTypeIdent {
		t.Errorf("expected head token type preserved as TokenTypeIdent, got %v", head[2].Type)
	}

	if len(tail) != 1 {
		t.Fatalf("expected 1 tail token, got %d: %+v", len(tail), tail)
	}
	if tail[0].Text != "ssLongIdentifier" {
		t.Errorf("expected tail text 'ssLongIdentifier', got %q", tail[0].Text)
	}
	if tail[0].Type != TokenTypeIdent {
		t.Errorf("expected tail token type preserved as TokenTypeIdent, got %v", tail[0].Type)
	}
}
