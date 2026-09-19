package readcard

import (
	_ "embed"
	"fmt"
	"image/color"
	"strconv"

	"gopkg.in/yaml.v3"
)

//go:embed spec/glyphs.yaml
var glyphSpecYAML []byte

//go:embed spec/font_5x8.yaml
var font5x8SpecYAML []byte

//go:embed spec/font_tables.yaml
var fontTablesSpecYAML []byte

//go:embed spec/font_extensions.yaml
var fontExtensionsSpecYAML []byte

type glyphSpecFile struct {
	Charset  string                         `yaml:"charset"`
	Glyphs   map[string][]string            `yaml:"glyphs"`
	Profiles map[string]map[string][]string `yaml:"profiles"`
}

func supportedGlyphCharset() string {
	if specGlyphFileData.Charset == "" {
		panic("readcard glyph spec: empty charset")
	}
	return specGlyphFileData.Charset
}

// SupportedGlyphCharset returns the ordered set of glyphs covered by the spec.
func SupportedGlyphCharset() string { return supportedGlyphCharset() }

type font5x8SpecFile struct {
	Fonts map[string]struct {
		Glyphs map[string][]string `yaml:"glyphs"`
	} `yaml:"fonts"`
}

type fontTableSpecFile struct {
	Fonts map[string]struct {
		Glyphs map[string][]string `yaml:"glyphs"`
	} `yaml:"fonts"`
}

type fontExtensionSpecFile struct {
	Fonts map[string]struct {
		Glyphs map[string][]string `yaml:"glyphs"`
	} `yaml:"fonts"`
}

var specGlyphs = loadGlyphSpec()
var specGlyphProfiles = loadGlyphProfiles()
var specFont5x8 = loadFont5x8()
var specFontTables = loadFontTables()
var specFontExtensions = loadFontExtensions()

func loadFontExtensions() map[string]map[rune][]byte {
	var file fontExtensionSpecFile
	if err := yaml.Unmarshal(fontExtensionsSpecYAML, &file); err != nil {
		panic(fmt.Sprintf("readcard font extension spec: %v", err))
	}
	result := make(map[string]map[rune][]byte, len(file.Fonts))
	for name, font := range file.Fonts {
		glyphs := make(map[rune][]byte, len(font.Glyphs))
		for key, rows := range font.Glyphs {
			runes := []rune(key)
			if len(runes) != 1 || len(rows) == 0 {
				panic(fmt.Sprintf("readcard font extension spec: invalid %s glyph %q", name, key))
			}
			bits := make([]byte, len(rows))
			for i, row := range rows {
				value, err := strconv.ParseUint(row, 2, 8)
				if err != nil {
					panic(fmt.Sprintf("readcard font extension spec: invalid %s glyph %q row %q", name, key, row))
				}
				bits[i] = byte(value)
			}
			glyphs[runes[0]] = bits
		}
		result[name] = glyphs
	}
	return result
}

func loadFontTables() map[string]map[rune][]string {
	var file fontTableSpecFile
	if err := yaml.Unmarshal(fontTablesSpecYAML, &file); err != nil {
		panic(fmt.Sprintf("readcard font table spec: %v", err))
	}
	result := make(map[string]map[rune][]string, len(file.Fonts))
	for name, font := range file.Fonts {
		result[name] = parseGlyphs(name, font.Glyphs)
	}
	return result
}

func loadFont5x8() map[rune][]uint8 {
	var file font5x8SpecFile
	if err := yaml.Unmarshal(font5x8SpecYAML, &file); err != nil {
		panic(fmt.Sprintf("readcard 5x8 font spec: %v", err))
	}
	font := file.Fonts["retro_pixel_5x8"]
	result := make(map[rune][]uint8, len(font.Glyphs))
	for key, rows := range font.Glyphs {
		runes := []rune(key)
		if len(runes) != 1 || len(rows) != 8 {
			panic(fmt.Sprintf("readcard 5x8 font spec: invalid glyph %q", key))
		}
		bits := make([]uint8, 8)
		for y, row := range rows {
			if len([]rune(row)) != 6 {
				panic(fmt.Sprintf("readcard 5x8 font spec: invalid row for %q", key))
			}
			for x, bit := range row {
				if bit == '1' {
					bits[y] |= 1 << (5 - x)
				}
			}
		}
		result[runes[0]] = bits
	}
	return result
}

func loadGlyphSpec() map[rune][]string {
	return parseGlyphs("base", specGlyphFileData.Glyphs)
}

func loadGlyphProfiles() map[string]map[rune][]string {
	result := make(map[string]map[rune][]string, len(specGlyphFileData.Profiles))
	for profile, glyphs := range specGlyphFileData.Profiles {
		result[profile] = parseGlyphs(profile, glyphs)
	}
	return result
}

var specGlyphFileData = parseGlyphSpecFile()

func parseGlyphSpecFile() glyphSpecFile {
	var file glyphSpecFile
	if err := yaml.Unmarshal(glyphSpecYAML, &file); err != nil {
		panic(fmt.Sprintf("readcard glyph spec: %v", err))
	}
	return file
}

func parseGlyphs(name string, glyphs map[string][]string) map[rune][]string {
	result := make(map[rune][]string, len(glyphs))
	for key, pattern := range glyphs {
		runes := []rune(key)
		if len(runes) != 1 || len(pattern) == 0 {
			panic(fmt.Sprintf("readcard glyph spec: invalid %s glyph %q", name, key))
		}
		rowWidth := len([]rune(pattern[0]))
		for _, row := range pattern {
			if len([]rune(row)) != rowWidth {
				panic(fmt.Sprintf("readcard glyph spec: uneven %s pattern for %q", name, key))
			}
		}
		result[runes[0]] = pattern
	}
	return result
}

func glyphPattern(r rune) ([]string, bool) {
	pattern, ok := specGlyphs[r]
	return pattern, ok
}

func glyphPatternForFont(f *MonospaceFont, r rune) ([]string, bool) {
	profile := ""
	if f == Font3x5 {
		profile = "micro"
	} else if f == Font5x8 {
		profile = "default"
	}
	if pattern, ok := specGlyphProfiles[profile][r]; ok {
		return pattern, true
	}
	return glyphPattern(r)
}

func drawGlyphPattern(img interface{ SetRGBA(x, y int, c color.RGBA) }, pattern []string, x, y int, col color.RGBA, width, height int) {
	if len(pattern) == 0 {
		return
	}
	gridWidth := len([]rune(pattern[0]))
	if gridWidth == 0 {
		return
	}
	for py, row := range pattern {
		for px, bit := range row {
			if bit != '1' {
				continue
			}
			dx := x + px*width/gridWidth
			dy := y + py*height/len(pattern)
			img.SetRGBA(dx, dy, col)
		}
	}
}
