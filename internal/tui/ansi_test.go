package tui

import (
	"regexp"
	"strings"
	"testing"

	"ubunatic.com/loom"
)

var sgrRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func visible(s string) string { return sgrRe.ReplaceAllString(s, "") }

func styleOf(seq string) loom.Style { return loom.ParseANSI(seq + "x")[0].Style }

func TestWrapANSIPlainTextUnchanged(t *testing.T) {
	for _, s := range []string{"", "hello world", "a\n\nb"} {
		got := strings.Join(wrapANSI(s, 80), "\n")
		if got != s {
			t.Errorf("wrapANSI(%q) = %q", s, got)
		}
	}
	got := wrapANSI(strings.Repeat("ab", 50), 80)
	if len(got) != 2 || got[0] != strings.Repeat("ab", 40) || got[1] != strings.Repeat("ab", 10) {
		t.Errorf("plain wrap = %q", got)
	}
	if got := wrapANSI("a\tb", 80); got[0] != "a       b" {
		t.Errorf("tab = %q", got[0])
	}
}

func TestWrapANSIKeepsStyleAcrossBreaks(t *testing.T) {
	red := styleOf("\x1b[1;31m")
	in := "\x1b[1;31m" + strings.Repeat("r", 100) + "\nnext\x1b[0m plain"
	got := wrapANSI(in, 80)
	if len(got) != 3 {
		t.Fatalf("lines = %q", got)
	}
	for i, l := range got {
		if w := loom.StringWidth(visible(l)); w > 80 {
			t.Errorf("line %d width %d", i, w)
		}
	}
	for i, l := range got[:2] {
		if !strings.HasSuffix(l, sgrReset) {
			t.Errorf("styled line %d does not end with a reset: %q", i, l)
		}
	}
	// Wrapped and newline-continued lines reopen the style.
	for i, l := range got[1:] {
		if c := loom.ParseANSI(l); c[0].Style != red {
			t.Errorf("line %d starts with style %+v; want %+v", i+1, c[0].Style, red)
		}
	}
	last := loom.ParseANSI(got[2])
	if last[len(last)-1].Style != (loom.Style{}) {
		t.Errorf("text after reset is styled: %+v", last[len(last)-1].Style)
	}
}

func TestWrapANSIStripsControls(t *testing.T) {
	in := "a\x1b[2J\x1b[H\x1b[5A\x1b[?25l\x1b[1;1r" + // CSI erase, cursor, private mode, margins
		"b\x1b]0;title\x07" + // OSC title
		"\x1b]8;;https://evil.example\x1b\\link\x1b]8;;\x1b\\" + // OSC 8 hyperlink
		"\x1b]52;c;ZXZpbA==\x07" + // OSC 52 clipboard
		"\x1bP+q544e\x1b\\\x1b_apc\x1b\\" + // DCS, APC
		"\x1bc\x1b7\x1b(B\x1b=" + // RIS, DECSC, charset, keypad
		"\u009b2J\u009d0;t\u009c" + // C1 CSI and OSC (as UTF-8 runes)
		"\x1b[38:2::1:2:3m\x1b[>4;2m" + // colon SGR and private SGR-like
		"c\rd\x07e\x00f\x1b"
	got := wrapANSI(in, 80)
	if len(got) != 1 || got[0] != "ablinkcdef" {
		t.Fatalf("wrapANSI = %q; want %q", got, "ablinkcdef")
	}
	// An unterminated OSC drops only the rest of its line.
	got = wrapANSI("x\x1b]52;c;AAAA\nnext", 80)
	if strings.Join(got, "|") != "x|next" {
		t.Errorf("unterminated OSC = %q", got)
	}
	// SGR survives next to stripped sequences.
	got = wrapANSI("\x1b[2K\x1b[32mok\x1b[0m", 80)
	if got[0] != "\x1b[32mok\x1b[0m" {
		t.Errorf("SGR kept = %q", got[0])
	}
}

func TestWrapANSIWideChars(t *testing.T) {
	in := "\x1b[34m" + strings.Repeat("你", 45) + "x"
	got := wrapANSI(in, 80)
	if len(got) != 2 {
		t.Fatalf("lines = %q", got)
	}
	if w := loom.StringWidth(visible(got[0])); w != 80 {
		t.Errorf("first line width %d; want 80", w)
	}
	if v := visible(got[1]); v != strings.Repeat("你", 5)+"x" {
		t.Errorf("second line = %q", v)
	}
	// A wide char that does not fit moves whole to the next line.
	got = wrapANSI("a"+strings.Repeat("你", 40), 80)
	if loom.StringWidth(got[0]) != 79 || got[1] != "你" {
		t.Errorf("odd wrap = %q", got)
	}
	// Combining marks stay in one cluster.
	if got := wrapANSI(strings.Repeat("é", 81), 80); len(got) != 2 || got[1] != "é" {
		t.Errorf("combining wrap = %q", got)
	}
}

func TestChatRendersANSIToolOutput(t *testing.T) {
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	app := NewAppWidget(spec, "codex:luna", "s", nil)
	green := styleOf("\x1b[32m")
	AddTurn(app.Chat, RoleTool, "exec", "\x1b[32m"+strings.Repeat("g", 90)+"\x1b[0m done\x1b[2J\x1b]52;c;QUFB\x07\x1b]0;pwn\x07")
	AddTurn(app.Chat, RoleAssistant, "codex:luna", "plain 你好 text")
	rows := RenderCanvas(app, 80, 20)
	screen := strings.Join(rows, "\n")
	for _, bad := range []string{"\x1b]", "\x07", "\x1b[2J", "52;c", "pwn"} {
		if strings.Contains(screen, bad) {
			t.Errorf("rendered screen contains %q", bad)
		}
	}
	var wrapped []loom.Cell
	for _, r := range rows {
		if strings.Contains(visible(r), strings.Repeat("g", 10)+" done") {
			wrapped = loom.ParseANSI(r)
		}
	}
	if wrapped == nil {
		t.Fatalf("wrapped tool row missing:\n%s", visible(screen))
	}
	if wrapped[0].Style.FG != green.FG || wrapped[11].Style.FG == green.FG {
		t.Errorf("wrapped row styles: first %+v, after reset %+v", wrapped[0].Style, wrapped[11].Style)
	}
	if !strings.Contains(visible(screen), "plain 你好 text") {
		t.Errorf("plain text missing:\n%s", visible(screen))
	}
}

func TestSanitizeLinesForJobLog(t *testing.T) {
	got := sanitizeLines([]string{"\x1b[31mred", "still\x1b[0m\x1b]0;t\x07", strings.Repeat("x", 200)})
	if len(got) != 3 || visible(got[1]) != "still" || len(got[2]) != 200 {
		t.Fatalf("sanitizeLines = %q", got)
	}
	if c := loom.ParseANSI(got[1]); c[0].Style != styleOf("\x1b[31m") {
		t.Errorf("style not carried to the next log line: %+v", c[0].Style)
	}
	if sanitizeLines(nil) != nil {
		t.Error("sanitizeLines(nil) != nil")
	}
}
