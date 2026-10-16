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
	// CancelledLabel is the neutral note shown when a turn is cancelled.
	CancelledLabel string `yaml:"cancelled_label"`
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
	OpenNative []string `yaml:"open_native"`
	ToggleJobs []string `yaml:"toggle_jobs"`
	JobOpen    []string `yaml:"job_open"`
	JobCancel  []string `yaml:"job_cancel"`
	JobClose   []string `yaml:"job_close"`
	// Readline-style prompt editing; all act on the current line.
	LineStart     []string `yaml:"line_start"`
	LineEnd       []string `yaml:"line_end"`
	DeleteToStart []string `yaml:"delete_to_start"`
	DeleteToEnd   []string `yaml:"delete_to_end"`
	DeleteWord    []string `yaml:"delete_word"`
	WordLeft      []string `yaml:"word_left"`
	WordRight     []string `yaml:"word_right"`
	// Picker keys act on an open @/ picker before any other binding.
	PickerSelect []string `yaml:"picker_select"`
	PickerClose  []string `yaml:"picker_close"`
	PickerUp     []string `yaml:"picker_up"`
	PickerDown   []string `yaml:"picker_down"`
}

// NativeLabelsSpec holds the open_native chat notes; placeholder {{ .Provider }}.
type NativeLabelsSpec struct {
	Returned    string `yaml:"returned"`
	Unsupported string `yaml:"unsupported"`
	NoSession   string `yaml:"no_session"`
	Busy        string `yaml:"busy"`
	Unavailable string `yaml:"unavailable"`
}

// NativeSpec configures the open_native key (the provider's own CLI).
type NativeSpec struct {
	Labels NativeLabelsSpec `yaml:"labels"`
}

// JobLabelsSpec holds the job manager's chat notes and panel texts.
// Placeholders: {{ .ID }}, {{ .Command }}, {{ .Status }}, {{ .Exit }},
// {{ .Duration }}, {{ .Running }}.
type JobLabelsSpec struct {
	Started       string `yaml:"started"`
	Done          string `yaml:"done"`
	Usage         string `yaml:"usage"`
	Unavailable   string `yaml:"unavailable"`
	PanelTitle    string `yaml:"panel_title"`
	LogTitle      string `yaml:"log_title"`
	PanelEmpty    string `yaml:"panel_empty"`
	PanelHint     string `yaml:"panel_hint"`
	LogHint       string `yaml:"log_hint"`
	HeaderRunning string `yaml:"header_running"`
}

// JobsSpec configures background jobs started with the prompt prefix.
type JobsSpec struct {
	Prefix         string        `yaml:"prefix"`
	Shell          string        `yaml:"shell"`
	LogBufferBytes int           `yaml:"log_buffer_bytes"`
	CancelGraceMS  int           `yaml:"cancel_grace_ms"`
	MaxFinished    int           `yaml:"max_finished"`
	Labels         JobLabelsSpec `yaml:"labels"`
}

// PickerCommand is a TUI built-in listed in the command picker.
type PickerCommand struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	// Usage is the invocation form shown in the usage error, e.g. "/model <spec>".
	Usage string `yaml:"usage"`
}

// PickerLabelsSpec holds the picker titles and status rows.
type PickerLabelsSpec struct {
	FilesTitle    string `yaml:"files_title"`
	CommandsTitle string `yaml:"commands_title"`
	Indexing      string `yaml:"indexing"`
	NoMatches     string `yaml:"no_matches"`
	// ReadError replaces NoMatches when the file index hit a read error;
	// placeholder: {{ .Error }}.
	ReadError string `yaml:"read_error"`
}

// PickerSpec configures the inline @ file and / command pickers.
type PickerSpec struct {
	FileTrigger     string           `yaml:"file_trigger"`
	CommandTrigger  string           `yaml:"command_trigger"`
	MaxItems        int              `yaml:"max_items"`
	MaxWidth        int              `yaml:"max_width"`
	MaxIndexedFiles int              `yaml:"max_indexed_files"`
	SkipDirs        []string         `yaml:"skip_dirs"`
	Builtins        []PickerCommand  `yaml:"builtins"`
	Labels          PickerLabelsSpec `yaml:"labels"`
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

// SessionLabelsSpec names the backend session state shown in the header.
type SessionLabelsSpec struct {
	Resumed string `yaml:"resumed"`
	New     string `yaml:"new"`
}

// SessionsSpec configures the per-tui-session store of backend session ids.
type SessionsSpec struct {
	// Dir holds one <tui session id>.json per tui session; "~/" is expanded.
	Dir string `yaml:"dir"`
	// MaxAgeDays prunes other tui sessions' store files unused this long.
	MaxAgeDays int               `yaml:"max_age_days"`
	Labels     SessionLabelsSpec `yaml:"labels"`
}

// Spec is the top-level shape of spec/tui.yaml.
type Spec struct {
	Header      HeaderSpec      `yaml:"header"`
	Chat        ChatSpec        `yaml:"chat"`
	Input       InputSpec       `yaml:"input"`
	Sessions    SessionsSpec    `yaml:"sessions"`
	Terminal    TerminalSpec    `yaml:"terminal"`
	Editor      EditorSpec      `yaml:"editor"`
	Native      NativeSpec      `yaml:"native"`
	Jobs        JobsSpec        `yaml:"jobs"`
	Picker      PickerSpec      `yaml:"picker"`
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
	if s.Chat.CancelledLabel == "" {
		return nil, fmt.Errorf("%s: chat cancelled_label must be set", specPath)
	}
	if s.Jobs.Prefix == "" || s.Jobs.Shell == "" || s.Jobs.LogBufferBytes < 1 || s.Jobs.CancelGraceMS < 1 || s.Jobs.MaxFinished < 1 {
		return nil, fmt.Errorf("%s: jobs prefix, shell, log_buffer_bytes, cancel_grace_ms and max_finished must be set", specPath)
	}
	if s.Sessions.Dir == "" || s.Sessions.MaxAgeDays < 1 || s.Sessions.Labels.Resumed == "" || s.Sessions.Labels.New == "" {
		return nil, fmt.Errorf("%s: sessions dir, max_age_days and labels must be set", specPath)
	}
	if nl := s.Native.Labels; nl.Returned == "" || nl.Unsupported == "" || nl.NoSession == "" || nl.Busy == "" || nl.Unavailable == "" {
		return nil, fmt.Errorf("%s: native labels must be set", specPath)
	}
	if err := validatePicker(&s); err != nil {
		return nil, err
	}
	if s.Input.MinLines < 1 || s.Input.MaxLines < s.Input.MinLines {
		return nil, fmt.Errorf("%s: input min_lines and max_lines invalid", specPath)
	}
	return &s, nil
}

func validatePicker(s *Spec) error {
	p := s.Picker
	single := func(t string) bool { return len([]rune(t)) == 1 }
	if !single(p.FileTrigger) || !single(p.CommandTrigger) {
		return fmt.Errorf("%s: picker file_trigger and command_trigger must be one character", specPath)
	}
	if p.MaxItems < 1 || p.MaxWidth < 10 || p.MaxIndexedFiles < 1 {
		return fmt.Errorf("%s: picker max_items >= 1, max_width >= 10 and max_indexed_files >= 1 required", specPath)
	}
	for _, b := range p.Builtins {
		if b.Name == "" || b.Usage == "" {
			return fmt.Errorf("%s: picker builtins need name and usage", specPath)
		}
	}
	l := p.Labels
	if l.FilesTitle == "" || l.CommandsTitle == "" || l.Indexing == "" || l.NoMatches == "" || l.ReadError == "" {
		return fmt.Errorf("%s: picker labels must be set", specPath)
	}
	kb := s.Keybindings
	if len(kb.PickerSelect) == 0 || len(kb.PickerClose) == 0 || len(kb.PickerUp) == 0 || len(kb.PickerDown) == 0 {
		return fmt.Errorf("%s: picker_select, picker_close, picker_up and picker_down keybindings must be set", specPath)
	}
	return nil
}
