package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// claudeFillAfter arms one fill timer. Tests replace it, so no test sleeps.
var claudeFillAfter = func(d time.Duration, fn func()) { time.AfterFunc(d, fn) }

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
// no_boundary when there is none (issue 1303 M1d, M1e).
func claudeBoundaryReason(compact *claudeCompact, preAt time.Time) string {
	switch {
	case compact == nil:
		return reasonNoBoundary
	case compact.Timestamp.Before(preAt):
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

// readClaudeFillPending returns the session's pending fill. ok is false when there
// is none, or when the record cannot be read.
func readClaudeFillPending(sessionID string) (claudeFillPending, bool) {
	path, err := claudeFillPendingPath(sessionID)
	if err != nil {
		return claudeFillPending{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return claudeFillPending{}, false
	}
	var p claudeFillPending
	if err := json.Unmarshal(data, &p); err != nil {
		return claudeFillPending{}, false
	}
	return p, true
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
	claudeFillAfter(schedule[index], func() { claudeFillFire(dbPath, sessionID, index) })
}

// claudeFillFire is one scheduled re-read. It opens the DB by path, because the timer
// runs outside the event that armed it. A failed attempt arms the next re-read, and
// the last one drops the record.
func claudeFillFire(dbPath, sessionID string, index int) {
	p, ok := readClaudeFillPending(sessionID)
	if !ok {
		return
	}
	db, err := telemetry.Open(dbPath)
	if err == nil {
		applied, stepErr := claudeFillStep(db, p, fmt.Sprintf("retry:%d", index+1))
		_ = db.Close()
		if stepErr == nil && applied {
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
	p, ok := readClaudeFillPending(sessionID)
	if !ok {
		return nil
	}
	schedule, err := hookdispatch.LoadClaudeCompactFillRetry()
	if err != nil {
		return err
	}
	var window time.Duration
	for _, d := range schedule {
		window += d
	}
	if time.Since(p.ArmedAt) > window {
		removeClaudeFillPending(sessionID)
		return nil
	}
	_, err = claudeFillStep(db, p, "event")
	return err
}

// claudeFillStep reads the pending row's transcript once. When the boundary belongs
// to this compaction, it fills the row, writes the snapshots, reconciles and drops
// the record; filledAfter names the attempt that filled it. Every attempt writes one
// diagnostic note. It reports whether the row was filled.
func claudeFillStep(db *telemetry.DB, p claudeFillPending, filledAfter string) (bool, error) {
	compact, readErr := scanClaudeCompact(p.TranscriptPath)
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
		updated, err := db.FillCompactionEvent(p.RowID, compact.PostTokens, compact.Model)
		if err != nil {
			return false, err
		}
		if !updated {
			// The row already has its tokens; there is nothing left to write.
			removeClaudeFillPending(p.SessionID)
			return true, nil
		}
		if err := insertClaudeSnapshot(db, p.SessionID, "precompact", nil, compact.PreTokens); err != nil {
			return false, err
		}
		if err := insertClaudeSnapshot(db, p.SessionID, "postcompact", &p.BoundaryID, compact.PostTokens); err != nil {
			return false, err
		}
		catalog, err := telemetry.LoadPricingCatalog("")
		if err != nil {
			return false, err
		}
		if err := db.ReconcileCompactionEconomics(p.SessionID, compact.Model, catalog); err != nil {
			return false, err
		}
		removeClaudeFillPending(p.SessionID)
	} else {
		filledAfter = ""
	}
	if err := writeCompactDiagnostic(db.Path(), compactNote(p.SessionID, out, p.Hook, filledAfter)); err != nil {
		return filled, err
	}
	return filled, nil
}
