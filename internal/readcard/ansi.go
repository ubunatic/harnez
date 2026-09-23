package readcard

import (
	"image/color"
	"strconv"
	"strings"
)

type ansiState struct{ fg, bg *color.RGBA }

func stripANSIEscapes(s string) string {
	if !strings.Contains(s, "\x1b") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && (s[i] < 0x40 || s[i] > 0x7e) {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// parseANSILine consumes SGR controls and preserves their colors on text spans.
// Incomplete or unsupported controls are discarded so they cannot corrupt layout.
func parseANSILine(line string, state *ansiState) []Token {
	if !strings.Contains(line, "\x1b") {
		return []Token{{Type: TokenText, Text: line, FG: state.fg, BG: state.bg}}
	}
	var out []Token
	start := 0
	for i := 0; i < len(line); {
		if line[i] != 0x1b || i+1 >= len(line) || line[i+1] != '[' {
			i++
			continue
		}
		if i > start {
			out = append(out, Token{Type: TokenText, Text: line[start:i], FG: state.fg, BG: state.bg})
		}
		j := i + 2
		for j < len(line) && (line[j] < 0x40 || line[j] > 0x7e) {
			j++
		}
		if j == len(line) {
			break
		}
		if line[j] == 'm' {
			applySGR(line[i+2:j], state)
		}
		i, start = j+1, j+1
	}
	if start < len(line) {
		out = append(out, Token{Type: TokenText, Text: line[start:], FG: state.fg, BG: state.bg})
	}
	return out
}

func applySGR(params string, s *ansiState) {
	if params == "" {
		params = "0"
	}
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			continue
		}
		switch {
		case n == 0:
			s.fg, s.bg = nil, nil
		case n == 39:
			s.fg = nil
		case n == 49:
			s.bg = nil
		case n >= 30 && n <= 37:
			c := ansiColor(n-30, false)
			s.fg = &c
		case n >= 40 && n <= 47:
			c := ansiColor(n-40, false)
			s.bg = &c
		case n >= 90 && n <= 97:
			c := ansiColor(n-90, true)
			s.fg = &c
		case n >= 100 && n <= 107:
			c := ansiColor(n-100, true)
			s.bg = &c
		case n == 38 || n == 48:
			if i+2 < len(parts) && parts[i+1] == "5" {
				if v, e := strconv.Atoi(parts[i+2]); e == nil {
					c := ansi256(v)
					if n == 38 {
						s.fg = &c
					} else {
						s.bg = &c
					}
					i += 2
				}
			} else if i+4 < len(parts) && parts[i+1] == "2" {
				r, er := strconv.Atoi(parts[i+2])
				g, eg := strconv.Atoi(parts[i+3])
				b, eb := strconv.Atoi(parts[i+4])
				if er == nil && eg == nil && eb == nil && r >= 0 && r <= 255 && g >= 0 && g <= 255 && b >= 0 && b <= 255 {
					c := color.RGBA{uint8(r), uint8(g), uint8(b), 255}
					if n == 38 {
						s.fg = &c
					} else {
						s.bg = &c
					}
					i += 4
				}
			}
		}
	}
}

func ansiColor(n int, bright bool) color.RGBA {
	base := []color.RGBA{{0, 0, 0, 255}, {205, 49, 49, 255}, {13, 188, 121, 255}, {229, 229, 16, 255}, {36, 114, 200, 255}, {188, 63, 188, 255}, {17, 168, 205, 255}, {229, 229, 229, 255}}
	c := base[n%8]
	if bright {
		c.R, c.G, c.B = uint8(min(255, int(c.R)+45)), uint8(min(255, int(c.G)+45)), uint8(min(255, int(c.B)+45))
	}
	return c
}

func ansi256(n int) color.RGBA {
	if n < 16 {
		return ansiColor(n%8, n >= 8)
	}
	if n >= 232 {
		v := uint8(8 + (n-232)*10)
		return color.RGBA{v, v, v, 255}
	}
	levels := []uint8{0, 95, 135, 175, 215, 255}
	n -= 16
	return color.RGBA{levels[n/36], levels[(n/6)%6], levels[n%6], 255}
}
