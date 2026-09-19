package readcard

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var pipelineFonts = []struct {
	name          string
	font          func() *MonospaceFont
	spec          string
	width, height int
	upstream      bool
}{
	{"5x8", func() *MonospaceFont { return Font5x8 }, "5x8", 6, 8, false},
	{"3x5", func() *MonospaceFont { return Font3x5 }, "3x5", 4, 6, true},
	{"6x12", func() *MonospaceFont { return Font6x12 }, "6x12", 7, 12, true},
	{"7x13", func() *MonospaceFont { return DefaultFont7x13 }, "7x13", 7, 13, true},
	{"8x16", func() *MonospaceFont { return DefaultFont8x16 }, "8x16", 8, 16, true},
}

var (
	inkColor = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	bgColor  = color.RGBA{A: 255}
)

// renderTextPNG rasterizes text with font, encodes it as a PNG and decodes it
// again, so assertions run on the bytes a user would actually receive.
func renderTextPNG(t *testing.T, f *MonospaceFont, text string) *image.RGBA {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, f.CharWidth*len([]rune(text))+2, f.CharHeight+2))
	drawRect(img, 0, 0, img.Bounds().Dx(), img.Bounds().Dy(), bgColor)
	f.DrawString(img, text, 0, 0, inkColor)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	out := image.NewRGBA(decoded.Bounds())
	for y := 0; y < out.Bounds().Dy(); y++ {
		for x := 0; x < out.Bounds().Dx(); x++ {
			out.Set(x, y, decoded.At(x, y))
		}
	}
	return out
}

// cell extracts the cw x ch pixels of the i-th character cell as text rows.
func cell(img *image.RGBA, i, cw, ch int) []string {
	rows := make([]string, ch)
	for y := 0; y < ch; y++ {
		var sb strings.Builder
		for x := 0; x < cw; x++ {
			if img.RGBAAt(i*cw+x, y) == inkColor {
				sb.WriteByte('1')
			} else {
				sb.WriteByte(' ')
			}
		}
		rows[y] = sb.String()
	}
	return rows
}

func joinRows(rows []string) string { return strings.Join(rows, "\n") }

func hasInk(rows []string) bool { return strings.Contains(joinRows(rows), "1") }

// TestTextToPNGMatchesSpecMatrices reads the YAML specs directly (not through
// glyph_spec.go) and checks every spec glyph survives text -> PNG -> pixels.
func TestTextToPNGMatchesSpecMatrices(t *testing.T) {
	for _, tc := range pipelineFonts {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("spec", "glyphs-"+tc.spec+".yaml"))
			if err != nil {
				t.Fatal(err)
			}
			var spec struct {
				Glyphs map[string][]string `yaml:"glyphs"`
			}
			if err := yaml.Unmarshal(raw, &spec); err != nil {
				t.Fatal(err)
			}
			if len(spec.Glyphs) == 0 {
				t.Fatal("spec has no glyphs")
			}
			f := tc.font()
			for key, rows := range spec.Glyphs {
				img := renderTextPNG(t, f, key)
				got := cell(img, 0, tc.width, tc.height)
				// The spec matrix is padded to the cell width; trim to compare.
				for y := range rows {
					want := strings.ReplaceAll(rows[y], "0", " ")
					want = want + strings.Repeat(" ", tc.width-len(want))
					if got[y] != want {
						t.Fatalf("glyph %q row %d = %q, want %q", key, y, got[y], want)
					}
				}
			}
		})
	}
}

// TestTextToPNGUpstreamCoverage checks that real OSS-font glyphs reach the PNG,
// are distinct from the fallback, and that glyphs in neither source fall back to '?'.
func TestTextToPNGUpstreamCoverage(t *testing.T) {
	for _, tc := range pipelineFonts {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.font()
			text := "Hi!"
			img := renderTextPNG(t, f, text)
			for i, r := range []rune(text) {
				c := cell(img, i, tc.width, tc.height)
				if !hasInk(c) {
					t.Errorf("%q rendered blank", r)
				}
			}
			qm := cell(renderTextPNG(t, f, "?"), 0, tc.width, tc.height)
			for i, r := range []rune("Hi") {
				if joinRows(cell(img, i, tc.width, tc.height)) == joinRows(qm) {
					t.Errorf("%q rendered as the fallback glyph", r)
				}
			}
			// U+E000 is in no font and no spec: must draw the '?' fallback.
			missing := cell(renderTextPNG(t, f, ""), 0, tc.width, tc.height)
			if joinRows(missing) != joinRows(qm) {
				t.Errorf("unmapped rune did not fall back to '?':\n%s\nwant\n%s", joinRows(missing), joinRows(qm))
			}
			// Space stays blank.
			if hasInk(cell(renderTextPNG(t, f, " "), 0, tc.width, tc.height)) {
				t.Error("space has ink")
			}
		})
	}
}

// TestTextToPNGUpstreamBitmapsMatchImage compares production-parsed upstream
// bitmaps to the decoded PNG for the whole printable ASCII range.
func TestTextToPNGUpstreamBitmapsMatchImage(t *testing.T) {
	for _, tc := range pipelineFonts {
		if !tc.upstream {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			bitmaps := upstreamBitmaps(tc.spec, tc.width, tc.height)
			var ascii []rune
			for r := rune('!'); r <= '~'; r++ {
				if _, ok := bitmaps[r]; ok {
					ascii = append(ascii, r)
				}
			}
			if len(ascii) < 90 {
				t.Fatalf("only %d upstream ASCII glyphs; BDF not loaded?", len(ascii))
			}
			img := renderTextPNG(t, tc.font(), string(ascii))
			bpr := (tc.width + 7) / 8
			for i, r := range ascii {
				got := cell(img, i, tc.width, tc.height)
				for y := 0; y < tc.height; y++ {
					for x := 0; x < tc.width; x++ {
						bit := bitmaps[r][y*bpr+x/8]&(1<<(7-x%8)) != 0
						if bit != (got[y][x] == '1') {
							t.Fatalf("%q pixel (%d,%d): bitmap=%v png=%q", r, x, y, bit, got[y])
						}
					}
				}
			}
		})
	}
}

// TestTextToPNGBrailleIsProcedural verifies braille needs no spec entry.
func TestTextToPNGBrailleIsProcedural(t *testing.T) {
	for _, tc := range pipelineFonts {
		if _, ok := parseGlyphSpec(tc.spec).Glyphs["⣿"]; ok {
			t.Errorf("%s spec carries a braille matrix", tc.name)
		}
		if !hasInk(cell(renderTextPNG(t, tc.font(), "⣿"), 0, tc.width, tc.height)) {
			t.Errorf("%s: full braille cell rendered blank", tc.name)
		}
	}
}

// TestRenderFileToCardsPNGPerFont drives the real card pipeline (text lines ->
// PNG file on disk) for every font and checks the written file.
func TestRenderFileToCardsPNGPerFont(t *testing.T) {
	lines := []string{"func main() {", "\tprintln(\"héllo ✓ →\")", "}"}
	sizes := map[string][]byte{}
	for _, tc := range pipelineFonts {
		t.Run(tc.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "card.png")
			res, err := RenderFileToCards(lines, "main.go", RenderOptions{
				FontName: tc.name, Theme: "dark", OutputPath: out, Title: "main.go",
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Files) != 1 {
				t.Fatalf("files = %v", res.Files)
			}
			raw, err := os.ReadFile(res.Files[0])
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(raw))
			if err != nil {
				t.Fatalf("output is not a valid PNG: %v", err)
			}
			if img.Bounds().Dx() != res.Width || img.Bounds().Dy() != res.Height {
				t.Errorf("PNG is %v, result says %dx%d", img.Bounds(), res.Width, res.Height)
			}
			bg := DarkTheme.Bg
			var ink int
			for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
				for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					if uint8(r>>8) != bg.R || uint8(g>>8) != bg.G || uint8(b>>8) != bg.B {
						ink++
					}
				}
			}
			if ink < 100 {
				t.Errorf("card has only %d non-background pixels", ink)
			}
			sizes[tc.name] = raw
		})
	}
	for a := range sizes {
		for b := range sizes {
			if a < b && bytes.Equal(sizes[a], sizes[b]) {
				t.Errorf("fonts %s and %s produced identical cards", a, b)
			}
		}
	}
}
