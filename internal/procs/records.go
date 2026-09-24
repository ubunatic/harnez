package procs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Record describes a process group started by harnez exec.
type Record struct {
	PGID           int       `json:"pgid"`
	PIDStarttime   uint64    `json:"pid_starttime"`
	Argv           []string  `json:"argv"`
	CWD            string    `json:"cwd"`
	OwnerPID       int       `json:"owner_pid"`
	OwnerStarttime uint64    `json:"owner_starttime"`
	Quota1         bool      `json:"quota_1"`
	Started        time.Time `json:"started"`
}

// RecordDir returns the runtime process-record directory.
func RecordDir() (string, error) {
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "harnez", "procs"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory for process records: %w", err)
	}
	if home == "" {
		return "", fmt.Errorf("resolve home directory for process records: home directory is empty")
	}
	return filepath.Join(home, ".harnez", "run", "procs"), nil
}

// WriteRecord persists one process group record at <dir>/<pgid>.json.
func WriteRecord(dir string, record Record) (string, error) {
	if record.PGID <= 0 {
		return "", fmt.Errorf("invalid process group id %d", record.PGID)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create process record directory: %w", err)
	}
	data, err := json.Marshal(record)
	if err != nil {
		return "", fmt.Errorf("encode process record: %w", err)
	}
	path := filepath.Join(dir, strconv.Itoa(record.PGID)+".json")
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		return "", fmt.Errorf("write process record: %w", err)
	}
	return path, nil
}

// RemoveRecord removes one process record. A missing record is already clean.
func RemoveRecord(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
