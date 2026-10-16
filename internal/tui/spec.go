package tui

import (
	"bytes"
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

const specPath = "spec/tui.yaml"

// HeaderSpec describes header status line configuration.
type HeaderSpec struct {
	Show             bool              `yaml:"show"`
	Format           string            `yaml:"format"`
	StatusIndicators map[string]string `yaml:"status_indicators"`
}

// ChatSpec describes the scrollable chat thread configuration.
type ChatSpec struct {
	MinHeight       int    `yaml:"min_height"`
	MaxScrollBack   int    `yaml:"max_scroll_back"`
	ShowTimestamps  bool   `yaml:"show_timestamps"`
	UserPrefix      string `yaml:"user_prefix"`
	AssistantPrefix string `yaml:"assistant_prefix"`
	ToolPrefix      string `yaml:"tool_prefix"`
	ErrorPrefix     string `yaml:"error_prefix"`
}

// InputSpec describes multiline prompt box behavior.
type InputSpec struct {
	HistorySize  int    `yaml:"history_size"`
	MinLines     int    `yaml:"min_lines"`
	MaxLines     int    `yaml:"max_lines"`
	PromptSymbol string `yaml:"prompt_symbol"`
}

// TerminalSpec describes terminal protocol options for the loom pane.
type TerminalSpec struct {
	// KittyKeyboard sets loom Pane.KittyKeyboard so Shift+Enter and Ctrl+J
	// arrive as distinct keys on terminals that support the protocol.
	KittyKeyboard bool `yaml:"kitty_keyboard"`
}

// KeybindingsSpec describes keyboard actions in harnez tui.
type KeybindingsSpec struct {
	SendPrompt []string `yaml:"send_prompt"`
	Newline    []string `yaml:"newline"`
	Quit       []string `yaml:"quit"`
	CancelTurn []string `yaml:"cancel_turn"`
	ScrollUp   []string `yaml:"scroll_up"`
	ScrollDown []string `yaml:"scroll_down"`
	OpenEditor []string `yaml:"open_editor"`
	TogglePTY  []string `yaml:"toggle_pty"`
}

// ThemeSpec describes styling for harnez tui.
type ThemeSpec struct {
	BorderStyle    string `yaml:"border_style"`
	HeaderColor    string `yaml:"header_color"`
	UserColor      string `yaml:"user_color"`
	AssistantColor string `yaml:"assistant_color"`
	ToolColor      string `yaml:"tool_color"`
	InputColor     string `yaml:"input_color"`
}

// Spec is the top-level shape of spec/tui.yaml.
type Spec struct {
	Header      HeaderSpec      `yaml:"header"`
	Chat        ChatSpec        `yaml:"chat"`
	Input       InputSpec       `yaml:"input"`
	Terminal    TerminalSpec    `yaml:"terminal"`
	Keybindings KeybindingsSpec `yaml:"keybindings"`
	Theme       ThemeSpec       `yaml:"theme"`
}

// LoadSpec loads and validates spec/tui.yaml from embedded harnez.DefaultFS.
func LoadSpec() (*Spec, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, specPath)
	if err != nil {
		return nil, fmt.Errorf("read embedded %s: %w", specPath, err)
	}
	return ParseSpec(data)
}

// ParseSpec parses and validates TUI specification YAML bytes.
func ParseSpec(data []byte) (*Spec, error) {
	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("%s decode error: %w", specPath, err)
	}
	if s.Chat.MinHeight < 1 {
		return nil, fmt.Errorf("%s: chat min_height must be >= 1", specPath)
	}
	if s.Input.MinLines < 1 || s.Input.MaxLines < s.Input.MinLines {
		return nil, fmt.Errorf("%s: input min_lines and max_lines invalid", specPath)
	}
	return &s, nil
}
