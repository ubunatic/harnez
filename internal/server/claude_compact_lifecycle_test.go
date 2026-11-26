package server

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Issue 1303 M3: the fill lifecycle. Timers belong to the collector and stop on
// shutdown; a second PostCompact replaces the first timer; one lock serializes a
// timer's attempt with an event's attempt; a corrupt pending record is removed and
// noted; a stale record is pruned at start. Fake timers keep every test free of
// sleeps.

// The comparison is at millisecond precision: a boundary in the same millisecond as
// the PreCompact row is the same compaction, and one millisecond earlier is not.
func TestClaudeBoundaryReasonComparesAtMillisecond(t *testing.T) {
	pre := time.Date(2026, 11, 26, 12, 0, 0, 123999999, time.UTC)
	sameMS := &claudeCompact{Timestamp: time.Date(2026, 11, 26, 12, 0, 0, 123000000, time.UTC)}
	if got := claudeBoundaryReason(sameMS, pre); got != reasonApplied {
		t.Errorf("boundary in the same ms as PreCompact = %q, want %q", got, reasonApplied)
	}
	sameMSLater := &claudeCompact{Timestamp: time.Date(2026, 11, 26, 12, 0, 0, 123456789, time.UTC)}
	if got := claudeBoundaryReason(sameMSLater, pre); got != reasonApplied {
		t.Errorf("boundary 456789ns into the same ms = %q, want %q", got, reasonApplied)
	}
	earlierMS := &claudeCompact{Timestamp: time.Date(2026, 11, 26, 12, 0, 0, 122999999, time.UTC)}
	if got := claudeBoundaryReason(earlierMS, pre); got != reasonStaleBoundary {
		t.Errorf("boundary one ms earlier = %q, want %q", got, reasonStaleBoundary)
	}
}

// A boundary of another session is not this session's compaction at fill time, even
// when it is newer than the PreCompact row.
func TestClaudeFillIgnoresForeignSessionBoundary(t *testing.T) {
	fills := isolateCompactEnv(t)
	dbPath, db := openCompactDB(t)
	transcript := writeTranscript(t, claudeAssistantLineFixture)
	processPrepared(t, db, fillRaw("PreCompact", transcript))
	processPrepared(t, db, fillRaw("PostCompact", transcript))
	foreign := fmt.Sprintf(`{"type":"system","subtype":"compact_boundary","sessionId":"other-session","timestamp":%q,"compactMetadata":{"trigger":"manual","preTokens":%d,"postTokens":%d}}`,
		time.Now().UTC().Format(time.RFC3339Nano), m1dPreTokens, m1dPostTokens)
	appendTranscriptLine(t, transcript, foreign)
	fills.drain(t)

	if total, _ := postcompactRow(t, dbPath, fillSession); total.Valid {
		t.Errorf("postcompact row filled from another session's boundary: %v", total)
	}
	if got := lastNote(t, dbPath); got.Reason != reasonNoBoundary {
		t.Errorf("final note reason %q, want %q", got.Reason, reasonNoBoundary)
	}
}

// Stopping the collector stops every armed fill timer and leaves none pending.
func TestStopClaudeFillsOnShutdownStopsEveryTimer(t *testing.T) {
	fills := isolateCompactEnv(t)
	claudeFills.arm("sess-a", 2*time.Second, func() {})
	claudeFills.arm("sess-b", 2*time.Second, func() {})
	if len(fills.pending) != 2 {
		t.Fatalf("pending timers = %d, want 2 armed before shutdown", len(fills.pending))
	}

	StopClaudeFills()

	if len(fills.pending) != 0 {
		t.Errorf("pending timers after shutdown = %d, want 0", len(fills.pending))
	}
	if fills.stops != 2 {
		t.Errorf("stops = %d, want 2", fills.stops)
	}
	if len(claudeFills.armed) != 0 {
		t.Errorf("registry still holds %d sessions after shutdown", len(claudeFills.armed))
	}
}

// A second PostCompact for a session stops the earlier timer: one timer per session.
func TestClaudeFillSecondPostCompactReplacesTimer(t *testing.T) {
	fills := isolateCompactEnv(t)
	_, db := openCompactDB(t)
	transcript := writeTranscript(t, claudeAssistantLineFixture)
	processPrepared(t, db, fillRaw("PreCompact", transcript))
	processPrepared(t, db, fillRaw("PostCompact", transcript))
	processPrepared(t, db, fillRaw("PostCompact", transcript))

	if len(fills.armed) != 2 {
		t.Fatalf("armed %v, want two arms (one per PostCompact)", fills.armed)
	}
	if fills.stops != 1 {
		t.Errorf("stops = %d, want 1: the second PostCompact stops the first timer", fills.stops)
	}
	if len(fills.pending) != 1 {
		t.Errorf("pending timers = %d, want 1", len(fills.pending))
	}
}

// A fill attempt and an event attempt never run together. While a timer's attempt
// holds the lock, the event skips its attempt and leaves the record for the timer.
func TestClaudeFillOnEventSkipsWhileTimerAttemptRuns(t *testing.T) {
	fills := isolateCompactEnv(t)
	dbPath, db := openCompactDB(t)
	transcript := writeTranscript(t, claudeAssistantLineFixture)
	processPrepared(t, db, fillRaw("PreCompact", transcript))
	processPrepared(t, db, fillRaw("PostCompact", transcript))
	appendTranscriptLine(t, transcript, fmt.Sprintf(claudeBoundaryLineFmt, time.Now().UTC().Format(time.RFC3339Nano), m1dPreTokens, m1dPostTokens))

	claudeFillMu.Lock()
	err := claudeFillOnEvent(db, fillSession)
	claudeFillMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if total, _ := postcompactRow(t, dbPath, fillSession); total.Valid {
		t.Fatalf("event attempt filled the row while a timer attempt held the lock: %v", total)
	}
	if _, ok := readClaudeFillPending(fillSession); !ok {
		t.Fatal("the skipped event attempt dropped the pending record")
	}
	if !fills.fireNext() {
		t.Fatal("no retry armed")
	}
	if total, _ := postcompactRow(t, dbPath, fillSession); !total.Valid || total.Int64 != m1dPostTokens {
		t.Errorf("timer attempt left total_tokens %v, want %d", total, m1dPostTokens)
	}
}

// A pending record that cannot be decoded is removed, and the removal is noted.
func TestClaudeFillCorruptPendingIsRemovedAndNoted(t *testing.T) {
	isolateCompactEnv(t)
	dbPath, db := openCompactDB(t)
	path, err := claudeFillPendingPath(fillSession)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := claudeFillOnEvent(db, fillSession); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("corrupt pending record still on disk (stat err %v)", err)
	}
	if got := lastNote(t, dbPath); got.Reason != reasonPendingCorrupt || got.SessionID != fillSession {
		t.Errorf("note reason %q session %q, want %q for %s", got.Reason, got.SessionID, reasonPendingCorrupt, fillSession)
	}
}

// At collector start, records older than the fill schedule and corrupt records are
// removed; a fresh record is kept. With an empty state dir the prune does nothing.
func TestPruneClaudeFillPendingDropsStaleAndCorrupt(t *testing.T) {
	isolateCompactEnv(t)
	dbPath, _ := openCompactDB(t)
	if err := PruneClaudeFillPending(dbPath, time.Now()); err != nil {
		t.Fatalf("prune with no state dir: %v", err)
	}

	now := time.Now().UTC()
	stale := claudeFillPending{SessionID: "stale-s", TranscriptPath: "/t.jsonl", PreCompactAt: now.Add(-3 * time.Hour), RowID: 1, BoundaryID: 1, ArmedAt: now.Add(-3 * time.Hour)}
	fresh := claudeFillPending{SessionID: "fresh-s", TranscriptPath: "/t.jsonl", PreCompactAt: now, RowID: 2, BoundaryID: 2, ArmedAt: now}
	if err := writeClaudeFillPending(stale); err != nil {
		t.Fatal(err)
	}
	if err := writeClaudeFillPending(fresh); err != nil {
		t.Fatal(err)
	}
	badPath, err := claudeFillPendingPath("bad-s")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(badPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badPath, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := PruneClaudeFillPending(dbPath, now); err != nil {
		t.Fatal(err)
	}
	if _, ok := readClaudeFillPending("stale-s"); ok {
		t.Error("stale pending record survived the prune")
	}
	if _, ok := readClaudeFillPending("fresh-s"); !ok {
		t.Error("fresh pending record was pruned")
	}
	if _, err := os.Stat(badPath); !os.IsNotExist(err) {
		t.Errorf("corrupt pending record survived the prune (stat err %v)", err)
	}
	if got := lastNote(t, dbPath); got.Reason != reasonPendingCorrupt || got.SessionID != "bad-s" {
		t.Errorf("prune note reason %q session %q, want %q for bad-s", got.Reason, got.SessionID, reasonPendingCorrupt)
	}
}
