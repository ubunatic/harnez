package transcript

import (
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractEmitsBeforeReadingRest(t *testing.T) {
	reader := &gatedReader{
		first: fixtureLine(t, "first", nil),
		rest:  fixtureLine(t, "second", nil),
	}
	stats, err := Extract(reader, Claude{}, func(r Record) error {
		reader.emitted = true
		return nil
	})
	if err != nil || stats.UserMessages != 2 {
		t.Fatalf("streaming evidence: %+v %v", stats, err)
	}
}

type gatedReader struct {
	first, rest string
	emitted     bool
}

func (r *gatedReader) Read(p []byte) (int, error) {
	if r.first != "" {
		n := copy(p, r.first)
		r.first = r.first[n:]
		return n, nil
	}
	if !r.emitted {
		return 0, io.ErrNoProgress
	}
	if r.rest == "" {
		return 0, io.EOF
	}
	n := copy(p, r.rest)
	r.rest = r.rest[n:]
	return n, nil
}

func TestWriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	writeFixture(t, path, fixtureLine(t, "/issue retain arguments", nil)+fixtureLine(t, "summary", map[string]any{"isCompactSummary": true}))
	var output strings.Builder
	if err := WriteFile(&output, path, true); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Source struct {
			Path  string `json:"path"`
			Bytes int64  `json:"bytes"`
		} `json:"source"`
		Records []Record `json:"records"`
		Stats   Stats    `json:"stats"`
	}
	if err := json.Unmarshal([]byte(output.String()), &result); err != nil {
		t.Fatal(err)
	}
	if result.Source.Path != path || result.Source.Bytes == 0 || len(result.Records) != 2 ||
		result.Records[0].Text != "/issue retain arguments" || result.Records[0].UUID != "fixture-uuid" || result.Records[0].Line != 1 ||
		result.Records[1].Line != 2 || result.Stats.UserMessages != 1 || result.Stats.Summaries != 1 {
		t.Fatalf("incorrect JSON: %s", output.String())
	}
	output.Reset()
	if err := WriteFile(&output, path, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{path, "--- user 2026-10-20T18:00:00Z uuid=fixture-uuid line=1 ---", "/issue retain arguments", "uuid=fixture-uuid", "line=2", "Summaries: 1", "User messages: 1"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("text output missing %q", want)
		}
	}
	for _, asJSON := range []bool{false, true} {
		if err := WriteFile(failingWriter{}, path, asJSON); err != io.ErrClosedPipe {
			t.Fatalf("lost output error: %v", err)
		}
	}
}

func TestWriteEmptyAndToolOnly(t *testing.T) {
	for _, input := range []string{"", fixtureLine(t, []any{map[string]any{"type": "tool_result", "content": "output"}}, nil)} {
		path := filepath.Join(t.TempDir(), "empty.jsonl")
		writeFixture(t, path, input)
		var out strings.Builder
		if err := WriteFile(&out, path, true); err != nil {
			t.Fatal(err)
		}
		if !json.Valid([]byte(out.String())) || !strings.Contains(out.String(), `"records":[]`) || !strings.Contains(out.String(), `"user_messages":0`) {
			t.Fatalf("empty JSON: %s", out.String())
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
