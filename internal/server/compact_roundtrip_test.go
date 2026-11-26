package server

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
)

// Issue 1303 M1e: a real-shaped Claude compaction round-trips through the record
// JSON the socket uses. At ingest the transcript holds only an older boundary, so
// the PostCompact row stays empty and a fill is armed. Claude writes the new
// boundary after PostCompact returns; the first scheduled re-read fills the row.
// The fill timer is faked, so the test never sleeps.
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

// processWire prepares a raw hook payload and feeds it through the socket wire format.
func processWire(t *testing.T, db *telemetry.DB, raw []byte) {
	t.Helper()
	prepared, err := PrepareCodexTelemetry(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := ProcessCodexTelemetryTx(db, wireRecord(t, prepared)); err != nil {
		t.Fatal(err)
	}
}

// The second compaction's boundary is missing at ingest, so the row stays empty
// and one fill is armed. Once the boundary lands, the first retry fills it.
func TestCompactRoundTripStaleThenFilled(t *testing.T) {
	fills := isolateCompactEnv(t)
	dbPath, db := openCompactDB(t)

	// The earlier compaction, eight minutes ago. It predates this PreCompact.
	earlier := time.Now().Add(-8 * time.Minute).UTC().Format(time.RFC3339Nano)
	transcript := writeTranscript(t, claudeAssistantLineFixture, fmt.Sprintf(claudeBoundaryLineFmt, earlier, m1dOldPre, m1dOldPost))

	processWire(t, db, m1dPreCompactRaw(transcript))
	processWire(t, db, m1dPostCompactRaw(transcript))

	ingest := lastNote(t, dbPath)
	if ingest.Reason != reasonStaleBoundary {
		t.Errorf("ingest note reason = %q, want %q (hook %+v)", ingest.Reason, reasonStaleBoundary, ingest.Hook)
	}
	if ingest.Hook == nil || !slices.Contains(ingest.Hook.Keys, "compact_summary") {
		t.Errorf("hook keys %+v, want compact_summary among them", ingest.Hook)
	}
	if ingest.PreCompactAt == nil || ingest.BoundaryAt == nil {
		t.Fatalf("ingest note carries no precompact_at/boundary_at: %+v", ingest)
	}
	if ingest.BoundaryAt.After(*ingest.PreCompactAt) {
		t.Errorf("stale boundary_at %v is after precompact_at %v", ingest.BoundaryAt, ingest.PreCompactAt)
	}
	if total, _ := postcompactRow(t, dbPath, m1dSessionID); total.Valid {
		t.Fatal("postcompact row filled from a boundary older than its PreCompact")
	}
	if len(fills.armed) != 1 {
		t.Fatalf("armed fills %v, want one retry", fills.armed)
	}

	// Claude writes the new boundary after PostCompact returned.
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	appendTranscriptLine(t, transcript, fmt.Sprintf(claudeBoundaryLineFmt, stamp, m1dPreTokens, m1dPostTokens))
	if !fills.fireNext() {
		t.Fatal("no fill timer armed")
	}

	filled := lastNote(t, dbPath)
	if filled.Reason != reasonApplied || filled.FilledAfter != "retry:1" {
		t.Errorf("fill note reason %q filled_after %q, want %q retry:1", filled.Reason, filled.FilledAfter, reasonApplied)
	}
	if filled.Hook == nil || !slices.Contains(filled.Hook.Keys, "compact_summary") {
		t.Errorf("fill note hook keys %+v, want compact_summary among them", filled.Hook)
	}
	if filled.PreCompactAt == nil || filled.BoundaryAt == nil {
		t.Fatalf("fill note carries no precompact_at/boundary_at: %+v", filled)
	}
	if !filled.BoundaryAt.After(*filled.PreCompactAt) {
		t.Errorf("boundary_at %v is not after precompact_at %v", filled.BoundaryAt, filled.PreCompactAt)
	}

	total, model := postcompactRow(t, dbPath, m1dSessionID)
	if !total.Valid || total.Int64 != m1dPostTokens {
		t.Errorf("postcompact total_tokens = %v, want %d", total, m1dPostTokens)
	}
	if model != "claude-opus-5-5" {
		t.Errorf("postcompact model = %q, want claude-opus-5-5", model)
	}
}
