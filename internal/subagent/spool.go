package subagent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SpoolVersion is the pointer format version written to the spool.
const SpoolVersion = 1

// spoolPointer is one spool file. It tells the host where a session lives and
// carries a copy of its metadata. A pointer is a hint, never a source of truth:
// readers confirm it against the registry at Store before they trust it.
type spoolPointer struct {
	V         int             `json:"v"`
	SessionID string          `json:"session_id"`
	Store     string          `json:"store"`
	UpdatedAt string          `json:"updated_at"`
	Entry     json.RawMessage `json:"entry"`
}

// SpoolDir returns HARNEZ_SPOOL_DIR, trimmed, or ~/.harnez/spool when it is unset
// or blank.
func SpoolDir() string {
	if v := strings.TrimSpace(os.Getenv("HARNEZ_SPOOL_DIR")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.harnez/spool"
	}
	return filepath.Join(home, ".harnez", "spool")
}

// safeSpoolID reports whether id can name a spool file without escaping the
// spool directory.
func safeSpoolID(id string) bool {
	return id != "" && !strings.ContainsAny(id, `/\`) && !strings.HasPrefix(id, ".")
}

// writeSpoolPointer records a session's stored entry in the spool. Callers
// ignore its error: a missing or full spool must not fail a registry write.
func writeSpoolPointer(storeDir, id string, entry []byte, now time.Time) error {
	if !safeSpoolID(id) {
		return fmt.Errorf("session id %q cannot name a spool file", id)
	}
	store, err := filepath.Abs(storeDir)
	if err != nil {
		return err
	}
	dir := SpoolDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(spoolPointer{
		V:         SpoolVersion,
		SessionID: id,
		Store:     store,
		UpdatedAt: now.UTC().Format(time.RFC3339Nano),
		Entry:     json.RawMessage(entry),
	}, "", "  ")
	if err != nil {
		return err
	}
	// Write and rename so a reader never sees a partial pointer.
	tmp, err := os.CreateTemp(dir, id+".json.tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, id+".json")); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// removeSpoolPointer drops a session's pointer once the session is deleted.
// A missing pointer is not an error.
func removeSpoolPointer(id string) error {
	if !safeSpoolID(id) {
		return nil
	}
	err := os.Remove(filepath.Join(SpoolDir(), id+".json"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
