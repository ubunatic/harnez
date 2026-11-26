package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
)

// Issue 1303 M1d: the live failure. A session that compacts twice in a few
// minutes already has an older complete boundary in its transcript when the
// second PostCompact runs. Claude has not flushed the new boundary yet, so the
// first read finds the old one. It is inside boundary_window, so the hook takes
// it as fresh and never waits. The collector then sees it predates this
// compaction's PreCompact row and marks it stale. This test round-trips a
// real-shaped payload through the record JSON the socket uses, and writes the
// new boundary during the first wait poll (the clock is faked; no sleeps).
const (
	m1dSessionID  = "claude-m1d"
	m1dPreTokens  = 157210
	m1dPostTokens = 11106
	m1dOldPre     = 80748
	m1dOldPost    = 16136
)

func m1dPreCompactRaw(path string) []byte {
	return []byte(fmt.Sprintf(`{"session_id":%q,"transcript_path":%q,"cwd":%q,"hook_event_name":"PreCompact","prompt_id":"p-1","trigger":"manual","custom_instructions":%q}`,
		m1dSessionID, path, filepath.Dir(path), diagInstructionsSentinel))
}

func m1dPostCompactRaw(path string) []byte {
	return []byte(fmt.Sprintf(`{"session_id":%q,"transcript_path":%q,"cwd":%q,"hook_event_name":"PostCompact","prompt_id":"p-1","trigger":"manual","compact_summary":%q}`,
		m1dSessionID, path, filepath.Dir(path), diagSummarySentinel))
}

// wireRecord sends a prepared payload the way the socket does: wrapped in a
// telemetry record, marshalled, then decoded by the collector.
func wireRecord(t *testing.T, prepared []byte) []byte {
	t.Helper()
	record, err := NewTelemetryRecord("codex", prepared)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var decoded TelemetryRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.Payload
}

// The second compaction's boundary must reach its PostCompact row, even though
// an older boundary is still inside the window when PostCompact first reads.
func TestCompactRoundTripRealShapeAppliesBoundary(t *testing.T) {
	isolateCompactEnv(t)
	start := time.Now().UTC()
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	// The earlier compaction, eight minutes ago: inside boundary_window (10m).
	earlier := time.Now().Add(-8 * time.Minute).UTC().Format(time.RFC3339Nano)
	transcript := writeTranscript(t, claudeAssistantLineFixture, fmt.Sprintf(claudeBoundaryLineFmt, earlier, m1dOldPre, m1dOldPost))

	// Claude flushes the new boundary during the first wait poll.
	fakeCompactClock(t, start, func(call int) {
		if call == 1 {
			stamp := time.Now().UTC().Format(time.RFC3339Nano)
			appendTranscriptLine(t, transcript, fmt.Sprintf(claudeBoundaryLineFmt, stamp, m1dPreTokens, m1dPostTokens))
		}
	})

	prepared, err := PrepareCodexTelemetry(m1dPreCompactRaw(transcript))
	if err != nil {
		t.Fatal(err)
	}
	if err := ProcessCodexTelemetryTx(db, wireRecord(t, prepared)); err != nil {
		t.Fatal(err)
	}

	prepared, err = PrepareCodexTelemetry(m1dPostCompactRaw(transcript))
	if err != nil {
		t.Fatal(err)
	}
	if err := ProcessCodexTelemetryTx(db, wireRecord(t, prepared)); err != nil {
		t.Fatal(err)
	}

	notes := compactDiagNotes(t, dbPath)
	last := notes[len(notes)-1]
	if last.Reason != reasonApplied {
		t.Errorf("note reason = %q, want %q (hook %+v)", last.Reason, reasonApplied, last.Hook)
	}
	if last.Hook == nil || !slices.Contains(last.Hook.Keys, "compact_summary") {
		t.Errorf("hook keys %+v, want compact_summary among them", last.Hook)
	}
	if last.PreCompactAt == nil || last.BoundaryAt == nil {
		t.Fatalf("note carries no precompact_at/boundary_at: %+v", last)
	}
	if !last.BoundaryAt.After(*last.PreCompactAt) {
		t.Errorf("boundary_at %v is not after precompact_at %v", last.BoundaryAt, last.PreCompactAt)
	}

	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	var total sql.NullInt64
	var model string
	if err := rawDB.QueryRow(`SELECT total_tokens, model FROM compaction_events WHERE session_id = ? AND event_type = 'postcompact' ORDER BY id DESC LIMIT 1`, m1dSessionID).Scan(&total, &model); err != nil {
		t.Fatalf("postcompact row: %v", err)
	}
	if !total.Valid || total.Int64 != m1dPostTokens {
		t.Errorf("postcompact total_tokens = %v, want %d", total, m1dPostTokens)
	}
	if model != "claude-opus-5-5" {
		t.Errorf("postcompact model = %q, want claude-opus-5-5", model)
	}
}
