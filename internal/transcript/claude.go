// Package transcript extracts retrospective evidence without retaining a session
// in memory. Only the Claude Code transcript format is currently supported.
package transcript

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Record is one compaction summary or owner message in transcript order.
type Record struct {
	Kind              string `json:"kind"`
	Timestamp         string `json:"timestamp,omitempty"`
	UUID              string `json:"uuid,omitempty"`
	Line              int    `json:"line"`
	Text              string `json:"text"`
	structuredCommand bool
}

// Adapter is the provider seam: decode one JSONL record into selected evidence.
// A nil record means the line contains no retrospective evidence.
type Adapter interface {
	Decode([]byte) (*Record, error)
}

// Claude decodes Claude Code's nested message.content format.
type Claude struct{}

// Decode selects summaries and owner text from one Claude JSONL record.
func (Claude) Decode(line []byte) (*Record, error) {
	var row struct {
		Type             string `json:"type"`
		Timestamp        string `json:"timestamp"`
		UUID             string `json:"uuid"`
		IsCompactSummary bool   `json:"isCompactSummary"`
		IsMeta           bool   `json:"isMeta"`
		Message          struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &row); err != nil {
		return nil, err
	}
	if !row.IsCompactSummary && (row.Type != "user" || row.IsMeta) {
		return nil, nil
	}
	text, err := claudeContent(row.Message.Content)
	if err != nil {
		return nil, err
	}
	kind := "summary"
	structuredCommand := false
	if !row.IsCompactSummary {
		kind = "user"
		structuredCommand = strings.Contains(text, "<command-name>")
		text = ownerText(text)
	}
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	return &Record{Kind: kind, Timestamp: row.Timestamp, UUID: row.UUID, Text: text, structuredCommand: structuredCommand}, nil
}

func commandTwin(plain, structured Record) bool {
	if plain.Kind != "user" || structured.Kind != "user" || plain.structuredCommand ||
		!structured.structuredCommand || !strings.HasPrefix(plain.Text, "/") || plain.Text != structured.Text {
		return false
	}
	first, err := time.Parse(time.RFC3339Nano, plain.Timestamp)
	if err != nil {
		return false
	}
	second, err := time.Parse(time.RFC3339Nano, structured.Timestamp)
	if err != nil {
		return false
	}
	delta := second.Sub(first)
	if delta < 0 {
		delta = -delta
	}
	return delta <= time.Second
}

func claudeContent(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var text string
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return "", err
		}
		return text, nil
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", err
	}
	var parts []string
	image := 0
	for _, block := range blocks {
		switch block.Type {
		case "text":
			parts = append(parts, block.Text)
		case "image":
			image++
			parts = append(parts, fmt.Sprintf("[Image #%d]", image))
		}
	}
	return strings.Join(parts, "\n"), nil
}

func tagContent(text, tag string) string {
	_, rest, found := strings.Cut(text, "<"+tag+">")
	if !found {
		return ""
	}
	value, _, _ := strings.Cut(rest, "</"+tag+">")
	return strings.TrimSpace(value)
}

func ownerText(text string) string {
	// Remove injected blocks, retaining real owner text before and after them.
	for _, tag := range []string{"system-reminder", "task-notification", "local-command-caveat", "local-command-stdout", "bash-input", "bash-stdout", "bash-stderr"} {
		for {
			before, rest, found := strings.Cut(text, "<"+tag+">")
			if !found {
				break
			}
			_, after, _ := strings.Cut(rest, "</"+tag+">")
			text = before + after
		}
	}
	if name := tagContent(text, "command-name"); name != "" {
		return strings.TrimSpace(name + " " + tagContent(text, "command-args"))
	}
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "Base directory for this skill:") ||
		strings.HasPrefix(text, "This session is being continued from a previous conversation") {
		return ""
	}
	return text
}
