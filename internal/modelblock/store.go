package modelblock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Block is one local, expiring "out of quota" record for a provider model.
type Block struct {
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	Until     time.Time `json:"until"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Active reports whether the block still applies at now.
func (b Block) Active(now time.Time) bool { return b.Until.After(now) }

// Path returns the block file: $XDG_STATE_HOME/harnez/model-blocks.json, or
// ~/.local/state/harnez/model-blocks.json.
func Path() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "harnez", "model-blocks.json")
}

func key(provider, model string) string { return provider + "\x00" + model }

func read(path string) ([]Block, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var blocks []Block
	if err := json.Unmarshal(data, &blocks); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return blocks, nil
}

func write(path string, blocks []Block) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(blocks, "", "  ")
	if err != nil {
		return err
	}
	// A unique temp name keeps concurrent writers from clobbering each other's
	// half-written file; rename is atomic, so readers never see a torn file.
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		_ = os.Remove(tmp) // best-effort cleanup; failure does not change the result
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp) // best-effort cleanup; failure does not change the result
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp) // best-effort cleanup; failure does not change the result
		return err
	}
	return nil
}

// locked runs fn holding an exclusive advisory flock on path+".lock", so
// concurrent block/unblock calls (threads or processes) never lose updates.
func locked(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	return fn()
}

// Add records a block, replacing an earlier one for the same model and
// dropping expired records.
func Add(path string, b Block, now time.Time) error {
	return locked(path, func() error {
		blocks, err := read(path)
		if err != nil {
			return err
		}
		kept := []Block{b}
		for _, old := range blocks {
			if old.Active(now) && key(old.Provider, old.Model) != key(b.Provider, b.Model) {
				kept = append(kept, old)
			}
		}
		return write(path, kept)
	})
}

// Remove deletes the block for one model and reports whether one existed.
func Remove(path, provider, model string) (bool, error) {
	found := false
	err := locked(path, func() error {
		blocks, err := read(path)
		if err != nil {
			return err
		}
		var kept []Block
		for _, b := range blocks {
			if key(b.Provider, b.Model) == key(provider, model) {
				found = true
				continue
			}
			kept = append(kept, b)
		}
		if !found {
			return nil
		}
		return write(path, kept)
	})
	return found, err
}

// Active returns the blocks still in force at now, soonest expiry first.
// A missing or unreadable file means no blocks.
func Active(path string, now time.Time) []Block {
	blocks, _ := read(path)
	var active []Block
	for _, b := range blocks {
		if b.Active(now) {
			active = append(active, b)
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].Until.Before(active[j].Until) })
	return active
}

// AutoBlock records a block when errText is a known quota error for the
// provider (issue 1122), until the reset time the error names or the spec
// default. It reports the block and whether one was written.
func AutoBlock(path string, spec *Spec, provider, model, errText string, now time.Time) (Block, bool, error) {
	until, ok := spec.Detect(provider, errText, now)
	if !ok {
		return Block{}, false, nil
	}
	line := strings.TrimSpace(strings.SplitN(strings.TrimSpace(errText), "\n", 2)[0])
	b := Block{Provider: provider, Model: model, Until: until, Reason: "auto: " + line, CreatedAt: now}
	return b, true, Add(path, b, now)
}

// ParseUntil reads an RFC3339 time or a duration relative to now.
func ParseUntil(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		if d <= 0 {
			return time.Time{}, fmt.Errorf("--until duration must be positive: %q", s)
		}
		return now.Add(d), nil
	}
	return time.Time{}, fmt.Errorf("--until %q is neither RFC3339 nor a duration like 3h30m", s)
}
