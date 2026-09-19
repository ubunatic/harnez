package readcard

import (
	"bufio"
	"embed"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// The BDFs are embedded here because go:embed cannot cross into third_party.
// They are parsed on first use of each non-default font.
//
//go:embed upstream/*.bdf
var upstreamFonts embed.FS

type bdfBitmap struct {
	code, width, height, xoff, yoff int
	rows                            []string
}
type bdfFont struct {
	glyphs       map[rune]bdfBitmap
	ascent, xoff int
}

var bdfCache sync.Map

func upstreamBitmaps(size string, width, height int) map[rune][]byte {
	if v, ok := bdfCache.Load(size); ok {
		return v.(map[rune][]byte)
	}
	paths := map[string]string{"3x5": "upstream/tom-thumb.bdf", "6x12": "upstream/spleen-6x12.bdf", "7x13": "upstream/7x13.bdf", "8x16": "upstream/spleen-8x16.bdf"}
	path, ok := paths[size]
	if !ok {
		return nil
	}
	b, err := upstreamFonts.Open(path)
	if err != nil {
		panic(fmt.Sprintf("readcard BDF %s: %v", size, err))
	}
	defer b.Close()
	font, err := parseEmbeddedBDF(b)
	if err != nil {
		panic(fmt.Sprintf("readcard BDF %s: %v", size, err))
	}
	result := make(map[rune][]byte, len(font.glyphs))
	for r, g := range font.glyphs {
		result[r] = embeddedMatrix(g, font, width, height)
	}
	actual, _ := bdfCache.LoadOrStore(size, result)
	return actual.(map[rune][]byte)
}

func parseEmbeddedBDF(s interface{ Read([]byte) (int, error) }) (bdfFont, error) {
	sc := bufio.NewScanner(s)
	font := bdfFont{glyphs: map[rune]bdfBitmap{}}
	var g *bdfBitmap
	bitmap := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "FONTBOUNDINGBOX "):
			f := strings.Fields(line)
			if len(f) != 5 {
				return font, fmt.Errorf("invalid FONTBOUNDINGBOX")
			}
			h, _ := strconv.Atoi(f[2])
			y, _ := strconv.Atoi(f[4])
			font.ascent = h + y
			font.xoff, _ = strconv.Atoi(f[3])
		case strings.HasPrefix(line, "STARTCHAR "):
			g = &bdfBitmap{code: -1}
		case strings.HasPrefix(line, "ENCODING ") && g != nil:
			f := strings.Fields(line)
			if len(f) < 2 {
				return font, fmt.Errorf("invalid ENCODING")
			}
			var e error
			g.code, e = strconv.Atoi(f[1])
			if e != nil {
				return font, e
			}
		case strings.HasPrefix(line, "BBX ") && g != nil:
			f := strings.Fields(line)
			if len(f) != 5 {
				return font, fmt.Errorf("invalid BBX")
			}
			g.width, _ = strconv.Atoi(f[1])
			g.height, _ = strconv.Atoi(f[2])
			g.xoff, _ = strconv.Atoi(f[3])
			g.yoff, _ = strconv.Atoi(f[4])
		case line == "BITMAP":
			bitmap = true
		case line == "ENDCHAR" && g != nil:
			if g.code >= 0 {
				font.glyphs[rune(g.code)] = *g
			}
			g = nil
			bitmap = false
		case bitmap && g != nil:
			g.rows = append(g.rows, line)
		}
	}
	if err := sc.Err(); err != nil {
		return font, err
	}
	return font, nil
}

func embeddedMatrix(g bdfBitmap, font bdfFont, width, height int) []byte {
	rows := make([]byte, height)
	top := font.ascent - (g.yoff + g.height)
	for y, encoded := range g.rows {
		bits, err := strconv.ParseUint(encoded, 16, 64)
		if err != nil {
			panic(err)
		}
		for x := 0; x < g.width && x < width; x++ {
			shift := len(encoded)*4 - 1 - x
			if shift >= 0 && bits&(1<<uint(shift)) != 0 {
				dx := g.xoff - font.xoff + x
				dy := top + y
				if dx >= 0 && dx < width && dy >= 0 && dy < height {
					rows[dy] |= 1 << uint(7-dx)
				}
			}
		}
	}
	return rows
}
