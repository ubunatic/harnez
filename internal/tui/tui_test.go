package tui

import (
	"strings"
	"testing"

	"ubunatic.com/loom"
)

func TestPromptBufferEditingAndCtrlJNewline(t *testing.T) {
	pb := NewPromptBuffer(3, 8, 100, "> ")

	// Type "hello"
	for _, r := range "hello" {
		pb.InsertChar(r)
	}
	if pb.Text() != "hello" {
		t.Fatalf("Text() = %q, want hello", pb.Text())
	}

	// Insert newline via Ctrl+J
	pb.InsertNewline()
	for _, r := range "world" {
		pb.InsertChar(r)
	}

	if pb.Text() != "hello\nworld" {
		t.Fatalf("multiline Text() = %q, want hello\\nworld", pb.Text())
	}

	lines := pb.Lines(40)
	if len(lines) < 3 {
		t.Fatalf("Lines() count = %d, want >= 3", len(lines))
	}
	if !strings.Contains(lines[0], "hello") {
		t.Errorf("line 0 = %q, want contains hello", lines[0])
	}
	if !strings.Contains(lines[1], "world") {
		t.Errorf("line 1 = %q, want contains world", lines[1])
	}

	// Test backspace at start of line 2 (merges with line 1)
	pb.cursorRow = 1
	pb.cursorCol = 0
	pb.Backspace()

	if pb.Text() != "helloworld" {
		t.Fatalf("merged Text() = %q, want helloworld", pb.Text())
	}
}

func TestPromptBufferHistoryNavigation(t *testing.T) {
	pb := NewPromptBuffer(1, 5, 100, "> ")

	pb.AddHistory("first prompt")
	pb.AddHistory("second prompt")

	// Up -> loads "second prompt"
	pb.MoveUp()
	if pb.Text() != "second prompt" {
		t.Errorf("history prev 1 = %q, want 'second prompt'", pb.Text())
	}

	// Up -> loads "first prompt"
	pb.MoveUp()
	if pb.Text() != "first prompt" {
		t.Errorf("history prev 2 = %q, want 'first prompt'", pb.Text())
	}

	// Down -> loads "second prompt"
	pb.MoveDown()
	if pb.Text() != "second prompt" {
		t.Errorf("history next 1 = %q, want 'second prompt'", pb.Text())
	}

	// Down -> restores draft
	pb.MoveDown()
	if pb.Text() != "" {
		t.Errorf("history restore draft = %q, want empty", pb.Text())
	}
}

func TestChatThreadTurnFormattingAndScrolling(t *testing.T) {
	spec := ChatSpec{
		MinHeight:       5,
		MaxScrollBack:   100,
		ShowTimestamps:  false,
		UserPrefix:      "User:",
		AssistantPrefix: "Agent ({{ .Model }}):",
		ToolPrefix:      "[Tool: {{ .Name }}]",
	}
	ct := NewChatThread(spec)

	AddTurn(ct, RoleUser, "", "Please refactor statusline.")
	AddTurn(ct, RoleAssistant, "claude:sonnet", "I will inspect statusline.go.")
	AddTurn(ct, RoleTool, "make test", "PASS")

	lines := ct.FormattedLines(80)
	if len(lines) == 0 {
		t.Fatal("FormattedLines() is empty")
	}

	visible := ct.VisibleLines(80, 10)
	if len(visible) == 0 {
		t.Fatal("VisibleLines() is empty")
	}

	joined := strings.Join(visible, "\n")
	if !strings.Contains(joined, "User:") || !strings.Contains(joined, "Agent (claude:sonnet):") {
		t.Errorf("visible chat missing expected headers, got:\n%s", joined)
	}

	// Test scroll up
	ScrollUp(ct, 3)
	if ct.scrollOffset != 3 {
		t.Errorf("scrollOffset = %d, want 3", ct.scrollOffset)
	}

	// Test scroll down
	ScrollDown(ct, 2)
	if ct.scrollOffset != 1 {
		t.Errorf("scrollOffset = %d, want 1", ct.scrollOffset)
	}
}

func TestAppWidgetKeyEventsAndLoomRendering(t *testing.T) {
	spec, err := LoadSpec()
	if err != nil {
		t.Fatalf("LoadSpec() error: %v", err)
	}

	submittedText := ""
	app := NewAppWidget(spec, "codex:luna", "sess-123", func(prompt string) {
		submittedText = prompt
	})

	// Type 'a', 'b', 'c'
	app.ConsumeKey(loom.KeyEvent{Text: "a"})
	app.ConsumeKey(loom.KeyEvent{Text: "b"})
	app.ConsumeKey(loom.KeyEvent{Text: "c"})

	if app.Prompt.Text() != "abc" {
		t.Fatalf("Prompt.Text() = %q, want abc", app.Prompt.Text())
	}

	// Press Ctrl+J for newline
	app.ConsumeKey(loom.KeyEvent{Key: "ctrl-j"})
	app.ConsumeKey(loom.KeyEvent{Text: "d"})

	if app.Prompt.Text() != "abc\nd" {
		t.Fatalf("Prompt.Text() after Ctrl+J = %q, want abc\\nd", app.Prompt.Text())
	}

	// Press Enter to submit
	app.ConsumeKey(loom.KeyEvent{Key: "enter"})

	if submittedText != "abc\nd" {
		t.Fatalf("submittedText = %q, want abc\\nd", submittedText)
	}
	if app.Prompt.Text() != "" {
		t.Fatalf("Prompt text after submission = %q, want empty", app.Prompt.Text())
	}

	// Verify chat thread contains the submitted user turn
	turns := app.Chat.Turns()
	if len(turns) != 1 || turns[0].Content != "abc\nd" {
		t.Fatalf("Chat turns = %#v, want 1 user turn with 'abc\\nd'", turns)
	}

	// Render frame to canvas string lines
	canvasLines := RenderCanvas(app, 80, 24)
	if len(canvasLines) != 24 {
		t.Fatalf("RenderCanvas lines count = %d, want 24", len(canvasLines))
	}

	fullFrame := strings.Join(canvasLines, "\n")
	if !strings.Contains(fullFrame, "harnez tui") || !strings.Contains(fullFrame, "codex:luna") {
		t.Errorf("Rendered frame missing header status line, got:\n%s", fullFrame)
	}
	if !strings.Contains(fullFrame, "─") {
		t.Errorf("Rendered frame missing prompt divider line")
	}

	// Press Ctrl+C to test quit request
	app.ConsumeKey(loom.KeyEvent{Key: "ctrl-c"})
	if !app.QuitRequested {
		t.Error("expected app.QuitRequested to be true after Ctrl+C")
	}
}

// Real terminals deliver Enter and Ctrl+J as the same "enter" key, so the
// newline paths that work in practice are a trailing backslash and Alt+Enter.
func TestAppWidgetTerminalNewlinePaths(t *testing.T) {
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	app := NewAppWidget(spec, "m", "s", func(p string) { got = append(got, p) })
	feed := func(raw string) { app.ConsumeKey(loom.DecodeKey([]byte(raw))) }

	for _, r := range "äb\\" {
		app.ConsumeKey(loom.KeyEvent{Text: string(r)})
	}
	feed("\r") // backslash + Enter -> newline, no submit
	app.ConsumeKey(loom.KeyEvent{Text: "c"})
	feed("\x1b\r") // Alt+Enter -> newline
	app.ConsumeKey(loom.KeyEvent{Text: "d"})
	if want := "äb\nc\nd"; app.Prompt.Text() != want || len(got) != 0 {
		t.Fatalf("Text() = %q, submits = %v; want %q and no submit", app.Prompt.Text(), got, want)
	}
	feed("\r")
	if len(got) != 1 || got[0] != "äb\nc\nd" {
		t.Fatalf("submits = %q", got)
	}

	app.ConsumePaste(loom.PasteEvent{Text: "x\r\ny"})
	if app.Prompt.Text() != "x\ny" {
		t.Fatalf("paste Text() = %q, want x\\ny", app.Prompt.Text())
	}

	header := RenderCanvas(app, 80, 24)[0]
	if !strings.Contains(header, "● ready") {
		t.Errorf("header = %q, want idle status indicator", header)
	}
	if app.ConsumeKey(loom.DecodeKey([]byte("\x1b"))); !app.QuitRequested {
		t.Error("esc should quit")
	}
}

func TestPromptBufferWrapsAndKeepsCursorVisible(t *testing.T) {
	pb := NewPromptBuffer(1, 2, 2, "> ")
	for range 50 {
		pb.InsertChar('x')
	}
	lines := pb.Lines(20)
	if len(lines) != 2 || !strings.Contains(lines[1], "█") {
		t.Fatalf("Lines() = %q, want 2 wrapped lines with cursor on the last", lines)
	}
	for _, l := range lines {
		if n := len([]rune(l)); n > 20 {
			t.Errorf("line %q is %d wide, want <= 20", l, n)
		}
	}

	for _, h := range []string{"a", "b", "c"} {
		pb.AddHistory(h)
	}
	if len(pb.history) != 2 || pb.history[0] != "b" {
		t.Errorf("history = %q, want capped to [b c]", pb.history)
	}
}

// TestAppWidgetKittyNewlineKeys covers #775: with the kitty keyboard protocol
// Shift+Enter and Ctrl+J insert a newline; legacy Ctrl+J (LF) and Enter send.
func TestAppWidgetKittyNewlineKeys(t *testing.T) {
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	app := NewAppWidget(spec, "m", "s", func(p string) { got = append(got, p) })
	feed := func(raw string) { app.ConsumeKey(loom.DecodeKey([]byte(raw))) }
	typ := func(s string) {
		for _, r := range s {
			app.ConsumeKey(loom.KeyEvent{Text: string(r)})
		}
	}

	for _, tc := range []struct{ name, raw, key string }{
		{"shift-enter", "\x1b[13;2u", "shift-enter"},
		{"ctrl-j", "\x1b[106;5u", "ctrl-j"},
		{"kitty alt-enter", "\x1b[13;3u", "alt-enter"},
		{"legacy alt-enter", "\x1b\r", "alt-enter"},
	} {
		if k := keyName(loom.DecodeKey([]byte(tc.raw))); k != tc.key {
			t.Fatalf("%s decodes to %q, want %q", tc.name, k, tc.key)
		}
		typ("a")
		feed(tc.raw)
		typ("b")
		if app.Prompt.Text() != "a\nb" || len(got) != 0 {
			t.Fatalf("%s: Text() = %q, submits = %v; want newline, no submit", tc.name, app.Prompt.Text(), got)
		}
		app.Prompt.Clear()
	}

	typ("x")
	feed("\n") // legacy Ctrl+J arrives as Enter and sends
	typ("y")
	feed("\r")
	if len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Fatalf("submits = %q, want [x y]", got)
	}
}

func TestReadlineKeys(t *testing.T) {
	type step struct {
		keys     []string
		text     string
		row, col int
	}
	cases := []struct {
		name  string
		start string // typed into the prompt; the cursor ends after it
		steps []step
	}{
		{"home-end", "hello world", []step{
			{[]string{"ctrl-a"}, "hello world", 0, 0},
			{[]string{"ctrl-e"}, "hello world", 0, 11},
			{[]string{"home"}, "hello world", 0, 0},
			{[]string{"end"}, "hello world", 0, 11},
		}},
		{"kill-line", "hello world", []step{
			{[]string{"left", "left", "left", "left", "left", "ctrl-k"}, "hello ", 0, 6},
			{[]string{"left", "ctrl-u"}, " ", 0, 0},
		}},
		{"delete-word", "see internal/tui/app.go", []step{
			{[]string{"ctrl-w"}, "see internal/tui/app.", 0, 21},
			{[]string{"ctrl-w"}, "see internal/tui/", 0, 17},
			{[]string{"ctrl-w", "ctrl-w"}, "see ", 0, 4},
			{[]string{"ctrl-w"}, "", 0, 0},
			{[]string{"ctrl-w"}, "", 0, 0},
		}},
		{"word-moves", "ab  cd_e fg", []step{
			{[]string{"alt-b"}, "ab  cd_e fg", 0, 9},
			{[]string{"alt-b"}, "ab  cd_e fg", 0, 4},
			{[]string{"alt-b", "alt-b"}, "ab  cd_e fg", 0, 0},
			{[]string{"alt-f"}, "ab  cd_e fg", 0, 2},
			{[]string{"alt-f", "alt-f", "alt-f", "alt-f"}, "ab  cd_e fg", 0, 11},
			{[]string{"ctrl-left"}, "ab  cd_e fg", 0, 9},
			{[]string{"ctrl-right"}, "ab  cd_e fg", 0, 11},
		}},
	}
	for _, tc := range cases {
		app, _, _ := newPickerApp(t, nil)
		typeText(app, tc.start)
		for i, st := range tc.steps {
			for _, k := range st.keys {
				if r := app.ConsumeKey(loom.KeyEvent{Key: k}); r != loom.Handled() {
					t.Fatalf("%s step %d: %s not handled", tc.name, i, k)
				}
			}
			row, col := app.Prompt.Cursor()
			if app.Prompt.Text() != st.text || row != st.row || col != st.col {
				t.Fatalf("%s step %d %v: text %q at %d,%d; want %q at %d,%d", tc.name, i, st.keys, app.Prompt.Text(), row, col, st.text, st.row, st.col)
			}
		}
	}
}

// In a multi-line prompt every readline key stays on the current line.
func TestReadlineKeysActOnCurrentLine(t *testing.T) {
	app, _, _ := newPickerApp(t, nil)
	typeText(app, "one two")
	app.ConsumeKey(loom.KeyEvent{Key: "ctrl-j"})
	typeText(app, "three four")
	app.ConsumeKey(loom.KeyEvent{Key: "ctrl-j"})
	typeText(app, "five six")
	app.ConsumeKey(loom.KeyEvent{Key: "up"})
	app.ConsumeKey(loom.KeyEvent{Key: "ctrl-a"})
	app.ConsumeKey(loom.KeyEvent{Key: "ctrl-w"}) // line start: nothing to delete, no line merge
	if got := app.Prompt.Text(); got != "one two\nthree four\nfive six" {
		t.Fatalf("ctrl-w at line start changed text: %q", got)
	}
	app.ConsumeKey(loom.KeyEvent{Key: "alt-f"})
	app.ConsumeKey(loom.KeyEvent{Key: "ctrl-k"})
	app.ConsumeKey(loom.KeyEvent{Key: "alt-b"}) // back to the first word start
	app.ConsumeKey(loom.KeyEvent{Key: "ctrl-e"})
	app.ConsumeKey(loom.KeyEvent{Key: "ctrl-u"})
	if got, row := app.Prompt.Text(), func() int { r, _ := app.Prompt.Cursor(); return r }(); got != "one two\n\nfive six" || row != 1 {
		t.Fatalf("text %q row %d; want the middle line emptied only", got, row)
	}
	app.ConsumeKey(loom.KeyEvent{Key: "ctrl-e"})
	if _, col := app.Prompt.Cursor(); col != 0 {
		t.Fatalf("ctrl-e on empty line: col %d", col)
	}
}

// Readline keys must not collide with any other spec binding.
func TestReadlineKeysDoNotStealBindings(t *testing.T) {
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	kb := spec.Keybindings
	readline := map[string][]string{
		"line_start": kb.LineStart, "line_end": kb.LineEnd, "delete_to_start": kb.DeleteToStart,
		"delete_to_end": kb.DeleteToEnd, "delete_word": kb.DeleteWord, "word_left": kb.WordLeft, "word_right": kb.WordRight,
	}
	others := map[string][]string{
		"send_prompt": kb.SendPrompt, "newline": kb.Newline, "quit": kb.Quit, "cancel_turn": kb.CancelTurn,
		"scroll_up": kb.ScrollUp, "scroll_down": kb.ScrollDown, "open_editor": kb.OpenEditor, "open_native": kb.OpenNative,
		"toggle_jobs": kb.ToggleJobs, "picker_select": kb.PickerSelect, "picker_close": kb.PickerClose,
		"picker_up": kb.PickerUp, "picker_down": kb.PickerDown,
	}
	seen := map[string]string{}
	for name, keys := range readline {
		if len(keys) == 0 {
			t.Errorf("%s has no keys", name)
		}
		for _, k := range keys {
			n := strings.ReplaceAll(strings.ToLower(k), "+", "-")
			if prev, ok := seen[n]; ok {
				t.Errorf("%s and %s both bind %q", prev, name, k)
			}
			seen[n] = name
			for on, ok := range others {
				if bound(n, ok) {
					t.Errorf("%s binds %q, already used by %s", name, k, on)
				}
			}
		}
	}
}
