package tui

import (
	"encoding/json"
	"io/fs"
	"testing"

	"ubunatic.com/harnez"
)

func TestEmbeddedTUISpecLoadsAndValidates(t *testing.T) {
	spec, err := LoadSpec()
	if err != nil {
		t.Fatalf("LoadSpec() failed: %v", err)
	}

	if !spec.Header.Show {
		t.Error("expected spec.Header.Show to be true")
	}
	if spec.Chat.MinHeight < 1 {
		t.Errorf("got min_height=%d, want >= 1", spec.Chat.MinHeight)
	}
	if len(spec.Keybindings.SendPrompt) == 0 {
		t.Error("expected keybindings.send_prompt to be non-empty")
	}
	if spec.Theme.BorderStyle == "" {
		t.Error("expected theme.border_style to be set")
	}
}

func TestTUISchemaJSONValid(t *testing.T) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/tui.schema.json")
	if err != nil {
		t.Fatalf("read spec/schemas/tui.schema.json error: %v", err)
	}

	var schema struct {
		Title      string                 `json:"title"`
		Required   []string               `json:"required"`
		Properties map[string]interface{} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("json unmarshal schema error: %v", err)
	}

	if schema.Title == "" {
		t.Error("expected title in tui.schema.json")
	}
	expectedRequired := []string{"header", "chat", "input", "terminal", "keybindings", "theme"}
	for _, req := range expectedRequired {
		found := false
		for _, r := range schema.Required {
			if r == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing required property %q in schema.Required", req)
		}
	}
}

func TestTUISpecTerminalAndNewlineKeys(t *testing.T) {
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	if !spec.Terminal.KittyKeyboard {
		t.Error("terminal.kitty_keyboard should default to true")
	}
	for _, k := range []string{"shift-enter", "ctrl-j", "alt-enter"} {
		if !bound(k, spec.Keybindings.Newline) {
			t.Errorf("newline keybindings %v lack %q", spec.Keybindings.Newline, k)
		}
	}
	if bound("enter", spec.Keybindings.Newline) || !bound("enter", spec.Keybindings.SendPrompt) {
		t.Error("enter must send, not insert a newline")
	}
}
