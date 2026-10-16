package tui

import (
	"strings"
	"unicode"

	"ubunatic.com/loom"
)

// Chat content (assistant text, tool and job output, errors) comes from
// models and tools, so it is untrusted terminal input. wrapANSI keeps only
// SGR sequences (colors and text attributes, ESC [ <digits;> m) and drops
// every other control: CSI cursor movement and erase, OSC (titles, OSC 8
// links, OSC 52 clipboard), DCS/APC/PM/SOS strings, other ESC sequences, C1
// controls, CR and the remaining C0 controls. Tabs expand to 8-column stops.

const sgrReset = "\x1b[0m"

// ansiToken is one piece of sanitized content: an SGR sequence, a newline,
// a tab, or a printable grapheme cluster of display width w.
type ansiToken struct {
	kind byte // 'm' SGR, '\n', '\t', 'c' cluster
	text string
	w    int
}

// wrapANSI sanitizes content and wraps it to width display columns. Styles
// carry across content and wrap line breaks: a styled line ends with a reset
// and the next line reopens the active style. Plain text comes back unchanged
// apart from tab expansion.
func wrapANSI(content string, width int) []string {
	var lines []string
	var cur strings.Builder
	var style loom.Style
	col := 0
	styled := func() bool { return style != loom.Style{} }
	flush := func() {
		if styled() {
			cur.WriteString(sgrReset)
		}
		lines = append(lines, cur.String())
		cur.Reset()
		col = 0
		if styled() {
			cur.WriteString(style.ANSI())
		}
	}
	put := func(text string, w int) {
		if width > 0 && col > 0 && col+w > width {
			flush()
		}
		cur.WriteString(text)
		col += w
	}
	for _, t := range ansiTokens(content) {
		switch t.kind {
		case 'm':
			cur.WriteString(t.text)
			style = applySGR(style, t.text)
		case '\n':
			flush()
		case '\t':
			for n := 8 - col%8; n > 0; n-- {
				put(" ", 1)
			}
		default:
			put(t.text, t.w)
		}
	}
	flush()
	return lines
}

// sanitizeLines applies the wrapANSI policy to log lines without wrapping;
// styles carry from line to line.
func sanitizeLines(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	return wrapANSI(strings.Join(lines, "\n"), 0)
}

// applySGR returns style after the SGR sequence seq, using loom's own parser
// so the tracked state matches what the canvas draws.
func applySGR(style loom.Style, seq string) loom.Style {
	cells := loom.ParseANSI(style.ANSI() + seq + " ")
	if len(cells) == 0 {
		return style
	}
	return cells[len(cells)-1].Style
}

// ansiTokens splits s into sanitized tokens (see wrapANSI for the policy).
func ansiTokens(s string) []ansiToken {
	rs := []rune(s)
	n := len(rs)
	var out []ansiToken
	for i := 0; i < n; {
		r := rs[i]
		switch {
		case r == 0x1b:
			var tok string
			i, tok = skipEscape(rs, i)
			if tok != "" {
				out = append(out, ansiToken{kind: 'm', text: tok})
			}
		case r == 0x9b: // C1 CSI: dropped, even when it is SGR
			i, _ = skipCSI(rs, i+1, i)
		case r == 0x90 || r == 0x9d || r == 0x98 || r == 0x9e || r == 0x9f: // C1 DCS, OSC, SOS, PM, APC
			i = skipString(rs, i+1)
		case r == '\n':
			out = append(out, ansiToken{kind: '\n'})
			i++
		case r == '\t':
			out = append(out, ansiToken{kind: '\t'})
			i++
		case unicode.IsControl(r):
			i++
		default:
			j := clusterEnd(rs, i)
			c := string(rs[i:j])
			out = append(out, ansiToken{kind: 'c', text: c, w: loom.StringWidth(c)})
			i = j
		}
	}
	return out
}

// skipEscape consumes the escape sequence at rs[i] == ESC and returns the
// next index plus the sequence if it is a kept SGR.
func skipEscape(rs []rune, i int) (int, string) {
	if i+1 >= len(rs) {
		return i + 1, ""
	}
	switch next := rs[i+1]; {
	case next == '[':
		return skipCSI(rs, i+2, i)
	case next == ']' || next == 'P' || next == 'X' || next == '^' || next == '_':
		return skipString(rs, i+2), ""
	case next >= 0x20 && next <= 0x2f: // nF: intermediates, then one final byte
		j := i + 1
		for j < len(rs) && rs[j] >= 0x20 && rs[j] <= 0x2f {
			j++
		}
		if j < len(rs) && rs[j] >= 0x30 && rs[j] <= 0x7e {
			j++
		}
		return j, ""
	case next >= 0x30 && next <= 0x7e: // Fp, Fe, Fs: ESC 7, ESC c, ESC M, ...
		return i + 2, ""
	}
	return i + 1, "" // a lone ESC; the next rune is handled on its own
}

// skipCSI consumes a CSI body starting at rs[j] (after ESC [ or C1 CSI at
// start). It returns the sequence when it is a plain SGR introduced by ESC [.
// A malformed body ends at the first byte outside the CSI ranges, which is
// then handled on its own.
func skipCSI(rs []rune, j, start int) (int, string) {
	params := j
	for j < len(rs) && rs[j] >= 0x30 && rs[j] <= 0x3f {
		j++
	}
	mid := j
	for j < len(rs) && rs[j] >= 0x20 && rs[j] <= 0x2f {
		j++
	}
	if j >= len(rs) || rs[j] < 0x40 || rs[j] > 0x7e {
		return j, ""
	}
	if rs[j] == 'm' && mid == j && rs[start] == 0x1b && sgrParams(rs[params:mid]) {
		return j + 1, string(rs[start : j+1])
	}
	return j + 1, ""
}

// sgrParams reports whether p holds only digits and ';' (no private markers
// or ':' sub-parameters, which loom does not parse).
func sgrParams(p []rune) bool {
	for _, r := range p {
		if (r < '0' || r > '9') && r != ';' {
			return false
		}
	}
	return true
}

// skipString consumes a control string body (OSC, DCS, SOS, PM, APC) up to
// and including its terminator: BEL, ESC \ or C1 ST. An unterminated string
// is dropped up to the end of the line, so it cannot swallow later lines.
func skipString(rs []rune, j int) int {
	for ; j < len(rs); j++ {
		switch rs[j] {
		case 0x07, 0x9c:
			return j + 1
		case '\n':
			return j
		case 0x1b:
			if j+1 < len(rs) && rs[j+1] == '\\' {
				return j + 2
			}
		}
	}
	return j
}

// clusterEnd returns the end of the grapheme cluster at rs[i], grouped the
// way loom's ANSI parser groups cells: regional-indicator pairs, combining
// marks, variation selectors and ZWJ sequences.
func clusterEnd(rs []rune, i int) int {
	n := len(rs)
	j := i + 1
	if isRegional(rs[i]) && j < n && isRegional(rs[j]) {
		return j + 1
	}
	for j < n {
		next := rs[j]
		if next == 0x200D {
			j++
			// Only a printable rune joins after ZWJ: a control (ESC, C1,
			// newline, ...) ends the cluster so the filter still sees it.
			if j < n && !unicode.IsControl(rs[j]) {
				j++
			}
			continue
		}
		if next == 0xFE0F || next == 0xFE0E || unicode.Is(unicode.Mn, next) || unicode.Is(unicode.Me, next) {
			j++
			continue
		}
		break
	}
	return j
}

func isRegional(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }
