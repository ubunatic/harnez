package readcard

import (
	_ "embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

// glyphSpecYAML is the single source of truth for every bitmap glyph: one
// pixel matrix per font size, keyed by character.
//
//go:embed spec/glyphs.yaml
var glyphSpecYAML []byte

type glyphSpecFile struct {
	Charset string                         `yaml:"charset"`
	Glyphs  map[string]map[string][]string `yaml:"glyphs"`
}

var glyphSpec = parseGlyphSpec()

func parseGlyphSpec() glyphSpecFile {
	var file glyphSpecFile
	if err := yaml.Unmarshal(glyphSpecYAML, &file); err != nil {
		panic(fmt.Sprintf("readcard glyph spec: %v", err))
	}
	if file.Charset == "" {
		panic("readcard glyph spec: empty charset")
	}
	return file
}

// SupportedGlyphCharset returns the ordered set of glyphs shown in the golden matrices.
func SupportedGlyphCharset() string { return glyphSpec.Charset }

func supportedGlyphCharset() string { return glyphSpec.Charset }

// glyphBitmaps packs the spec matrices of one font size into MSB-first row bytes.
func glyphBitmaps(size string, width, height int) map[rune][]byte {
	result := make(map[rune][]byte, len(glyphSpec.Glyphs))
	for key, sizes := range glyphSpec.Glyphs {
		rows, ok := sizes[size]
		if !ok {
			continue
		}
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
	// For non-default sizes the embedded upstream font is authoritative; YAML
	// remains the small, explicit fill-in layer for code points it lacks.
	for r, bits := range upstreamBitmaps(size, width, height) {
		result[r] = bits
	}
	return result
}
