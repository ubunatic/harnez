package transcript

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixtureLine(t *testing.T, content any, extra map[string]any) string {
	t.Helper()
	row := map[string]any{"type": "user", "timestamp": "2026-10-20T18:00:00Z", "uuid": "fixture-uuid", "message": map[string]any{"content": content}}
	for key, value := range extra {
		row[key] = value
	}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + "\n"
}

func TestClaudeEvidence(t *testing.T) {
	tests := []struct {
		name    string
		content any
		extra   map[string]any
		kind    string
		text    string
	}{
		{"owner", "keep all of this decision", nil, "user", "keep all of this decision"},
		{"reminders", "<system-reminder>injected</system-reminder>keep <system-reminder>more</system-reminder>both", nil, "user", "keep both"},
		{"only reminder", "<system-reminder>injected</system-reminder>", nil, "", ""},
		{"slash args", "<command-message>lean-sprint</command-message>\n<command-name>/lean-sprint</command-name>\n<command-args>097 --model luna\nkeep line two</command-args>", nil, "user", "/lean-sprint 097 --model luna\nkeep line two"},
		{"slash no args", "<command-name>/compact</command-name>", nil, "user", "/compact"},
		{"literal slash", "/issue keep these arguments", nil, "user", "/issue keep these arguments"},
		{"tool only", []any{map[string]any{"type": "tool_result", "content": "secret output"}}, nil, "", ""},
		{"mixed blocks", []any{map[string]any{"type": "tool_result", "content": "noise"}, map[string]any{"type": "text", "text": "owner"}, map[string]any{"type": "image"}}, nil, "user", "owner\n[Image #1]"},
		{"images", []any{map[string]any{"type": "image"}, map[string]any{"type": "image"}}, nil, "user", "[Image #1]\n[Image #2]"},
		{"skill body", "Base directory for this skill: /tmp/skill\nbody", nil, "", ""},
		{"meta", "injected text", map[string]any{"isMeta": true}, "", ""},
		{"assistant", "assistant output", map[string]any{"type": "assistant"}, "", ""},
		{"summary", "This session is being continued from a previous conversation\nsummary", map[string]any{"isCompactSummary": true}, "summary", "This session is being continued from a previous conversation\nsummary"},
		{"preamble", "This session is being continued from a previous conversation\nnot owner", nil, "", ""},
		{"notification", "<task-notification>agent done</task-notification>", nil, "", ""},
		{"echo", "<bash-input>ls</bash-input><bash-stdout>output</bash-stdout>", nil, "", ""},
		{"interrupt", "[Request interrupted by user]\nplease stop", nil, "user", "[Request interrupted by user]\nplease stop"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, err := (Claude{}).Decode([]byte(fixtureLine(t, tt.content, tt.extra)))
			if err != nil {
				t.Fatal(err)
			}
			if tt.kind == "" {
				if record != nil {
					t.Fatalf("unexpected evidence: %+v", record)
				}
				return
			}
			want := &Record{Kind: tt.kind, Text: tt.text, Timestamp: "2026-10-20T18:00:00Z", UUID: "fixture-uuid"}
			if text, ok := tt.content.(string); ok {
				want.structuredCommand = strings.Contains(text, "<command-name>")
			}
			if !reflect.DeepEqual(record, want) {
				t.Fatalf("got %+v; want %+v", record, want)
			}
		})
	}
}

func TestExtractDeduplicatesAdjacentStructuredSlashCommand(t *testing.T) {
	plain := `{"type":"user","timestamp":"2026-10-20T18:00:00.000Z","uuid":"plain","message":{"content":"/compact"}}`
	twin := `{"type":"user","timestamp":"2026-10-20T18:00:00.500Z","uuid":"structured","message":{"content":"<command-name>/compact</command-name>"}}`
	late := `{"type":"user","timestamp":"2026-10-20T18:00:02.000Z","uuid":"later","message":{"content":"/compact"}}`
	summary := fixtureLine(t, "summary between command twins", map[string]any{"isCompactSummary": true})
	var got []Record
	stats, err := Extract(strings.NewReader(plain+"\n"+summary+twin+"\n"+late+"\n"), Claude{}, func(r Record) error {
		got = append(got, r)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.UserMessages != 2 || stats.Summaries != 1 || len(got) != 3 {
		t.Fatalf("paired command duplicate not removed or later command lost: stats=%+v records=%+v", stats, got)
	}
	if got[0].Kind != "summary" || got[0].Line != 2 || got[1].UUID != "structured" || got[1].Line != 3 || got[2].UUID != "later" || got[2].Line != 4 {
		t.Fatalf("wrong source records preserved: %+v", got)
	}
}

func TestExtractKeepsSlashCommandNearMiss(t *testing.T) {
	plain := `{"type":"user","timestamp":"2026-10-20T18:00:00.000Z","uuid":"plain","message":{"content":"/compact"}}`
	twin := `{"type":"user","timestamp":"2026-10-20T18:00:01.500Z","uuid":"structured","message":{"content":"<command-name>/compact</command-name>"}}`
	var got []Record
	stats, err := Extract(strings.NewReader(plain+"\n"+twin+"\n"), Claude{}, func(r Record) error {
		got = append(got, r)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.UserMessages != 2 || len(got) != 2 || got[0].UUID != "plain" || got[1].UUID != "structured" {
		t.Fatalf("near-miss command was dropped: stats=%+v records=%+v", stats, got)
	}
}

func TestExtractHugeAndTruncated(t *testing.T) {
	// Larger than both Scanner's default limit and the unrelated compact parser's
	// 10 MB ceiling. No fixture transcript is copied from a real session.
	huge := strings.Repeat("x", 11*1024*1024)
	input := fixtureLine(t, huge, nil) + fixtureLine(t, "summary", map[string]any{"isCompactSummary": true}) + `{"type":"user","message":`
	var got []Record
	stats, err := Extract(strings.NewReader(input), Claude{}, func(r Record) error {
		got = append(got, r)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Text != huge || got[1].Text != "summary" {
		t.Fatalf("huge message lost or reordered: records=%d", len(got))
	}
	if stats.UserMessages != 1 || stats.Summaries != 1 || !stats.TruncatedTail || stats.TextBytes != len(huge)+len("summary") {
		t.Fatalf("stats: %+v", stats)
	}
}

func TestExtractErrorsAndFinalLine(t *testing.T) {
	for _, input := range []string{"{broken}\n", "{broken}", "{\"type\":\n"} {
		_, err := Extract(strings.NewReader(input), Claude{}, func(Record) error { return nil })
		if err == nil || !strings.Contains(err.Error(), "line 1") {
			t.Fatalf("invalid line %q: %v", input, err)
		}
	}
	input := strings.TrimSuffix(fixtureLine(t, "complete final line", nil), "\n")
	want := errors.New("output stopped")
	_, err := Extract(strings.NewReader(input), Claude{}, func(Record) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("lost emit error: %v", err)
	}
	stats, err := Extract(strings.NewReader(input), Claude{}, func(Record) error { return nil })
	if err != nil || stats.UserMessages != 1 || stats.TruncatedTail {
		t.Fatalf("complete unterminated final line: %+v %v", stats, err)
	}
	stats, err = Extract(strings.NewReader("\n"), Claude{}, func(Record) error { t.Fatal("empty input emitted"); return nil })
	if err != nil || stats != (Stats{}) {
		t.Fatalf("empty: %+v %v", stats, err)
	}
	_, err = Extract(errorReader{}, Claude{}, func(Record) error { return nil })
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("lost read error: %v", err)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrClosedPipe }

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestResolve(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "projects", "slug", "abc-123.jsonl")
	writeFixture(t, path, "")
	writeFixture(t, filepath.Join(root, "projects", "slug", "abc-123", "subagents", "abc-sub.jsonl"), "")
	for _, id := range []string{path, "abc", "abc-123"} {
		got, err := Resolve(root, id)
		if err != nil || got != path {
			t.Fatalf("resolve %s: %s %v", id, got, err)
		}
	}
	for _, id := range []string{"", "missing", "abc-sub", filepath.Join(root, "missing.jsonl"), root} {
		if _, err := Resolve(root, id); err == nil {
			t.Fatalf("expected missing/invalid session error for %q", id)
		}
	}
	writeFixture(t, filepath.Join(root, "projects", "other", "abc-456.jsonl"), "")
	if _, err := Resolve(root, "abc"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous prefix: %v", err)
	}
}
