package subagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

// TurnQuotaEvent stores one before/after quota reading keyed by session and turn.
type TurnQuotaEvent struct {
	SessionID string                 `json:"session_id"`
	Turn      int                    `json:"turn"`
	Boundary  string                 `json:"boundary"`
	Provider  string                 `json:"provider"`
	Reading   usage.TurnQuotaReading `json:"reading"`
}

// RecordTurnQuota appends one boundary observation in the session store.
func (s *FileSessionStore) RecordTurnQuota(event TurnQuotaEvent) error {
	if event.SessionID == "" || event.Turn < 1 || (event.Boundary != "before" && event.Boundary != "after") {
		return fmt.Errorf("invalid turn quota event")
	}
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(s.dir, "quota-readings.jsonl")
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	locked := false
	for attempt := 0; attempt < 5; attempt++ {
		if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			locked = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !locked {
		return fmt.Errorf("turn quota store busy")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck
	if event.Reading.CapturedAt.IsZero() {
		event.Reading.CapturedAt = time.Now().UTC()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}
