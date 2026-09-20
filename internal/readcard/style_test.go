package readcard

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func styleLines() []string {
	var lines []string
	for i := 0; i < 60; i++ {
		lines = append(lines, "a", "## h"+strings.Repeat("x", i%5))
	}
	lines[3] = strings.Repeat("w", 70)
	return lines
}

func renderStyled(t *testing.T, o RenderOptions) (*RenderResult, string) {
	t.Helper()
	o.OutputPath, o.ShowLineNumbers = filepath.Join(t.TempDir(), "c.png"), true
	res, err := RenderFileToCards(styleLines(), "doc.md", o)
	if err != nil {
		t.Fatal(err)
	}
	return res, res.Files[0]
}

func TestStylePresetsShrinkTheCard(t *testing.T) {
	def, _ := renderStyled(t, RenderOptions{})
	for _, name := range []string{"compact", "max"} {
		o := RenderOptions{}
		if err := ApplyStylePreset(&o, name); err != nil {
			t.Fatal(err)
		}
		res, _ := renderStyled(t, o)
		if res.Height >= def.Height {
			t.Errorf("%s height %d, want < default %d", name, res.Height, def.Height)
		}
		if res.TokenStats.ClaudeTokens >= def.TokenStats.ClaudeTokens {
			t.Errorf("%s claude tokens %d, want < default %d", name, res.TokenStats.ClaudeTokens, def.TokenStats.ClaudeTokens)
		}
	}
	if err := ApplyStylePreset(&RenderOptions{}, "nope"); err == nil {
		t.Error("unknown preset accepted")
	}
}

func TestExplicitAxisOverridesPreset(t *testing.T) {
	o := RenderOptions{Frame: FrameOff}
	if err := ApplyStylePreset(&o, "compact"); err != nil {
		t.Fatal(err)
	}
	if o.Frame != FrameOff || o.Chrome != ChromeSlim {
		t.Errorf("got %+v", o)
	}
}

func TestUnknownStyleModeIsRejected(t *testing.T) {
	_, err := RenderFileToCards([]string{"x"}, "a.go", RenderOptions{Gutter: "huge"})
	if err == nil || !strings.Contains(err.Error(), "--gutter") {
		t.Errorf("err = %v", err)
	}
}

func hasColor(t *testing.T, path string, want [3]uint8) bool {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == want[0] && uint8(g>>8) == want[1] && uint8(bl>>8) == want[2] {
				return true
			}
		}
	}
	return false
}

func TestMetaBoxIsRedDottedOnlyWhenRequested(t *testing.T) {
	red := [3]uint8{metaRed.R, metaRed.G, metaRed.B}
	_, off := renderStyled(t, RenderOptions{})
	_, on := renderStyled(t, RenderOptions{Meta: MetaBox})
	if hasColor(t, off, red) {
		t.Error("red pixels without --meta=box")
	}
	if !hasColor(t, on, red) {
		t.Error("no red meta box with --meta=box")
	}
}

func TestChromeNoneFallsBackToStripWhenNoRoomForMeta(t *testing.T) {
	long := []string{strings.Repeat("w", 200), strings.Repeat("w", 200), strings.Repeat("w", 200), strings.Repeat("w", 200)}
	out := filepath.Join(t.TempDir(), "c.png")
	res, err := RenderFileToCards(long, "a.txt", RenderOptions{Chrome: ChromeNone, Columns: 1, OutputPath: out})
	if err != nil {
		t.Fatal(err)
	}
	if hasColor(t, res.Files[0], [3]uint8{metaRed.R, metaRed.G, metaRed.B}) {
		t.Error("meta box drawn over full-width text")
	}
}

func TestSectionStarts(t *testing.T) {
	got := sectionStarts([]string{"diff --git a b", "@@ -1 +1 @@", "+x", "func f() {", "## H"}, "x.diff")
	want := []bool{true, true, false, false, false}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("diff line %d = %v", i, got[i])
		}
	}
	if g := sectionStarts([]string{"func f() {", "type T int", "var x"}, "a.go"); !g[0] || !g[1] || g[2] {
		t.Errorf("go sections = %v", g)
	}
}

func TestFrameDrawsBorderColorLines(t *testing.T) {
	border := [3]uint8{DarkTheme.Border.R, DarkTheme.Border.G, DarkTheme.Border.B}
	_, sep := renderStyled(t, RenderOptions{Chrome: ChromeSlim, Frame: FrameSep})
	if !hasColor(t, sep, border) {
		t.Error("no separator lines")
	}
	a, _ := renderStyled(t, RenderOptions{Frame: FrameBox})
	b, _ := renderStyled(t, RenderOptions{})
	if a.Height != b.Height {
		t.Errorf("frames must not change geometry: %d vs %d", a.Height, b.Height)
	}
}

func TestTightGutterNarrowsTheCard(t *testing.T) {
	def, _ := renderStyled(t, RenderOptions{})
	tight, _ := renderStyled(t, RenderOptions{Gutter: GutterTight})
	if tight.Width >= def.Width {
		t.Errorf("tight width %d, want < %d", tight.Width, def.Width)
	}
}
