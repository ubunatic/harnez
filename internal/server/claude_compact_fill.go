package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ubunatic.com/harnez/internal/hookdispatch"
	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/xdgpath"
)

// Issue 1303 M1e: Claude writes its compact boundary after PostCompact returns, so
// the hook cannot read it. The collector fills the PostCompact row later. It re-reads
// the transcript on the claude_compact.fill_retry schedule, and each later session
// event gives a still-pending fill one attempt. A pending record on disk lets a
// restarted collector finish the fill. Only a boundary at or after the session's
// PreCompact time is applied, so an older boundary never fills the row.

// claudeFillAfter arms one fill timer and returns its stop. Tests replace it, so no
// test sleeps.
var claudeFillAfter = func(d time.Duration, fn func()) func() {
	t := time.AfterFunc(d, fn)
	return func() { t.Stop() }
}

// claudeFillMu serializes fill attempts (issue 1303 M3). A timer's attempt takes it
// with Lock. An event's attempt only TryLocks: it runs inside the event's write
// transaction, so waiting here could deadlock with the timer's SQLite write.
var claudeFillMu sync.Mutex

// claudeFills is the collector's registry of armed fill timers, one per session.
// StopClaudeFills stops them all on shutdown.
var claudeFills = &claudeFillTimers{armed: map[string]armedFill{}}

// armedFill is one armed timer. gen tells a fired timer whether it is still the
// session's current one.
type armedFill struct {
	gen  uint64
	stop func()
}

// claudeFillTimers holds at most one armed timer per session.
type claudeFillTimers struct {
	mu    sync.Mutex
	gen   uint64
	armed map[string]armedFill
}

// arm stops the session's previous timer and arms fn after d. Callers never hold mu
// while they take claudeFillMu, so the lock order is claudeFillMu then mu.
func (r *claudeFillTimers) arm(sessionID string, d time.Duration, fn func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if prev, ok := r.armed[sessionID]; ok {
		prev.stop()
		delete(r.armed, sessionID)
	}
	r.gen++
	gen := r.gen
	stop := claudeFillAfter(d, func() {
		r.release(sessionID, gen)
		fn()
	})
	r.armed[sessionID] = armedFill{gen: gen, stop: stop}
}

// release forgets a timer that has fired, unless the session has re-armed since.
func (r *claudeFillTimers) release(sessionID string, gen uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.armed[sessionID]; ok && a.gen == gen {
		delete(r.armed, sessionID)
	}
}

// stopAll stops every armed timer. Pending records stay on disk for the next start.
func (r *claudeFillTimers) stopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for sessionID, a := range r.armed {
		a.stop()
		delete(r.armed, sessionID)
	}
}

// StopClaudeFills stops every armed fill timer. The collector calls it on shutdown
// (issue 1303 M3). It does not clear the pending records: the next start finishes them.
func StopClaudeFills() {
	claudeFills.stopAll()
}

// claudeFillWindow is the total of the fill_retry schedule: a pending record older
// than this is past every re-read.
func claudeFillWindow() (time.Duration, error) {
	schedule, err := hookdispatch.LoadClaudeCompactFillRetry()
	if err != nil {
		return 0, err
	}
	var window time.Duration
	for _, d := range schedule {
		window += d
	}
	return window, nil
}

// claudeFillExpired reports whether a record armed at armedAt is older than window at
// now. Both times are compared at millisecond precision, the precision the record
// keeps across a restart (issue 1303 M3).
func claudeFillExpired(armedAt, now time.Time, window time.Duration) bool {
	return now.Truncate(time.Millisecond).Sub(armedAt.Truncate(time.Millisecond)) > window
}

// claudeFillPending is a PostCompact row that still waits for its boundary. It holds
// ids, times and the hook's key-name diagnostic only, never payload values.
type claudeFillPending struct {
	SessionID      string       `json:"session_id"`
	TranscriptPath string       `json:"transcript_path"`
	PreCompactAt   time.Time    `json:"precompact_at"`
	RowID          int64        `json:"row_id"`
	BoundaryID     int64        `json:"boundary_id"`
	Reason         string       `json:"reason"`
	ArmedAt        time.Time    `json:"armed_at"`
	Hook           *compactDiag `json:"hook,omitempty"`
}

// claudeBoundaryReason says whether compact belongs to the compaction that started
// at preAt: applied when it is not older, stale_boundary when it is, and
// no_boundary when there is none (issue 1303 M1d, M1e). The comparison is at
// millisecond precision (issue 1303 M3): a boundary in the PreCompact's millisecond
// is the same compaction.
func claudeBoundaryReason(compact *claudeCompact, preAt time.Time) string {
	switch {
	case compact == nil:
		return reasonNoBoundary
	case compact.Timestamp.Truncate(time.Millisecond).Before(preAt.Truncate(time.Millisecond)):
		return reasonStaleBoundary
	default:
		return reasonApplied
	}
}

// claudeFillPendingPath returns the pending record for a session. A session id
// that is not a plain file name is refused, so it cannot point outside the state dir.
func claudeFillPendingPath(sessionID string) (string, error) {
	if sessionID == "" || sessionID == "." || sessionID == ".." || filepath.Base(sessionID) != sessionID {
		return "", errors.New("claude fill: session id is not a plain file name")
	}
	return filepath.Join(xdgpath.StateHome(), "harnez", "claude-fill", sessionID+".json"), nil
}

// writeClaudeFillPending records a pending fill. The write is atomic: a temp file in
// the same directory is renamed over the record.
func writeClaudeFillPending(p claudeFillPending) error {
	path, err := claudeFillPendingPath(p.SessionID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pending-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// fillRecord is the state of a pending record on disk.
type fillRecord int

const (
	fillRecordMissing fillRecord = iota
	fillRecordOK
	fillRecordCorrupt
)

// readClaudeFillRecord reads the session's pending record and reports its state.
// A record that exists but does not decode is fillRecordCorrupt.
func readClaudeFillRecord(sessionID string) (claudeFillPending, fillRecord) {
	path, err := claudeFillPendingPath(sessionID)
	if err != nil {
		return claudeFillPending{}, fillRecordMissing
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return claudeFillPending{}, fillRecordMissing
	}
	var p claudeFillPending
	if err := json.Unmarshal(data, &p); err != nil {
		return claudeFillPending{}, fillRecordCorrupt
	}
	return p, fillRecordOK
}

// readClaudeFillPending returns the session's pending fill. ok is false when there
// is none, or when the record cannot be read.
func readClaudeFillPending(sessionID string) (claudeFillPending, bool) {
	p, state := readClaudeFillRecord(sessionID)
	return p, state == fillRecordOK
}

// loadClaudeFillPending is readClaudeFillPending for the collector. A corrupt record
// is removed and noted as pending_corrupt beside the DB at dbPath, so it never
// blocks a later attempt (issue 1303 M3).
func loadClaudeFillPending(dbPath, sessionID string) (claudeFillPending, bool) {
	p, state := readClaudeFillRecord(sessionID)
	switch state {
	case fillRecordOK:
		return p, true
	case fillRecordCorrupt:
		removeClaudeFillPending(sessionID)
		noteCorruptPending(dbPath, sessionID)
	}
	return claudeFillPending{}, false
}

// noteCorruptPending writes the pending_corrupt note for a session's record.
func noteCorruptPending(dbPath, sessionID string) {
	_ = writeCompactDiagnostic(dbPath, compactDiagNote{At: time.Now().UTC(), SessionID: sessionID, Kind: "pending", Reason: reasonPendingCorrupt, CollectorPID: os.Getpid()})
}

// removeClaudeFillPending drops the session's pending fill, if any.
func removeClaudeFillPending(sessionID string) {
	if path, err := claudeFillPendingPath(sessionID); err == nil {
		_ = os.Remove(path)
	}
}

// armClaudeFill records a pending fill and arms the first scheduled re-read.
func armClaudeFill(dbPath string, p claudeFillPending) error {
	if err := writeClaudeFillPending(p); err != nil {
		return err
	}
	claudeFillNext(dbPath, p.SessionID, 0)
	return nil
}

// claudeFillNext arms the re-read at index of the fill_retry schedule. A schedule
// that cannot be loaded, or an index past its end, arms nothing; the event path
// still gives the pending record its attempts.
func claudeFillNext(dbPath, sessionID string, index int) {
	schedule, err := hookdispatch.LoadClaudeCompactFillRetry()
	if err != nil || index >= len(schedule) {
		return
	}
	claudeFills.arm(sessionID, schedule[index], func() { claudeFillFire(dbPath, sessionID, index) })
}

// claudeFillFire is one scheduled re-read. It opens the DB by path, because the timer
// runs outside the event that armed it. A failed attempt arms the next re-read, and
// the last one drops the record. An applied fill drops the record here, after its
// own transaction has committed. The attempt holds claudeFillMu (issue 1303 M3).
func claudeFillFire(dbPath, sessionID string, index int) {
	claudeFillMu.Lock()
	defer claudeFillMu.Unlock()
	p, ok := loadClaudeFillPending(dbPath, sessionID)
	if !ok {
		return
	}
	db, err := telemetry.Open(dbPath)
	if err == nil {
		applied, stepErr := claudeFillStep(db, p, fmt.Sprintf("retry:%d", index+1))
		_ = db.Close()
		if stepErr == nil && applied {
			removeClaudeFillPending(sessionID)
			return
		}
	}
	schedule, err := hookdispatch.LoadClaudeCompactFillRetry()
	if err != nil || index+1 >= len(schedule) {
		removeClaudeFillPending(sessionID)
		return
	}
	claudeFillNext(dbPath, sessionID, index+1)
}

// claudeFillOnEvent gives a pending fill one attempt on each session event. This is
// how a restarted collector finishes a fill whose timers were lost. A record older
// than the whole schedule is dropped, so a boundary that never comes is not re-read
// on every event.
func claudeFillOnEvent(db *telemetry.DB, sessionID string) error {
	if !claudeFillMu.TryLock() {
		// A timer's attempt is running. It keeps the record and tries again itself.
		return nil
	}
	defer claudeFillMu.Unlock()
	p, ok := loadClaudeFillPending(db.Path(), sessionID)
	if !ok {
		return nil
	}
	window, err := claudeFillWindow()
	if err != nil {
		return err
	}
	if claudeFillExpired(p.ArmedAt, time.Now(), window) {
		removeClaudeFillPending(sessionID)
		return nil
	}
	_, err = claudeFillStep(db, p, "event")
	return err
}

// PruneClaudeFillPending runs once at collector start, before any event. It removes
// pending records older than the whole fill schedule and records that cannot be
// decoded, noting each corrupt one beside the DB at dbPath (issue 1303 M3). A fresh
// record is kept, so a restarted collector can still finish its fill.
func PruneClaudeFillPending(dbPath string, now time.Time) error {
	window, err := claudeFillWindow()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(xdgpath.StateHome(), "harnez", "claude-fill"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".json") {
			continue
		}
		sessionID := strings.TrimSuffix(name, ".json")
		p, state := readClaudeFillRecord(sessionID)
		switch {
		case state == fillRecordCorrupt:
			removeClaudeFillPending(sessionID)
			noteCorruptPending(dbPath, sessionID)
		case state == fillRecordOK && claudeFillExpired(p.ArmedAt, now, window):
			removeClaudeFillPending(sessionID)
		}
	}
	return nil
}

// claudeFillFault is a test seam (issue 1303 M2). It runs inside the fill's
// transaction after the row update, and a non-nil error fails the attempt there.
// Production leaves it nil.
var claudeFillFault func(stage string) error

// claudeFillStep reads the pending row's transcript once. When the boundary belongs
// to this compaction, it fills the row, writes the snapshots and reconciles in one
// transaction, so a failure leaves no partial state. filledAfter names the attempt
// that filled it. The caller drops the pending record once the fill is committed;
// a record left behind is dropped by the next attempt, which finds the row filled.
// Every attempt writes one diagnostic note. It reports whether the row was filled.
func claudeFillStep(db *telemetry.DB, p claudeFillPending, filledAfter string) (bool, error) {
	compact, readErr := scanClaudeCompactSpec(p.TranscriptPath, p.SessionID)
	preAt := p.PreCompactAt
	out := compactOutcome{preAt: &preAt}
	if readErr != nil {
		out.reason = reasonReadError
	} else {
		out.reason = claudeBoundaryReason(compact, preAt)
	}
	if compact != nil {
		boundaryAt := compact.Timestamp
		out.boundaryAt = &boundaryAt
	}
	filled := out.reason == reasonApplied
	if filled {
		catalog, err := telemetry.LoadPricingCatalog("")
		if err != nil {
			return false, err
		}
		updated := false
		err = db.InTransaction(func(tx *telemetry.DB) error {
			var err error
			updated, err = tx.FillCompactionEvent(p.RowID, compact.PostTokens, compact.Model)
			if err != nil || !updated {
				return err
			}
			if claudeFillFault != nil {
				if err := claudeFillFault("after-row"); err != nil {
					return err
				}
			}
			if err := insertClaudeSnapshot(tx, p.SessionID, "precompact", nil, compact.PreTokens); err != nil {
				return err
			}
			if err := insertClaudeSnapshot(tx, p.SessionID, "postcompact", &p.BoundaryID, compact.PostTokens); err != nil {
				return err
			}
			return tx.ReconcileCompactionEconomics(p.SessionID, compact.Model, catalog)
		})
		if err != nil {
			return false, err
		}
		if !updated {
			// The row already has its tokens; there is nothing left to write.
			removeClaudeFillPending(p.SessionID)
			return true, nil
		}
	} else {
		filledAfter = ""
	}
	if err := writeCompactDiagnostic(db.Path(), compactNote(p.SessionID, out, p.Hook, filledAfter)); err != nil {
		return filled, err
	}
	return filled, nil
}
