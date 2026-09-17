package readcard

import (
	"image"
	"image/color"
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
		FontSize:     11,
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

