package readcard

import (
	"image/color"
	"strings"
	"testing"
)

func TestParseANSILine(t *testing.T) {
	var state ansiState
	tokens := parseANSILine("\x1b[31mred\x1b[0m plain", &state)
	if len(tokens) != 2 || tokens[0].Text != "red" || tokens[1].Text != " plain" {
		t.Fatalf("unexpected tokens: %#v", tokens)
	}
	if tokens[0].FG == nil || tokens[0].FG.R == 0 {
		t.Fatalf("red style was not preserved: %#v", tokens[0])
	}
	if tokens[1].FG != nil {
		t.Fatalf("reset style leaked: %#v", tokens[1])
	}
	if strings.Contains(tokens[0].Text+tokens[1].Text, "\\x1b") {
		t.Fatal("escape sequence leaked into text")
	}
}

func TestParseANSIExtendedColors(t *testing.T) {
	var state ansiState
	tokens := parseANSILine("\x1b[38;5;196m256\x1b[48;2;1;2;3mtrue\x1b[0m", &state)
	if len(tokens) != 2 {
		t.Fatalf("tokens = %#v, want two spans", tokens)
	}
	if tokens[0].FG == nil || tokens[0].FG.R != 255 || tokens[0].FG.G != 0 || tokens[0].FG.B != 0 {
		t.Fatalf("256-color foreground = %#v", tokens[0].FG)
	}
	wantBG := color.RGBA{1, 2, 3, 255}
	if tokens[1].BG == nil || *tokens[1].BG != wantBG {
		t.Fatalf("truecolor background = %#v, want %#v", tokens[1].BG, wantBG)
	}
}

func TestStripANSIEscapesMalformed(t *testing.T) {
	if got := stripANSIEscapes("a\x1b[31b"); got != "a" {
		t.Fatalf("got %q", got)
	}
}
