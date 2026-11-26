package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// payloadKeys returns the payload's key names, sorted. Values are never kept.
func payloadKeys(payload map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Issue 1303 M1c: one diagnostic note per Claude PostCompact, appended beside the
// telemetry DB as compact-diagnostics.jsonl. The host reads it with
// `tail -n 4 "${XDG_DATA_HOME:-$HOME/.local/share}/harnez/compact-diagnostics.jsonl"`.
// It records names and outcomes only: payload key names, never values, and
// never compact_summary or custom_instructions.

// compactDiagFile is the note file name beside the telemetry DB.
const compactDiagFile = "compact-diagnostics.jsonl"

// Outcome reasons. The hook decides the transcript reasons; the collector adds
// the DB-side ones when the hook found a boundary.
const (
	reasonNoTranscript  = "no_transcript"
	reasonReadError     = "read_error"
	reasonNoBoundary    = "no_boundary"
	reasonStaleBoundary = "stale_boundary"
	reasonNoPrecompact  = "no_precompact"
	reasonApplied       = "applied"
	// reasonPendingCorrupt is a pending fill record the collector could not decode;
	// the collector removes it and notes it (issue 1303 M3).
	reasonPendingCorrupt = "pending_corrupt"
)

// compactDiag is what the hook process saw. It travels in harnez_context.
type compactDiag struct {
	// Side names the process that ran PrepareCodexTelemetry; always "hook".
	Side string `json:"side"`
	// PID is the hook process.
	PID int `json:"pid"`
	// TranscriptSet is whether the payload carried a transcript_path.
	TranscriptSet bool `json:"transcript_set"`
	// Reason is the hook's transcript outcome. Empty means a boundary was read
	// and the collector decides.
	Reason string `json:"reason,omitempty"`
	// Keys are the payload key names, sorted. Values are never recorded.
	Keys []string `json:"keys"`
}

// compactDiagNote is one line of compact-diagnostics.jsonl.
type compactDiagNote struct {
	At        time.Time `json:"at"`
	SessionID string    `json:"session_id"`
	Kind      string    `json:"kind"`
	Reason    string    `json:"reason"`
	// PreCompactAt is the newest PreCompact row's time; BoundaryAt is the boundary's
	// timestamp. Both are timestamps only, never content (issue 1303 M1d).
	PreCompactAt *time.Time `json:"precompact_at,omitempty"`
	BoundaryAt   *time.Time `json:"boundary_at,omitempty"`
	// FilledAfter is set on an applied fill (issue 1303 M1e): "retry:N" for the
	// Nth scheduled re-read, or "event" for the next session event.
	FilledAfter  string       `json:"filled_after,omitempty"`
	Hook         *compactDiag `json:"hook,omitempty"`
	CollectorPID int          `json:"collector_pid"`
}

// writeCompactDiagnostic appends one note beside the DB at dbPath. A handle with
// no file path writes nothing, so a test or in-memory handle cannot land the note
// in the working directory.
func writeCompactDiagnostic(dbPath string, note compactDiagNote) error {
	if dbPath == "" {
		return nil
	}
	line, err := json.Marshal(note)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(filepath.Dir(dbPath), compactDiagFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(line, '\n'))
	return err
}
