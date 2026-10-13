package tui

import (
	"strings"
	"testing"

	"ubunatic.com/loom"
)

func TestPromptBufferEditingAndCtrlJNewline(t *testing.T) {
	pb := NewPromptBuffer(3, 8, "> ")

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
	pb := NewPromptBuffer(1, 5, "> ")

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
