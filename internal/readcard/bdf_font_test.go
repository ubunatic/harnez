package readcard

import (
	"strings"
	"testing"
)

func TestParseEmbeddedBDFFixture(t *testing.T) {
	font, err := parseEmbeddedBDF(strings.NewReader(`FONTBOUNDINGBOX 8 8 0 -2
STARTCHAR A
ENCODING 65
BBX 4 4 1 0
BITMAP
F0
90
90
F0
ENDCHAR
ENDFONT
`))
	if err != nil {
		t.Fatal(err)
	}
	g, ok := font.glyphs['A']
	if !ok || g.width != 4 || g.height != 4 || len(g.rows) != 4 {
		t.Fatalf("parsed fixture glyph = %#v, want 4x4 with four rows", g)
	}
	if got := embeddedMatrix(g, font, 8, 8); got[2] != 0x78 || got[3] != 0x48 {
		t.Fatalf("embedded matrix = %#v, want fixture pixels aligned to baseline", got)
	}
}

func TestGlyphSpecsHaveNoUpstreamOverlap(t *testing.T) {
	for _, tc := range []struct {
		size          string
		width, height int
	}{
		{"3x5", 4, 6}, {"6x12", 7, 12}, {"7x13", 7, 13}, {"8x16", 8, 16},
	} {
		spec := parseGlyphSpec(tc.size)
		upstream := upstreamBitmaps(tc.size, tc.width, tc.height)
		for key := range spec.Glyphs {
			if key == "?" {
				continue // ? is intentionally editable as the fallback glyph.
			}
			if tc.size == "3x5" {
				continue // 3x5 is an editable full baseline for Dot8 tuning.
			}
			r := []rune(key)
			if len(r) == 1 {
				if _, ok := upstream[r[0]]; ok {
					t.Errorf("%s spec contains upstream glyph %q", tc.size, key)
				}
			}
		}
	}
}

func TestQuestionMarkFallbackUsesUpstreamOrSpec(t *testing.T) {
	for _, tc := range []struct {
		size          string
		width, height int
		font          map[rune][]byte
	}{
		{"5x8", 6, 8, glyphBitmaps("5x8", 6, 8)},
		{"3x5", 4, 6, glyphBitmaps("3x5", 4, 6)},
		{"6x12", 7, 12, glyphBitmaps("6x12", 7, 12)},
		{"7x13", 7, 13, glyphBitmaps("7x13", 7, 13)},
		{"8x16", 8, 16, glyphBitmaps("8x16", 8, 16)},
	} {
		upstream := upstreamBitmaps(tc.size, tc.width, tc.height)
		if len(tc.font['?']) == 0 {
			t.Errorf("%s has no editable question-mark fallback", tc.size)
		}
		if want, ok := upstream['?']; ok && string(tc.font['?']) != string(want) {
			t.Errorf("%s does not use the upstream question mark", tc.size)
		}
		for _, r := range []rune(glyphCharset) {
			if _, ok := tc.font[r]; !ok {
				t.Errorf("%s has no fallback matrix for %q", tc.size, r)
			}
		}
	}
}
