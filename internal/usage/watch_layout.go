package usage

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"
	"unsafe"
)

func renderSparkline(values []float64) string {
	return RenderSparkline(values)
}

// stripANSI removes ANSI escape sequences so visible width can be measured.
func stripANSI(s string) string {
	var sb strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func visLen(s string) int {
	return utf8.RuneCountInString(stripANSI(s))
}

func truncateVisible(s string, maxWidth int) string {
	if maxWidth <= 3 {
		return "..."
	}
	if visLen(s) <= maxWidth {
		return s
	}
	var sb strings.Builder
	inEsc := false
	count := 0
	target := maxWidth - 3
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			sb.WriteRune(r)
			continue
		}
		if inEsc {
			sb.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		if count >= target {
			break
		}
		sb.WriteRune(r)
		count++
	}
	sb.WriteString("...\x1b[0m")
	return sb.String()
}

// wbox is a single bordered panel in the compact btop-style watch grid.
type wbox struct {
	title string
	lines []string
	width int
}

func renderWBox(b wbox) []string {
	var res []string

	titleVis := visLen(b.title) + 4
	rem := b.width - titleVis - 1
	if rem < 1 {
		rem = 1
	}
	res = append(res, "┌─ "+b.title+" "+strings.Repeat("─", rem)+"┐")

	contentW := b.width - 4
	if contentW < 10 {
		contentW = 10
	}
	for _, l := range b.lines {
		if visLen(l) > contentW {
			l = truncateVisible(l, contentW)
		}
		pad := contentW - visLen(l)
		if pad < 0 {
			pad = 0
		}
		res = append(res, "│ "+l+strings.Repeat(" ", pad)+" │")
	}

	res = append(res, "└"+strings.Repeat("─", b.width-2)+"┘")
	return res
}

// combineRow lays rendered boxes out side by side, padding shorter boxes to
// the tallest one in the row.
func combineRow(cols [][]string) []string {
	maxLines := 0
	widths := make([]int, len(cols))
	for i, c := range cols {
		if len(c) > maxLines {
			maxLines = len(c)
		}
		if len(c) > 0 {
			widths[i] = visLen(c[0])
		}
	}
	res := make([]string, maxLines)
	for i := 0; i < maxLines; i++ {
		var parts []string
		for ci, c := range cols {
			if i < len(c) {
				parts = append(parts, c[i])
			} else {
				parts = append(parts, strings.Repeat(" ", widths[ci]))
			}
		}
		res[i] = strings.Join(parts, strings.Repeat(" ", boxGap))
	}
	return res
}

// Floors on the geometry we lay panels out for. Below these, box borders and
// truncated content stop being legible, so we clamp rather than trust a
// nonsensical reading.
const minTerminalWidth = 30

const minTerminalHeight = 6

// Fallbacks used only when every real size probe fails (no tty at all).
const defaultTerminalWidth = 90

const defaultTerminalHeight = 24

// ttySize queries f's window size directly via TIOCGWINSZ. This is more
// reliable than shelling out to `stty -F /dev/tty size`: it reads the actual
// fd we're writing to instead of a separate /dev/tty open, which can report a
// stale or unrelated size (or fail outright) under some pty wrappers.
func ttySize(f *os.File) (cols, rows int, ok bool) {
	var ws struct{ Row, Col, Xpixel, Ypixel uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws)))
	if errno != 0 || ws.Col == 0 || ws.Row == 0 {
		return 0, 0, false
	}
	return int(ws.Col), int(ws.Row), true
}

// terminalSize resolves the usable geometry for the watch grid, trying out's
// own fd first (most reliable), then os.Stdout, then a /dev/tty stty query,
// then $COLUMNS/$LINES, before giving up on fixed defaults.
//
// Rows matter as much as columns: the redraw loop must know how tall the
// viewport is so a frame can never overflow it. An overflowing frame scrolls
// the terminal, and once it has scrolled, the `\x1b[H` that starts the next
// redraw no longer points at the row the program thinks it does.
func terminalSize(out io.Writer) (cols, rows int) {
	clamp := func(c, r int) (int, int) {
		if c < minTerminalWidth {
			c = minTerminalWidth
		}
		if r < minTerminalHeight {
			r = minTerminalHeight
		}
		return c, r
	}

	if f, ok := out.(*os.File); ok {
		if c, r, ok := ttySize(f); ok {
			return clamp(c, r)
		}
	}
	if c, r, ok := ttySize(os.Stdout); ok {
		return clamp(c, r)
	}

	if raw, err := exec.Command("stty", "-F", "/dev/tty", "size").Output(); err == nil {
		parts := strings.Fields(string(raw))
		if len(parts) >= 2 {
			r, errR := strconv.Atoi(parts[0])
			c, errC := strconv.Atoi(parts[1])
			if errR == nil && errC == nil && c > 0 && r > 0 {
				return clamp(c, r)
			}
		}
	}

	cols, rows = defaultTerminalWidth, defaultTerminalHeight
	if v, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && v > 0 {
		cols = v
	}
	if v, err := strconv.Atoi(os.Getenv("LINES")); err == nil && v > 0 {
		rows = v
	}
	return clamp(cols, rows)
}

// screenFrame is one fully laid-out frame: lines already truncated to the
// terminal's width and count already capped to its height. Because it can
// never exceed the viewport, painting it is position-independent and cannot
// scroll the terminal.
type screenFrame struct {
	lines []string
	cols  int
	rows  int
}

// paint writes the frame at the top-left of the (alternate) screen.
//
// Every line is followed by an explicit erase-to-end-of-line. That is the part
// the previous implementation missed: `\x1b[H` + content + `\x1b[J` only clears
// *below* the new content, so any line shorter than the one it replaced kept
// the old line's tail, and blank lines (a bare "\n") cleared nothing at all.
// Shrinking the frame — e.g. toggling three panels down to one — therefore left
// debris from the larger frame interleaved with the new box.
//
// No newline is emitted after the final line, so even a frame exactly as tall
// as the terminal cannot push the viewport down by one row.
func (f screenFrame) paint(out io.Writer) error {
	var buf bytes.Buffer
	buf.WriteString("\x1b[H")
	for i, line := range f.lines {
		if i > 0 {
			buf.WriteString("\r\n")
		}
		buf.WriteString(line)
		buf.WriteString("\x1b[0m\x1b[K")
	}
	buf.WriteString("\x1b[J")
	_, err := out.Write(buf.Bytes())
	return err
}

// fit truncates lines to cols and caps them at rows, so the frame provably fits.
func fit(lines []string, cols, rows int) screenFrame {
	if len(lines) > rows {
		lines = lines[:rows]
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if visLen(l) > cols {
			l = truncateVisible(l, cols)
		}
		out[i] = l
	}
	return screenFrame{lines: out, cols: cols, rows: rows}
}
