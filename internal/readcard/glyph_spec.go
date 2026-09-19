package readcard

import (
	"embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

// glyphSpecYAML is the single source of truth for every bitmap glyph: one
// pixel matrix per font size, keyed by character.
//
//go:embed spec/charset.yaml
var glyphCharsetYAML []byte

//go:embed spec/glyphs-*.yaml
var glyphSpecYAML embed.FS

type glyphSpecFile struct {
	Charset string              `yaml:"charset"`
	Glyphs  map[string][]string `yaml:"glyphs"`
}

var glyphCharset = parseGlyphCharset()

func parseGlyphCharset() string {
	var file glyphSpecFile
	if err := yaml.Unmarshal(glyphCharsetYAML, &file); err != nil {
		panic(fmt.Sprintf("readcard glyph spec: %v", err))
	}
	if file.Charset == "" {
		panic("readcard glyph spec: empty charset")
	}
	return file.Charset
}

func parseGlyphSpec(size string) glyphSpecFile {
	b, err := glyphSpecYAML.ReadFile("spec/glyphs-" + size + ".yaml")
	if err != nil {
		panic(fmt.Sprintf("readcard glyph spec %s: %v", size, err))
	}
	var file glyphSpecFile
	if err := yaml.Unmarshal(b, &file); err != nil {
		panic(fmt.Sprintf("readcard glyph spec %s: %v", size, err))
	}
	return file
}

// SupportedGlyphCharset returns the ordered set of glyphs shown in the golden matrices.
func SupportedGlyphCharset() string { return glyphCharset }

func supportedGlyphCharset() string { return glyphCharset }

// glyphBitmaps packs the spec matrices of one font size into MSB-first row bytes.
func glyphBitmaps(size string, width, height int) map[rune][]byte {
	spec := parseGlyphSpec(size)
	result := make(map[rune][]byte, len(spec.Glyphs))
	for key, rows := range spec.Glyphs {
		runes := []rune(key)
		if len(runes) != 1 || len(rows) != height {
			panic(fmt.Sprintf("readcard glyph spec: invalid %s glyph %q", size, key))
		}
		bits := make([]byte, height)
		for y, row := range rows {
			if len([]rune(row)) != width {
				panic(fmt.Sprintf("readcard glyph spec: invalid %s row %d for %q", size, y, key))
			}
			for x, bit := range row {
				if bit == '1' {
					bits[y] |= 1 << (7 - x)
				}
			}
		}
		result[runes[0]] = bits
	}
	// Upstream is authoritative; YAML contains only its missing glyphs.
	for r, bits := range upstreamBitmaps(size, width, height) {
		result[r] = bits
	}
	if fallback := result['?']; fallback != nil {
		for _, r := range []rune(glyphCharset) {
			if _, ok := result[r]; !ok {
				result[r] = fallback
			}
		}
	}
	return result
}
