package readcard

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"strings"
)

// Card style axes. Each is a --flag=<mode> of `harnez read -I`; the zero value
// of every axis keeps the original card look.
const (
	ChromeFull = "full" // 36px header bar, generous padding
	ChromeSlim = "slim" // 14px title strip, tight padding
	ChromeNone = "none" // no header: title moves into the meta box (slim strip if it does not fit)

	GutterNormal = "normal" // line numbers in the code font
	GutterTight  = "tight"  // line numbers in the 3x5 micro font, centered
	GutterSup    = "sup"    // micro font, top-aligned like a superscript

	FrameOff = "off"
	FrameSep = "sep" // separator line above each section
	FrameBox = "box" // box around each section

	MetaOff = "off"
	MetaBox = "box" // red dotted info box in free space at the top right
)

// StylePresets bundle axes for one-flag selection; explicit axis flags override them.
var StylePresets = map[string]RenderOptions{
	"default": {},
	"compact": {Chrome: ChromeSlim, Gutter: GutterTight, Frame: FrameSep, Meta: MetaBox},
	"max":     {Chrome: ChromeNone, Gutter: GutterSup, Frame: FrameBox, Meta: MetaBox},
}

// ApplyStylePreset fills unset style axes of o from the named preset.
func ApplyStylePreset(o *RenderOptions, name string) error {
	p, ok := StylePresets[name]
	if !ok {
		return fmt.Errorf("unknown style %q (expected default, compact or max)", name)
	}
	if o.Chrome == "" {
		o.Chrome = p.Chrome
	}
	if o.Gutter == "" {
		o.Gutter = p.Gutter
	}
	if o.Frame == "" {
		o.Frame = p.Frame
	}
	if o.Meta == "" {
		o.Meta = p.Meta
	}
	return nil
}

// validateStyle checks the style axes; empty means the original look.
func validateStyle(o RenderOptions) error {
	for _, c := range []struct {
		axis, v string
		ok      []string
	}{
		{"chrome", o.Chrome, []string{ChromeFull, ChromeSlim, ChromeNone}},
		{"gutter", o.Gutter, []string{GutterNormal, GutterTight, GutterSup}},
		{"frame", o.Frame, []string{FrameOff, FrameSep, FrameBox}},
		{"meta", o.Meta, []string{MetaOff, MetaBox}},
	} {
		if c.v == "" {
			continue
		}
		found := false
		for _, k := range c.ok {
			found = found || k == c.v
		}
		if !found {
			return fmt.Errorf("unknown --%s %q (expected %s)", c.axis, c.v, strings.Join(c.ok, ", "))
		}
	}
	return nil
}

var metaRed = color.RGBA{R: 0xef, G: 0x44, B: 0x44, A: 0xff}

// drawDottedRect strokes a rectangle with a 2px-on, 2px-off dotted line.
func drawDottedRect(img *image.RGBA, x0, y0, x1, y1 int, col color.RGBA) {
	b := img.Bounds()
	dot := func(x, y, i int) {
		if i/2%2 == 0 && x >= b.Min.X && x < b.Max.X && y >= b.Min.Y && y < b.Max.Y {
			img.SetRGBA(x, y, col)
		}
	}
	for x := x0; x <= x1; x++ {
		dot(x, y0, x-x0)
		dot(x, y1, x-x0)
	}
	for y := y0; y <= y1; y++ {
		dot(x0, y, y-y0)
		dot(x1, y, y-y0)
	}
}

// sectionStarts marks the source lines that open a section: diff headers and
// hunks, Markdown headings, Go top-level declarations and bundle file markers.
func sectionStarts(lines []string, filename string) []bool {
	ext := strings.ToLower(filepath.Ext(filename))
	out := make([]bool, len(lines))
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "diff --git "), strings.HasPrefix(l, "@@ "), strings.HasPrefix(l, "=== "):
			out[i] = true
		case ext == ".md" && (strings.HasPrefix(l, "# ") || strings.HasPrefix(l, "## ")):
			out[i] = true
		case ext == ".go" && (strings.HasPrefix(l, "func ") || strings.HasPrefix(l, "type ")):
			out[i] = true
		}
	}
	return out
}

// metaLines is the info shown in the meta box: what the header bar would say
// plus the text-token cost the card replaces.
func metaLines(title string, chromeNone bool, first, last, total, textTokens int) []string {
	var out []string
	if chromeNone {
		out = append(out, title)
	}
	out = append(out, fmt.Sprintf("L%d-%d of %d", first, last, total))
	if textTokens > 0 {
		out = append(out, fmt.Sprintf("~%d text tok", textTokens))
	}
	return out
}

// drawFrames separates the sections of one column: a line above each section
// start (sep) or a box around each section run (box). x0..x1 is the code area.
func drawFrames(img *image.RGBA, rows []renderRow, mode string, x0, x1, colY, lineHeight int, theme ColorTheme) {
	if mode != FrameSep && mode != FrameBox {
		return
	}
	start := 0
	flush := func(end int) { // rows[start:end] form one section run
		if mode == FrameBox && end > start {
			drawStrokeRect(img, x0, colY+start*lineHeight, x1, colY+end*lineHeight-1, theme.Border)
		}
	}
	for i, r := range rows {
		if !r.section {
			continue
		}
		if i > start || i > 0 {
			flush(i)
			if mode == FrameSep && i > 0 {
				drawHorizontalLine(img, x0, x1, colY+i*lineHeight, theme.Border)
			}
		}
		start = i
	}
	flush(len(rows))
}

// metaPlan is the placement of the meta box for one page.
type metaPlan struct {
	lines []string
	x, w  int
	h     int
}

// planMeta places the meta box at the top right of the last column when the
// rows it covers leave enough free width; it returns nil when they do not.
// lastColX is the left pixel of the last column, rows its first rows.
func planMeta(lines []string, lastColRows []renderRow, lastColX, gutterWidth, cw, lineHeight, rightEdge int) *metaPlan {
	textChars := 0
	for _, l := range lines {
		textChars = max(textChars, len([]rune(l)))
	}
	w := (textChars + 2) * cw
	h := len(lines)*lineHeight + 2
	x := rightEdge - w
	for i := 0; i < min(len(lines)+1, len(lastColRows)); i++ {
		used := 0
		for _, t := range lastColRows[i].tokens {
			used += len([]rune(t.Text))
		}
		if lastColX+gutterWidth+used*cw+6 > x {
			return nil
		}
	}
	return &metaPlan{lines: lines, x: x, w: w, h: h}
}

func (m *metaPlan) draw(img *image.RGBA, font *MonospaceFont, theme ColorTheme, y int) {
	drawRect(img, m.x, y, m.w, m.h, theme.Bg)
	drawDottedRect(img, m.x, y, m.x+m.w-1, y+m.h-1, metaRed)
	for i, l := range m.lines {
		font.DrawString(img, l, m.x+font.CharWidth, y+2+i*(font.CharHeight+2), theme.BadgeFg)
	}
}
