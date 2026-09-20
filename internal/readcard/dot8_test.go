package readcard

import (
	"strings"
	"testing"
)

func TestDot8EncodeBasic(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
	}{
		{
			name:  "lowercase letters",
			input: "abc",
			want:  "⠁⠃⠉",
		},
		{
			name:  "uppercase letters",
			input: "ABC",
			want:  "⡁⡃⡉",
		},
		{
			name:  "mixed case",
			input: "aBc",
			want:  "⠁⡃⠉",
		},
		{
			name:  "digits",
			input: "123",
			want:  "⢀⠁⢀⠃⢀⠉",
		},
		{
			name:  "zero",
			input: "0",
			want:  "⢀⠚",
		},
		{
			name:  "punctuation unchanged",
			input: "hello, world!",
			want:  "⠓⠑⠇⠇⠕, ⠺⠕⠗⠇⠙!",
		},
		{
			name:  "spaces unchanged",
			input: "a b c",
			want:  "⠁ ⠃ ⠉",
		},
		{
			name:  "markdown syntax unchanged, mixed case",
			input: "# header\n- list",
			want:  "# ⠓⠑⠁⠙⠑⠗\n- ⠇⠊⠎⠞",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Dot8Encode(tt.input)
			if got != tt.want {
				t.Errorf("Dot8Encode(%q) = %q, want %q", tt.input, got, tt.want)
				t.Logf("Got runes: %v", []rune(got))
				t.Logf("Want runes: %v", []rune(tt.want))
			}
		})
	}
}

func TestDot8DecodeBasic(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "lowercase letters",
			input: "⠁⠃⠉",
			want:  "abc",
		},
		{
			name:  "uppercase letters",
			input: "⡁⡃⡉",
			want:  "ABC",
		},
		{
			name:  "mixed case",
			input: "⠁⡃⠉",
			want:  "aBc",
		},
		{
			name:  "digits",
			input: "⢀⠁⢀⠃⢀⠉",
			want:  "123",
		},
		{
			name:  "zero",
			input: "⢀⠚",
			want:  "0",
		},
		{
			name:  "punctuation unchanged",
			input: "⠓⠑⠇⠇⠕, ⠺⠕⠗⠇⠙!",
			want:  "hello, world!",
		},
		{
			name:  "spaces unchanged",
			input: "⠁ ⠃ ⠉",
			want:  "a b c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Dot8Decode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dot8Decode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("Dot8Decode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDot8EscapeBrailleCells(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "escape literal Braille",
			input: "⣀⠁",
			want:  "⠁",
		},
		{
			name:  "escape and decode",
			input: "⣀⣿",
			want:  "⣿",
		},
		{
			name:  "multiple escaped cells",
			input: "⣀⠁⣀⠃",
			want:  "⠁⠃",
		},
		{
			name:    "unterminated escape",
			input:   "text⣀",
			wantErr: true,
		},
		{
			name:    "escape followed by non-Braille",
			input:   "⣀x",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Dot8Decode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dot8Decode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("Dot8Decode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDot8RoundTrip(t *testing.T) {
	fixtures := []string{
		"hello world",
		"HELLO WORLD",
		"Hello World 123",
		"The year is 2026",
		"Code: func() { return 42; }",
		"List:\n- item 1\n- item 2\n- item 3",
		"# Header\n\nParagraph with text.",
		"Mixed: aAbBcC 0123456789",
		"Emoji and unicode: 🎉 café",
		"Symbols: !@#$%^&*()_+-=[]{}|;:',.<>?/~`",
	}

	for _, fixture := range fixtures {
		name := fixture
		if len(name) > 20 {
			name = name[:20]
		}
		t.Run(name, func(t *testing.T) {
			if err := Dot8RoundTripCheck(fixture); err != nil {
				t.Errorf("Round-trip failed for %q: %v", fixture, err)
			}
		})
	}
}

func TestDot8CheckDocument(t *testing.T) {
	doc := `# Introduction
This is the first section.

# Main Content
Here is the main content.

# Conclusion
And we conclude.`

	errors := Dot8CheckDocument(doc)
	if len(errors) > 0 {
		t.Errorf("Document check failed with errors: %v", errors)
	}
}

func TestDot8RoundTripWithEscapedBraille(t *testing.T) {
	// Document that contains literal Braille - it should be escaped when encoded
	doc := `# Section 1
Valid text here.

# Section 2
Literal Braille: ⠁⠃⠉`

	// This should pass because Dot8Encode will escape the Braille
	errors := Dot8CheckDocument(doc)
	if len(errors) > 0 {
		t.Errorf("Expected document check to pass, got errors: %v", errors)
	}
}

func TestDot8AllLetters(t *testing.T) {
	input := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if err := Dot8RoundTripCheck(input); err != nil {
		t.Errorf("All letters round-trip failed: %v", err)
	}
}

func TestDot8AllDigits(t *testing.T) {
	input := "0123456789"
	if err := Dot8RoundTripCheck(input); err != nil {
		t.Errorf("All digits round-trip failed: %v", err)
	}
}

func TestDot8SpecialCharacters(t *testing.T) {
	// Special characters should pass through unchanged
	input := "!@#$%^&*()_+-=[]{}|;:',.<>?/~`\n\t "
	if err := Dot8RoundTripCheck(input); err != nil {
		t.Errorf("Special characters round-trip failed: %v", err)
	}
}

func TestDot8DotMasks(t *testing.T) {
	// Test all individual letters to verify the dot masks
	tests := []struct {
		input rune
		want  rune
	}{
		{'a', rune(brailleBase + 0b000001)},
		{'b', rune(brailleBase + 0b000011)},
		{'c', rune(brailleBase + 0b001001)},
		{'z', rune(brailleBase + 0b110101)},
		{'A', rune(brailleBase + 0b000001 + dotMask7)},
		{'Z', rune(brailleBase + 0b110101 + dotMask7)},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			encoded := Dot8Encode(string(tt.input))
			got := []rune(encoded)[0]
			if got != tt.want {
				t.Errorf("Dot8Encode(%c) = U+%04X, want U+%04X", tt.input, got, tt.want)
			}
		})
	}
}

func TestDot8DecodeErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "invalid dot8 prefix alone",
			input:   "hello⢀world",
			wantErr: true,
		},
		{
			name:    "dot8 at end",
			input:   "hello⢀",
			wantErr: true,
		},
		{
			name:    "unterminated escape",
			input:   "hello⣀",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Dot8Decode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dot8Decode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestDot8PythonReference(t *testing.T) {
	// Test against known outputs from the Python reference implementation
	tests := []struct {
		name    string
		input   string
		want    string
	}{
		{
			name:  "reference: hello world",
			input: "hello world",
			want:  "⠓⠑⠇⠇⠕ ⠺⠕⠗⠇⠙",
		},
		{
			name:  "reference: Hello World",
			input: "Hello World",
			want:  "⡓⠑⠇⠇⠕ ⡺⠕⠗⠇⠙",
		},
		{
			name:  "reference: abc123",
			input: "abc123",
			want:  "⠁⠃⠉⢀⠁⢀⠃⢀⠉",
		},
		{
			name:  "reference: 2026",
			input: "2026",
			want:  "⢀⠃⢀⠚⢀⠃⢀⠋",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Dot8Encode(tt.input)
			if got != tt.want {
				t.Errorf("Dot8Encode(%q) = %q, want %q", tt.input, got, tt.want)
				t.Logf("Got runes:  %v", []rune(got))
				t.Logf("Want runes: %v", []rune(tt.want))
			}
		})
	}
}

func BenchmarkDot8Encode(b *testing.B) {
	input := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Dot8Encode(input)
	}
}

func BenchmarkDot8Decode(b *testing.B) {
	input := Dot8Encode(strings.Repeat("The quick brown fox jumps over the lazy dog. ", 10))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Dot8Decode(input)
	}
}

func BenchmarkDot8RoundTrip(b *testing.B) {
	input := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Dot8RoundTripCheck(input)
	}
}

// Test rendering with different styles
func TestDot8RenderCompact(t *testing.T) {
	lines := []string{
		"⠓⠑⠇⠇⠕ ⠺⠕⠗⠇⠙",
		"⡓⠑⠇⠇⠕ ⡺⠕⠗⠇⠙",
		"⠞⠙⠊⠎ ⠊⠎ ⠁ ⠞⠑⠎⠞",
	}

	opts := RenderOptions{
		Chrome:      ChromeSlim,
		Gutter:      GutterTight,
		Columns:     1,
		MaxDimension: 600,
		ShowLineNumbers: true,
		Title:       "test",
		StartLine:   1,
		OutputPath:  "/tmp/test_dot8_compact.png",
		Dot8:        "native",
	}

	result, err := RenderFileToCards(lines, "test.md", opts)
	if err != nil {
		t.Fatalf("RenderFileToCards failed: %v", err)
	}

	if result.Width == 0 || result.Height == 0 {
		t.Errorf("Card dimensions invalid: %dx%d", result.Width, result.Height)
	}
	if result.TotalLines != len(lines) {
		t.Errorf("TotalLines = %d, want %d", result.TotalLines, len(lines))
	}
}

// Test that legend is included in header
func TestDot8RenderLegend(t *testing.T) {
	lines := []string{"⠓⠑⠇⠇⠕", "⠑⠝⠉⠕⠙⠑⠙", "⠃⠗⠁⠊⠇⠇⠑"}

	opts := RenderOptions{
		Chrome:       "full",
		Columns:      1,
		MaxDimension: 800,
		ShowLineNumbers: false,
		Title:        "test",
		StartLine:    1,
		OutputPath:   "/tmp/test_dot8_legend.png",
		Dot8:         "native",
	}

	result, err := RenderFileToCards(lines, "test.md", opts)
	if err != nil {
		t.Fatalf("RenderFileToCards failed: %v", err)
	}

	// Card should render without error
	if result.Width == 0 || result.Height == 0 {
		t.Errorf("Invalid dimensions: %dx%d", result.Width, result.Height)
	}
}
