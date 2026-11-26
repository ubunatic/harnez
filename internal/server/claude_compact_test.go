package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	data := ""
	for _, line := range lines {
		data += line + "\n"
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadClaudeCompactNewestBoundaryAndMainModel(t *testing.T) {
	path := writeTranscript(t,
		`{"type":"assistant","timestamp":"2026-11-26T10:00:00Z","message":{"model":"claude-opus-5-5","usage":{"input_tokens":1}}}`,
		`{"type":"system","subtype":"compact_boundary","timestamp":"2026-11-26T10:05:00Z","compactMetadata":{"trigger":"auto","preTokens":100,"postTokens":10}}`,
		`{"type":"assistant","isSidechain":true,"timestamp":"2026-11-26T11:00:00Z","message":{"model":"claude-haiku-5-5"}}`,
		`{"type":"assistant","timestamp":"2026-11-26T11:30:00Z","message":{"model":"<synthetic>"}}`,
		`{"type":"assistant","timestamp":"2026-11-26T11:40:00Z","message":{"model":"claude-sonnet-5-5"}}`,
		`{"type":"system","subtype":"compact_boundary","timestamp":"2026-11-26T12:00:00Z","compactMetadata":{"trigger":"manual","preTokens":300,"postTokens":30}}`,
	)
	got := readClaudeCompact(path)
	if got == nil {
		t.Fatal("readClaudeCompact = nil, want the newest boundary")
	}
	if got.PreTokens != 300 || got.PostTokens != 30 {
		t.Errorf("tokens = %d/%d, want 300/30", got.PreTokens, got.PostTokens)
	}
	if got.Trigger != "manual" {
		t.Errorf("Trigger = %q, want manual", got.Trigger)
	}
	if got.Model != "claude-sonnet-5-5" {
		t.Errorf("Model = %q, want the last main-chain model before the boundary", got.Model)
	}
	want := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	if !got.Timestamp.Equal(want) {
		t.Errorf("Timestamp = %s, want %s", got.Timestamp, want)
	}
}

func TestReadClaudeCompactIgnoresBoundaryWithoutBothCounts(t *testing.T) {
	path := writeTranscript(t,
		`{"type":"system","subtype":"compact_boundary","timestamp":"2026-11-26T10:05:00Z","compactMetadata":{"trigger":"auto","preTokens":100,"postTokens":10}}`,
		`{"type":"system","subtype":"compact_boundary","timestamp":"2026-11-26T12:00:00Z","compactMetadata":{"trigger":"auto","preTokens":300}}`,
	)
	got := readClaudeCompact(path)
	if got == nil || got.PostTokens != 10 {
		t.Fatalf("readClaudeCompact = %+v, want the earlier complete boundary", got)
	}
}

func TestReadClaudeCompactNoBoundaryOrUnreadable(t *testing.T) {
	noBoundary := writeTranscript(t,
		`{"type":"assistant","message":{"model":"claude-opus-5-5"}}`,
		`not json at all`,
	)
	if got := readClaudeCompact(noBoundary); got != nil {
		t.Errorf("no boundary: got %+v, want nil", got)
	}
	if got := readClaudeCompact(filepath.Join(t.TempDir(), "missing.jsonl")); got != nil {
		t.Errorf("missing file: got %+v, want nil", got)
	}
	if got := readClaudeCompact(t.TempDir()); got != nil {
		t.Errorf("directory (unreadable): got %+v, want nil", got)
	}
}
