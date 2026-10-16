package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Store persists the backend session ids of one tui session, so
// `harnez tui --session X` resumes the provider sessions after a restart.
// Each tui session has its own file; updates merge under an flock, so two
// processes on the same tui session keep each other's keys.
type Store struct {
	path string
}

// storeFile is the on-disk shape of a Store.
type storeFile struct {
	Sessions  map[string]string `json:"sessions"` // provider:model -> backend session id
	UpdatedAt time.Time         `json:"updated_at"`
}

// NewStore returns the store of tui session id under dir.
func NewStore(dir, id string) (*Store, error) {
	if id == "" || id != filepath.Base(id) || id == "." || id == ".." {
		return nil, fmt.Errorf("invalid tui session id %q: use a plain name without path separators", id)
	}
	return &Store{path: filepath.Join(dir, id+".json")}, nil
}

// Path returns the store file.
func (s *Store) Path() string { return s.path }

// Load returns the stored backend session ids; a missing file is empty. A
// corrupt file is renamed to <file>.corrupt-<unix time> and Load returns no
// sessions plus a warning, so one bad write does not fail every later turn.
func (s *Store) Load() (sessions map[string]string, warning string, err error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("read tui session store: %w", err)
	}
	var f storeFile
	if err := json.Unmarshal(data, &f); err != nil {
		backup := fmt.Sprintf("%s.corrupt-%d", s.path, time.Now().Unix())
		if renameErr := os.Rename(s.path, backup); renameErr != nil {
			return nil, "", fmt.Errorf("parse tui session store %s: %w (backup failed: %v)", s.path, err, renameErr)
		}
		return map[string]string{}, fmt.Sprintf("tui session store %s was corrupt (%v); moved to %s, starting new backend sessions", s.path, err, backup), nil
	}
	if f.Sessions == nil {
		f.Sessions = map[string]string{}
	}
	return f.Sessions, "", nil
}

// Prune deletes tui session store files in dir (and their lock files) not
// modified for maxAge, except the store of keepID. It returns how many store
// files it removed; a missing dir is empty.
func Prune(dir string, maxAge time.Duration, keepID string) (int, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("prune tui session store: %w", err)
	}
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".json" || name == keepID+".json" {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		path := filepath.Join(dir, name)
		if os.Remove(path) == nil {
			removed++
			_ = os.Remove(path + ".lock")
		}
	}
	return removed, nil
}

// Set stores id for key; an empty id removes key.
func (s *Store) Set(key, id string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create tui session store dir: %w", err)
	}
	lock, err := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open tui session store lock: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock tui session store: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck
	sessions, _, err := s.Load()
	if err != nil {
		return err
	}
	if id == "" {
		delete(sessions, key)
	} else {
		sessions[key] = id
	}
	data, err := json.MarshalIndent(storeFile{Sessions: sessions, UpdatedAt: time.Now().UTC()}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("write tui session store: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("write tui session store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("write tui session store: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("write tui session store: %w", err)
	}
	return nil
}
