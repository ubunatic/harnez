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
		input      string
		total      int
		wantStart  int
		wantEnd    int
		wantErr    bool
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




