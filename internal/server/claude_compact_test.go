package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testMaxBytes bounds the test transcripts: each fits whole.
const testMaxBytes = 1 << 20

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
	got := readClaudeCompact(path, fillSession, testMaxBytes)
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
	got := readClaudeCompact(path, fillSession, testMaxBytes)
	if got == nil || got.PostTokens != 10 {
		t.Fatalf("readClaudeCompact = %+v, want the earlier complete boundary", got)
	}
}

func TestReadClaudeCompactNoBoundaryOrUnreadable(t *testing.T) {
	noBoundary := writeTranscript(t,
		`{"type":"assistant","message":{"model":"claude-opus-5-5"}}`,
		`not json at all`,
	)
	if got := readClaudeCompact(noBoundary, fillSession, testMaxBytes); got != nil {
		t.Errorf("no boundary: got %+v, want nil", got)
	}
	if got := readClaudeCompact(filepath.Join(t.TempDir(), "missing.jsonl"), fillSession, testMaxBytes); got != nil {
		t.Errorf("missing file: got %+v, want nil", got)
	}
	if got := readClaudeCompact(t.TempDir(), fillSession, testMaxBytes); got != nil {
		t.Errorf("directory (unreadable): got %+v, want nil", got)
	}
}

// Issue 1303 M3: a sidechain boundary and a boundary of another session are never
// the compaction of this session, even when they are newer.
func TestScanClaudeCompactSkipsSidechainAndForeignSession(t *testing.T) {
	path := writeTranscript(t,
		`{"type":"system","subtype":"compact_boundary","sessionId":"claude-fill","timestamp":"2026-11-26T10:05:00Z","compactMetadata":{"trigger":"manual","preTokens":100,"postTokens":10}}`,
		`{"type":"system","subtype":"compact_boundary","isSidechain":true,"sessionId":"claude-fill","timestamp":"2026-11-26T11:00:00Z","compactMetadata":{"trigger":"manual","preTokens":900,"postTokens":90}}`,
		`{"type":"system","subtype":"compact_boundary","sessionId":"other-session","timestamp":"2026-11-26T12:00:00Z","compactMetadata":{"trigger":"manual","preTokens":800,"postTokens":80}}`,
		`{"type":"system","subtype":"compact_boundary","sessionId":"older-session","timestamp":"2026-11-26T13:00:00Z","compactMetadata":{"trigger":"manual","preTokens":700,"postTokens":70}}`,
	)
	got, err := scanClaudeCompact(path, fillSession, testMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.PostTokens != 10 {
		t.Fatalf("scan = %+v, want this session's own boundary (post 10), not the near-misses", got)
	}
}

func TestScanClaudeCompactForeignOnlyIsNoBoundary(t *testing.T) {
	path := writeTranscript(t,
		`{"type":"system","subtype":"compact_boundary","isSidechain":true,"sessionId":"claude-fill","timestamp":"2026-11-26T11:00:00Z","compactMetadata":{"trigger":"manual","preTokens":900,"postTokens":90}}`,
		`{"type":"system","subtype":"compact_boundary","sessionId":"other-session","timestamp":"2026-11-26T12:00:00Z","compactMetadata":{"trigger":"manual","preTokens":800,"postTokens":80}}`,
	)
	got, err := scanClaudeCompact(path, fillSession, testMaxBytes)
	if err != nil || got != nil {
		t.Fatalf("scan = %+v err %v, want no boundary", got, err)
	}
}

// Issue 1303 M3: the read is bounded to the transcript's tail. A boundary before the
// tail is not found; a bound that covers the whole file finds it.
func TestScanClaudeCompactReadsOnlyTheTail(t *testing.T) {
	filler := `{"type":"assistant","timestamp":"2026-11-26T10:30:00Z","message":{"model":"claude-opus-5-5","content":"` + strings.Repeat("x", 200) + `"}}`
	lines := []string{`{"type":"system","subtype":"compact_boundary","sessionId":"claude-fill","timestamp":"2026-11-26T10:05:00Z","compactMetadata":{"trigger":"manual","preTokens":100,"postTokens":10}}`}
	for range 20 {
		lines = append(lines, filler)
	}
	path := writeTranscript(t, lines...)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := scanClaudeCompact(path, fillSession, 1000); err != nil || got != nil {
		t.Errorf("tail-only scan = %+v err %v, want no boundary before the tail", got, err)
	}
	if got, err := scanClaudeCompact(path, fillSession, info.Size()); err != nil || got == nil || got.PostTokens != 10 {
		t.Errorf("full-size scan = %+v err %v, want the boundary", got, err)
	}
}

func TestScanClaudeCompactRejectsNonPositiveBound(t *testing.T) {
	path := writeTranscript(t, `{"type":"system","subtype":"compact_boundary","timestamp":"2026-11-26T10:05:00Z","compactMetadata":{"trigger":"manual","preTokens":100,"postTokens":10}}`)
	for _, bound := range []int64{0, -1} {
		if _, err := scanClaudeCompact(path, fillSession, bound); err == nil {
			t.Errorf("scan with bound %d succeeded, want an error (no unbounded read)", bound)
		}
	}
}
