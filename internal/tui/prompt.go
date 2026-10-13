package tui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// PromptBuffer manages the multiline text input state, cursor position, and history.
type PromptBuffer struct {
	lines     []string
	cursorRow int
	cursorCol int
	history   []string
	histIdx   int
	draft     string
	histSize  int
	minLines  int
	maxLines  int
	symbol    string
}

// NewPromptBuffer creates a fresh PromptBuffer with specified bounds and prompt symbol.
func NewPromptBuffer(minLines, maxLines, histSize int, symbol string) *PromptBuffer {
	if minLines < 1 {
		minLines = 1
	}
	if maxLines < minLines {
		maxLines = minLines
	}
	if symbol == "" {
		symbol = "> "
	}
	return &PromptBuffer{
		lines:    []string{""},
		histIdx:  -1,
		histSize: histSize,
		minLines: minLines,
		maxLines: maxLines,
		symbol:   symbol,
	}
}

// Text returns the full prompt string joined by newlines.
func (pb *PromptBuffer) Text() string {
	return strings.Join(pb.lines, "\n")
}

// SetText sets the prompt text and moves cursor to the end.
func (pb *PromptBuffer) SetText(s string) {
	pb.lines = strings.Split(s, "\n")
	if len(pb.lines) == 0 {
		pb.lines = []string{""}
	}
	pb.cursorRow = len(pb.lines) - 1
	pb.cursorCol = len([]rune(pb.lines[pb.cursorRow]))
}

// Clear resets the buffer to empty state.
func (pb *PromptBuffer) Clear() {
	pb.lines = []string{""}
	pb.cursorRow = 0
	pb.cursorCol = 0
	pb.histIdx = -1
	pb.draft = ""
}

// InsertChar inserts a rune at the current cursor position.
func (pb *PromptBuffer) InsertChar(r rune) {
	line := []rune(pb.lines[pb.cursorRow])
	col := pb.cursorCol
	if col > len(line) {
		col = len(line)
	}
	newLine := append(line[:col], append([]rune{r}, line[col:]...)...)
	pb.lines[pb.cursorRow] = string(newLine)
	pb.cursorCol = col + 1
}

// InsertNewline splits the line at the current cursor position.
func (pb *PromptBuffer) InsertNewline() {
	line := []rune(pb.lines[pb.cursorRow])
	col := pb.cursorCol
	if col > len(line) {
		col = len(line)
	}
	head := string(line[:col])
	tail := string(line[col:])

	pb.lines[pb.cursorRow] = head
	pb.lines = append(pb.lines[:pb.cursorRow+1], append([]string{tail}, pb.lines[pb.cursorRow+1:]...)...)
	pb.cursorRow++
	pb.cursorCol = 0
}

// Backspace deletes character before cursor or merges lines.
func (pb *PromptBuffer) Backspace() {
	if pb.cursorCol > 0 {
		line := []rune(pb.lines[pb.cursorRow])
		col := pb.cursorCol
		if col > len(line) {
			col = len(line)
		}
		newLine := append(line[:col-1], line[col:]...)
		pb.lines[pb.cursorRow] = string(newLine)
		pb.cursorCol = col - 1
		return
	}

	if pb.cursorRow > 0 {
		prevLine := []rune(pb.lines[pb.cursorRow-1])
		curLine := pb.lines[pb.cursorRow]
		newCol := len(prevLine)

		merged := string(prevLine) + curLine
		pb.lines[pb.cursorRow-1] = merged
		pb.lines = append(pb.lines[:pb.cursorRow], pb.lines[pb.cursorRow+1:]...)
		pb.cursorRow--
		pb.cursorCol = newCol
	}
}

// TrimContinuation removes a backslash directly before the cursor at the end
// of the last line and reports whether it did.
func (pb *PromptBuffer) TrimContinuation() bool {
	line := []rune(pb.lines[pb.cursorRow])
	if pb.cursorRow != len(pb.lines)-1 || pb.cursorCol != len(line) || pb.cursorCol == 0 || line[pb.cursorCol-1] != '\\' {
		return false
	}
	pb.lines[pb.cursorRow] = string(line[:pb.cursorCol-1])
	pb.cursorCol--
	return true
}

// MoveLeft moves cursor left one rune.
func (pb *PromptBuffer) MoveLeft() {
	if pb.cursorCol > 0 {
		pb.cursorCol--
		return
	}
	if pb.cursorRow > 0 {
		pb.cursorRow--
		pb.cursorCol = len([]rune(pb.lines[pb.cursorRow]))
	}
}

// MoveRight moves cursor right one rune.
func (pb *PromptBuffer) MoveRight() {
	lineLen := len([]rune(pb.lines[pb.cursorRow]))
	if pb.cursorCol < lineLen {
		pb.cursorCol++
		return
	}
	if pb.cursorRow < len(pb.lines)-1 {
		pb.cursorRow++
		pb.cursorCol = 0
	}
}

// MoveUp moves cursor up one line, or steps backward in history if at top line.
func (pb *PromptBuffer) MoveUp() bool {
	if pb.cursorRow > 0 {
		pb.cursorRow--
		lineLen := len([]rune(pb.lines[pb.cursorRow]))
		if pb.cursorCol > lineLen {
			pb.cursorCol = lineLen
		}
		return false
	}
	return pb.HistoryPrev()
}

// MoveDown moves cursor down one line, or steps forward in history if at bottom line.
func (pb *PromptBuffer) MoveDown() bool {
	if pb.cursorRow < len(pb.lines)-1 {
		pb.cursorRow++
		lineLen := len([]rune(pb.lines[pb.cursorRow]))
		if pb.cursorCol > lineLen {
			pb.cursorCol = lineLen
		}
		return false
	}
	return pb.HistoryNext()
}

// AddHistory adds non-empty text to history buffer.
func (pb *PromptBuffer) AddHistory(text string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return
	}
	if len(pb.history) > 0 && pb.history[len(pb.history)-1] == trimmed {
		return
	}
	pb.history = append(pb.history, trimmed)
	if pb.histSize > 0 && len(pb.history) > pb.histSize {
		pb.history = pb.history[len(pb.history)-pb.histSize:]
	}
	pb.histIdx = -1
}

// HistoryPrev navigates to earlier history item.
func (pb *PromptBuffer) HistoryPrev() bool {
	if len(pb.history) == 0 {
		return false
	}
	if pb.histIdx == -1 {
		pb.draft = pb.Text()
		pb.histIdx = len(pb.history) - 1
	} else if pb.histIdx > 0 {
		pb.histIdx--
	} else {
		return false
	}
	pb.SetText(pb.history[pb.histIdx])
	return true
}

// HistoryNext navigates to later history item.
func (pb *PromptBuffer) HistoryNext() bool {
	if pb.histIdx == -1 {
		return false
	}
	if pb.histIdx < len(pb.history)-1 {
		pb.histIdx++
		pb.SetText(pb.history[pb.histIdx])
	} else {
		pb.histIdx = -1
		pb.SetText(pb.draft)
	}
	return true
}

// Lines returns the rendered prompt box lines formatted for display.
// Long lines wrap at width; when the box exceeds maxLines it keeps the cursor row visible.
func (pb *PromptBuffer) Lines(width int) []string {
	if width < 10 {
		width = 10
	}
	indent := strings.Repeat(" ", runewidth.StringWidth(pb.symbol))
	var res []string
	cursorLine := 0
	for i, l := range pb.lines {
		prefix := pb.symbol
		if i > 0 {
			prefix = indent
		}
		runes := []rune(l)
		if i == pb.cursorRow {
			col := min(pb.cursorCol, len(runes))
			if col < len(runes) {
				l = string(runes[:col]) + "█" + string(runes[col+1:])
			} else {
				l += "█"
			}
		}
		wrapped := wrapLine(prefix+l, width)
		if i == pb.cursorRow {
			cursorLine = len(res) + len(wrapped) - 1
			for j, w := range wrapped {
				if strings.Contains(w, "█") {
					cursorLine = len(res) + j
				}
			}
		}
		res = append(res, wrapped...)
	}

	for len(res) < pb.minLines {
		res = append(res, indent)
	}
	if len(res) > pb.maxLines {
		start := max(0, min(cursorLine-pb.maxLines+1, len(res)-pb.maxLines))
		res = res[start : start+pb.maxLines]
	}
	return res
}
