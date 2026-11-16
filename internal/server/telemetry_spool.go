package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

type TelemetryRecord struct {
	RecordID string          `json:"record_id,omitempty"`
	Kind     string          `json:"kind"`
	Payload  json.RawMessage `json:"payload"`
}

// NewTelemetryRecord assigns an identity once, before delivery or spooling.
func NewTelemetryRecord(kind string, payload []byte) (TelemetryRecord, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return TelemetryRecord{}, err
	}
	return TelemetryRecord{RecordID: hex.EncodeToString(id[:]), Kind: kind, Payload: payload}, nil
}

// ErrQuarantine signals to DrainTelemetrySpool that a record is permanently
// invalid and should be moved out of the active spool to quarantine/ rather than
// retried on subsequent ticks.
var ErrQuarantine = errors.New("telemetry record quarantined")

func AppendTelemetrySpool(dir string, record []byte, maxBytes int64) error {
	if maxBytes <= 0 || int64(len(record)) > maxBytes {
		return fmt.Errorf("telemetry spool record exceeds configured byte cap")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == ".lock" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
	}
	if total+int64(len(record)) > maxBytes {
		return fmt.Errorf("telemetry spool is full (%d byte cap)", maxBytes)
	}
	tmp, err := os.CreateTemp(dir, ".pending-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck // best-effort cleanup; error not actionable
	if _, err := tmp.Write(record); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	name := filepath.Join(dir, fmt.Sprintf("%020d-%s.json", time.Now().UnixNano(), filepath.Base(tmpName)))
	return os.Rename(tmpName, name)
}

func DrainTelemetrySpool(ctx context.Context, dir string, consume func(context.Context, []byte) error) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var failures []error
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := consume(ctx, data); err != nil {
			if errors.Is(err, ErrQuarantine) {
				quarantineDir := filepath.Join(dir, "quarantine")
				_ = os.MkdirAll(quarantineDir, 0700)
				_ = os.Rename(path, filepath.Join(quarantineDir, entry.Name()))
				continue
			}
			failures = append(failures, err)
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return errors.Join(failures...)
}
