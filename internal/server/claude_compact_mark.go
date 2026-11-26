package server

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"ubunatic.com/harnez/internal/xdgpath"
)

// Issue 1303 M1d: PreCompact records when this compaction started, one file per
// session under the harnez state dir. PostCompact then accepts only a boundary at
// or after that time. Without the mark, a boundary from an earlier compaction
// that is still inside boundary_window looks fresh, so PostCompact never waits
// for the new one (the live failure: the note said stale_boundary).

type claudeCompactMark struct {
	At time.Time `json:"at"`
}

// claudeCompactMarkPath returns the mark file for a session. A session id that is
// not a plain file name is refused, so it cannot point outside the state dir.
func claudeCompactMarkPath(sessionID string) (string, error) {
	if sessionID == "" || sessionID == "." || sessionID == ".." || filepath.Base(sessionID) != sessionID {
		return "", errors.New("claude compact mark: session id is not a plain file name")
	}
	return filepath.Join(xdgpath.StateHome(), "harnez", "claude-compact", sessionID+".json"), nil
}

// writeClaudeCompactMark records the PreCompact time for a session.
func writeClaudeCompactMark(sessionID string, at time.Time) error {
	path, err := claudeCompactMarkPath(sessionID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(claudeCompactMark{At: at.UTC()})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// readClaudeCompactMark returns the session's last PreCompact time, or the zero
// time when there is no usable mark. The zero time keeps the window-only rule.
func readClaudeCompactMark(sessionID string) time.Time {
	path, err := claudeCompactMarkPath(sessionID)
	if err != nil {
		return time.Time{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}
	}
	var mark claudeCompactMark
	if err := json.Unmarshal(data, &mark); err != nil {
		return time.Time{}
	}
	return mark.At
}
