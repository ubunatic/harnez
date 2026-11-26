package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
)

// Issue 1303 M1e: Claude writes its compact boundary after PostCompact returns, so
// the hook never waits. The collector fills the PostCompact row from a re-read of
// the transcript on the spec schedule (claude_compact.fill_retry). These tests use
// a fake timer, so none of them sleeps or runs a timer after it ends.

const (
	claudeBoundaryLineFmt      = `{"type":"system","subtype":"compact_boundary","timestamp":%q,"compactMetadata":{"trigger":"manual","preTokens":%d,"postTokens":%d}}`
	claudeAssistantLineFixture = `{"type":"assistant","timestamp":"2026-11-26T10:00:00Z","message":{"model":"claude-opus-5-5"}}`
	fillSession                = "claude-fill"
)

// fakeFill is one armed fill timer: its id, delay and the callback it would run.
type fakeFill struct {
	id    int
	delay time.Duration
	fn    func()
}

// fakeFills replaces claudeFillAfter and the timer registry. Armed timers wait in
// pending until a test fires them; a stop removes one from pending and counts as a
// stop. armed lists every delay in arm order; history keeps every callback.
type fakeFills struct {
	pending []fakeFill
	history []fakeFill
	armed   []time.Duration
	fires   int
	stops   int
	nextID  int
}

func installFakeFills(t *testing.T) *fakeFills {
	t.Helper()
	f := &fakeFills{}
	prev, prevTimers := claudeFillAfter, claudeFills
	claudeFills = &claudeFillTimers{armed: map[string]armedFill{}}
	claudeFillAfter = func(d time.Duration, fn func()) func() {
		f.nextID++
		id := f.nextID
		f.armed = append(f.armed, d)
		f.pending = append(f.pending, fakeFill{id, d, fn})
		f.history = append(f.history, fakeFill{id, d, fn})
		return func() {
			n := len(f.pending)
			f.pending = slices.DeleteFunc(f.pending, func(p fakeFill) bool { return p.id == id })
			if len(f.pending) != n {
				f.stops++
			}
		}
	}
	t.Cleanup(func() { claudeFillAfter, claudeFills = prev, prevTimers })
	return f
}

// fireNext runs the oldest armed timer and reports whether one was armed.
func (f *fakeFills) fireNext() bool {
	if len(f.pending) == 0 {
		return false
	}
	next := f.pending[0]
	f.pending = f.pending[1:]
	f.fires++
	next.fn()
	return true
}

// drain fires timers until none is armed. A test that never settles fails.
func (f *fakeFills) drain(t *testing.T) {
	t.Helper()
	for n := 0; f.fireNext(); n++ {
		if n > 20 {
			t.Fatal("fill timers keep re-arming past any schedule")
		}
	}
}

func appendTranscriptLine(t *testing.T, path, line string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		t.Fatal(err)
	}
}

// claudePostCompactRaw is a PostCompact payload. Claude's payload carries
// compact_summary; a Codex payload does not, and only Claude's row is filled.
func claudePostCompactRaw(path string, claude bool) []byte {
	extra := ""
	if claude {
		extra = `,"compact_summary":"summary"`
	}
	return []byte(fmt.Sprintf(`{"session_id":"claude-wait","transcript_path":%q,"hook_event_name":"PostCompact","trigger":"manual"%s}`, path, extra))
}

// fillRaw is a Claude hook payload for fillSession. The PostCompact one carries
// compact_summary, as Claude's does.
func fillRaw(event, path string) []byte {
	extra := ""
	if event == "PostCompact" {
		extra = `,"compact_summary":"summary"`
	}
	return []byte(fmt.Sprintf(`{"session_id":%q,"transcript_path":%q,"cwd":%q,"hook_event_name":%q,"prompt_id":"p-fill","trigger":"manual"%s}`,
		fillSession, path, filepath.Dir(path), event, extra))
}

func compactFromEvidence(t *testing.T, prepared []byte) *claudeCompact {
	t.Helper()
	var hook struct {
		Context codexEvidence `json:"harnez_context"`
	}
	if err := json.Unmarshal(prepared, &hook); err != nil {
		t.Fatal(err)
	}
	return hook.Context.Compact
}

// openCompactDB opens a scratch telemetry DB and closes it at cleanup.
func openCompactDB(t *testing.T) (string, *telemetry.DB) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return dbPath, db
}

func openRawCompactDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rawDB.Close() })
	return rawDB
}

// postcompactRow returns the newest postcompact row's total tokens and model.
func postcompactRow(t *testing.T, dbPath, sessionID string) (sql.NullInt64, string) {
	t.Helper()
	var total sql.NullInt64
	var model string
	err := openRawCompactDB(t, dbPath).QueryRow(`SELECT total_tokens, model FROM compaction_events WHERE session_id = ? AND event_type = 'postcompact' ORDER BY id DESC LIMIT 1`, sessionID).Scan(&total, &model)
	if err != nil {
		t.Fatalf("postcompact row: %v", err)
	}
	return total, model
}

// snapshotCount counts the session's token snapshots from one source.
func snapshotCount(t *testing.T, dbPath, sessionID, source string) int {
	t.Helper()
	var n int
	if err := openRawCompactDB(t, dbPath).QueryRow(`SELECT COUNT(*) FROM token_snapshots WHERE session_id = ? AND source = ?`, sessionID, source).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// filledNotes returns the notes whose outcome is applied.
func filledNotes(t *testing.T, dbPath string) []compactDiagNote {
	t.Helper()
	var out []compactDiagNote
	for _, note := range compactDiagNotes(t, dbPath) {
		if note.Reason == reasonApplied {
			out = append(out, note)
		}
	}
	return out
}

// lastNote returns the newest diagnostic note.
func lastNote(t *testing.T, dbPath string) compactDiagNote {
	t.Helper()
	notes := compactDiagNotes(t, dbPath)
	if len(notes) == 0 {
		t.Fatal("no diagnostic note written")
	}
	return notes[len(notes)-1]
}

// The boundary lands after the first retry, which fills the row.
func TestClaudeFillLandsOnFirstRetry(t *testing.T) {
	fills := isolateCompactEnv(t)
	dbPath, db := openCompactDB(t)
	transcript := writeTranscript(t, claudeAssistantLineFixture)
	processPrepared(t, db, fillRaw("PreCompact", transcript))
	processPrepared(t, db, fillRaw("PostCompact", transcript))

	if total, _ := postcompactRow(t, dbPath, fillSession); total.Valid {
		t.Fatal("postcompact row filled before its boundary was written")
	}
	if got := lastNote(t, dbPath); got.Reason != reasonNoBoundary || got.FilledAfter != "" {
		t.Errorf("ingest note reason %q filled_after %q, want %q and empty", got.Reason, got.FilledAfter, reasonNoBoundary)
	}
	if !slices.Equal(fills.armed, []time.Duration{2 * time.Second}) {
		t.Fatalf("armed %v, want one 2s retry", fills.armed)
	}

	appendTranscriptLine(t, transcript, fmt.Sprintf(claudeBoundaryLineFmt, time.Now().UTC().Format(time.RFC3339Nano), m1dPreTokens, m1dPostTokens))
	if !fills.fireNext() {
		t.Fatal("no retry armed")
	}

	total, model := postcompactRow(t, dbPath, fillSession)
	if !total.Valid || total.Int64 != m1dPostTokens {
		t.Errorf("postcompact total_tokens = %v, want %d", total, m1dPostTokens)
	}
	if model != "claude-opus-5-5" {
		t.Errorf("postcompact model = %q, want claude-opus-5-5", model)
	}
	got := lastNote(t, dbPath)
	if got.Reason != reasonApplied || got.FilledAfter != "retry:1" {
		t.Errorf("fill note reason %q filled_after %q, want %q and retry:1", got.Reason, got.FilledAfter, reasonApplied)
	}
	if got.PreCompactAt == nil || got.BoundaryAt == nil || !got.BoundaryAt.After(*got.PreCompactAt) {
		t.Errorf("fill note precompact_at %v boundary_at %v, want both with the boundary after", got.PreCompactAt, got.BoundaryAt)
	}
	if len(fills.pending) != 0 {
		t.Errorf("a retry was armed after the row filled: %v", fills.armed)
	}
}

// The boundary never lands: exactly the scheduled retries run, then the row stays
// empty and the pending record is dropped.
func TestClaudeFillRunsScheduledRetriesThenStops(t *testing.T) {
	fills := isolateCompactEnv(t)
	dbPath, db := openCompactDB(t)
	transcript := writeTranscript(t, claudeAssistantLineFixture)
	processPrepared(t, db, fillRaw("PreCompact", transcript))
	processPrepared(t, db, fillRaw("PostCompact", transcript))
	fills.drain(t)

	want := []time.Duration{2 * time.Second, 10 * time.Second, 60 * time.Second}
	if !slices.Equal(fills.armed, want) {
		t.Errorf("armed %v, want %v", fills.armed, want)
	}
	if fills.fires != len(want) {
		t.Errorf("retries run %d, want %d", fills.fires, len(want))
	}
	if total, _ := postcompactRow(t, dbPath, fillSession); total.Valid {
		t.Errorf("postcompact row filled with no boundary: %v", total)
	}
	if got := lastNote(t, dbPath); got.Reason != reasonNoBoundary || got.FilledAfter != "" {
		t.Errorf("final note reason %q filled_after %q, want %q and empty", got.Reason, got.FilledAfter, reasonNoBoundary)
	}
	if n := snapshotCount(t, dbPath, fillSession, "postcompact"); n != 0 {
		t.Errorf("postcompact snapshots = %d, want 0", n)
	}
	if _, ok := readClaudeFillPending(fillSession); ok {
		t.Error("pending fill survived its last retry")
	}
}

// Only an older boundary is in the transcript: it is never applied, and the note
// says so.
func TestClaudeFillNeverAppliesOlderBoundary(t *testing.T) {
	fills := isolateCompactEnv(t)
	dbPath, db := openCompactDB(t)
	old := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	transcript := writeTranscript(t, claudeAssistantLineFixture, fmt.Sprintf(claudeBoundaryLineFmt, old, m1dOldPre, m1dOldPost))
	processPrepared(t, db, fillRaw("PreCompact", transcript))
	processPrepared(t, db, fillRaw("PostCompact", transcript))
	if got := lastNote(t, dbPath); got.Reason != reasonStaleBoundary || got.BoundaryAt == nil {
		t.Errorf("ingest note reason %q boundary_at %v, want %q with the old boundary's time", got.Reason, got.BoundaryAt, reasonStaleBoundary)
	}
	fills.drain(t)

	if total, _ := postcompactRow(t, dbPath, fillSession); total.Valid {
		t.Errorf("postcompact row filled from an older boundary: %v", total)
	}
	if got := lastNote(t, dbPath); got.Reason != reasonStaleBoundary || got.FilledAfter != "" {
		t.Errorf("final note reason %q filled_after %q, want %q and empty", got.Reason, got.FilledAfter, reasonStaleBoundary)
	}
}

// A repeated timer callback and a later event must not write the row again.
func TestClaudeFillWritesOnceAcrossRepeatedRetries(t *testing.T) {
	fills := isolateCompactEnv(t)
	dbPath, db := openCompactDB(t)
	transcript := writeTranscript(t, claudeAssistantLineFixture)
	processPrepared(t, db, fillRaw("PreCompact", transcript))
	processPrepared(t, db, fillRaw("PostCompact", transcript))
	appendTranscriptLine(t, transcript, fmt.Sprintf(claudeBoundaryLineFmt, time.Now().UTC().Format(time.RFC3339Nano), m1dPreTokens, m1dPostTokens))
	pending, ok := readClaudeFillPending(fillSession)
	if !ok {
		t.Fatal("no pending fill before the first retry")
	}
	if !fills.fireNext() {
		t.Fatal("no retry armed")
	}
	// Issue 1303 M3: restore the pending record, so the repeat reaches the row
	// guard (total_tokens IS NULL) instead of returning at the missing record.
	if err := writeClaudeFillPending(pending); err != nil {
		t.Fatal(err)
	}

	fills.history[0].fn()
	processPrepared(t, db, fillRaw("SessionStart", transcript))

	if n := snapshotCount(t, dbPath, fillSession, "postcompact"); n != 1 {
		t.Errorf("postcompact snapshots = %d, want 1", n)
	}
	if n := snapshotCount(t, dbPath, fillSession, "precompact"); n != 1 {
		t.Errorf("precompact snapshots = %d, want 1", n)
	}
	if n := len(filledNotes(t, dbPath)); n != 1 {
		t.Errorf("applied notes = %d, want 1", n)
	}
	if _, ok := readClaudeFillPending(fillSession); ok {
		t.Error("the repeat left the pending record in place")
	}
}

// The collector restarts and its timers are gone. The next session event fills
// the row from the pending record on disk.
func TestClaudeFillSurvivesCollectorRestart(t *testing.T) {
	fills := isolateCompactEnv(t)
	dbPath, db := openCompactDB(t)
	transcript := writeTranscript(t, claudeAssistantLineFixture)
	processPrepared(t, db, fillRaw("PreCompact", transcript))
	processPrepared(t, db, fillRaw("PostCompact", transcript))
	appendTranscriptLine(t, transcript, fmt.Sprintf(claudeBoundaryLineFmt, time.Now().UTC().Format(time.RFC3339Nano), m1dPreTokens, m1dPostTokens))
	fills.pending = nil

	processPrepared(t, db, fillRaw("SessionStart", transcript))

	if total, _ := postcompactRow(t, dbPath, fillSession); !total.Valid || total.Int64 != m1dPostTokens {
		t.Errorf("postcompact total_tokens = %v, want %d after the next event", total, m1dPostTokens)
	}
	if got := lastNote(t, dbPath); got.Reason != reasonApplied || got.FilledAfter != "event" {
		t.Errorf("fill note reason %q filled_after %q, want %q and event", got.Reason, got.FilledAfter, reasonApplied)
	}
	if len(fills.pending) != 0 {
		t.Errorf("a retry was armed by the event fill: %v", fills.armed)
	}
}

// The hook reads the transcript once and arms nothing: the retries belong to the
// collector.
func TestPrepareClaudePostCompactReadsOnceAndArmsNothing(t *testing.T) {
	fills := isolateCompactEnv(t)
	path := writeTranscript(t, claudeAssistantLineFixture)
	prepared, err := PrepareCodexTelemetry(claudePostCompactRaw(path, true))
	if err != nil {
		t.Fatal(err)
	}
	if c := compactFromEvidence(t, prepared); c != nil {
		t.Errorf("compact = %+v, want none", c)
	}
	if diag := compactDiagFromEvidence(t, prepared); diag == nil || diag.Reason != reasonNoBoundary {
		t.Errorf("diagnostic %+v, want reason %q", diag, reasonNoBoundary)
	}
	if len(fills.armed) != 0 {
		t.Errorf("the hook armed fills %v, want none", fills.armed)
	}
}

// The pending record lives under the state dir and refuses a session id that
// could leave it.
func TestClaudeFillPendingStaysInStateDir(t *testing.T) {
	isolateCompactEnv(t)
	at := time.Now().UTC().Truncate(time.Millisecond)
	p := claudeFillPending{SessionID: "fill-state", TranscriptPath: "/transcript.jsonl", PreCompactAt: at, RowID: 7, BoundaryID: 3, Reason: reasonNoBoundary}
	if err := writeClaudeFillPending(p); err != nil {
		t.Fatal(err)
	}
	got, ok := readClaudeFillPending("fill-state")
	if !ok || got.RowID != 7 || got.BoundaryID != 3 || !got.PreCompactAt.Equal(at) {
		t.Errorf("pending = %+v ok=%v, want the written record", got, ok)
	}
	if _, ok := readClaudeFillPending("no-such-session"); ok {
		t.Error("missing pending record reported as present")
	}
	path, err := claudeFillPendingPath("fill-state")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, os.Getenv("XDG_STATE_HOME")) {
		t.Errorf("pending path %q is outside the state dir", path)
	}
	removeClaudeFillPending("fill-state")
	if _, ok := readClaudeFillPending("fill-state"); ok {
		t.Error("removed pending record still readable")
	}
}

func TestClaudeFillPendingRefusesUnsafeSessionID(t *testing.T) {
	isolateCompactEnv(t)
	for _, id := range []string{"", ".", "..", "../escape", "a/b", filepath.Join("x", "y")} {
		if _, err := claudeFillPendingPath(id); err == nil {
			t.Errorf("session id %q accepted", id)
		}
		if err := writeClaudeFillPending(claudeFillPending{SessionID: id}); err == nil {
			t.Errorf("write for session id %q succeeded", id)
		}
		if _, ok := readClaudeFillPending(id); ok {
			t.Errorf("read for session id %q reported a record", id)
		}
	}
}

// TestClaudeFillRollsBackPartialWriteAndNextAttemptFills injects a failure after the
// row update (issue 1303 M2). No partial state remains: the row stays empty, the
// snapshot and economics counts are unchanged and the pending record survives. The
// next attempt, with no fault, fills the row.
func TestClaudeFillRollsBackPartialWriteAndNextAttemptFills(t *testing.T) {
	isolateCompactEnv(t)
	dbPath, db := openCompactDB(t)
	transcript := writeTranscript(t, claudeAssistantLineFixture)
	processPrepared(t, db, fillRaw("PreCompact", transcript))
	processPrepared(t, db, fillRaw("PostCompact", transcript))
	appendTranscriptLine(t, transcript, fmt.Sprintf(claudeBoundaryLineFmt, time.Now().UTC().Format(time.RFC3339Nano), m1dPreTokens, m1dPostTokens))

	snapshotsBefore := snapshotCount(t, dbPath, fillSession, "precompact") + snapshotCount(t, dbPath, fillSession, "postcompact")
	economicsBefore := filledEconomicsCount(t, dbPath, fillSession)

	claudeFillFault = func(stage string) error {
		if stage != "after-row" {
			return nil
		}
		return errors.New("injected failure after row update")
	}
	t.Cleanup(func() { claudeFillFault = nil })
	p, ok := readClaudeFillPending(fillSession)
	if !ok {
		t.Fatal("no pending fill before the attempt")
	}
	if _, err := claudeFillStep(db, p, "event"); err == nil {
		t.Fatal("injected failure was not returned")
	}
	if total, _ := postcompactRow(t, dbPath, fillSession); total.Valid {
		t.Fatalf("failed attempt left total_tokens %d on the row", total.Int64)
	}
	if got := snapshotCount(t, dbPath, fillSession, "precompact") + snapshotCount(t, dbPath, fillSession, "postcompact"); got != snapshotsBefore {
		t.Fatalf("failed attempt changed snapshot rows: %d, want %d", got, snapshotsBefore)
	}
	if got := filledEconomicsCount(t, dbPath, fillSession); got != economicsBefore {
		t.Fatalf("failed attempt changed economics rows: %d, want %d", got, economicsBefore)
	}
	if _, ok := readClaudeFillPending(fillSession); !ok {
		t.Fatal("failed attempt dropped the pending record; the next attempt could not fill")
	}

	claudeFillFault = nil
	filled, err := claudeFillStep(db, p, "retry:1")
	if err != nil || !filled {
		t.Fatalf("next attempt filled=%v err=%v, want filled", filled, err)
	}
	if total, _ := postcompactRow(t, dbPath, fillSession); !total.Valid || total.Int64 != m1dPostTokens {
		t.Fatalf("row total_tokens = %v, want %d after the next attempt", total, m1dPostTokens)
	}
	if got := filledEconomicsCount(t, dbPath, fillSession); got <= economicsBefore {
		t.Fatal("next attempt did not reconcile the economics row")
	}
}

// filledEconomicsCount counts the session's economics rows that carry a compaction
// cost, so a reconciled row is visible even though the row count is unchanged.
func filledEconomicsCount(t *testing.T, dbPath, sessionID string) int {
	t.Helper()
	db := openRawCompactDB(t, dbPath)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM compaction_economics WHERE session_id = ? AND compaction_cost_micros IS NOT NULL`, sessionID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
