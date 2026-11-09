package usage

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// powerPeaksFilename is the boot-scoped Power box state file, kept in the
// usage state dir next to the other harnez usage state.
const powerPeaksFilename = "power-peaks.json"

// powerBootIDPath is the kernel's per-boot UUID. Peaks are only valid for the
// boot that measured them; a different boot_id resets them. A variable so
// tests can point it at a fixture.
var powerBootIDPath = "/proc/sys/kernel/random/boot_id"

// powerPeakSaveInterval is the write throttle for the peak file, from
// power_peak_save_interval in spec/usage.yaml. A new peak waits for this window
// before it is written; Flush writes it at once.
func powerPeakSaveInterval() time.Duration {
	d, err := powerPeakSaveIntervalFromSpec()
	if err != nil {
		panic(err)
	}
	return d
}

func powerPeakSaveIntervalFromSpec() (time.Duration, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, usageSpecPath)
	if err != nil {
		return 0, err
	}
	var spec usageSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return 0, err
	}
	d, err := time.ParseDuration(spec.PowerPeakSaveInterval)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s: power_peak_save_interval must be a positive duration", usageSpecPath)
	}
	return d, nil
}

// powerPeakFile is the on-disk shape of the peak state.
type powerPeakFile struct {
	BootID string             `json:"boot_id"`
	Peaks  map[string]float64 `json:"peaks"`
}

// powerPeakStore holds the highest watts measured per power source since the
// current boot. It is shared by every harnez watcher on the machine: each save
// re-reads the file under a lock, takes the max per key, and replaces the file
// atomically, so concurrent writers never lose a peak. When bootID is empty
// (no boot_id on this kernel) the store still works in memory, it just does
// not persist.
type powerPeakStore struct {
	path         string
	bootID       string
	now          func() time.Time
	saveInterval time.Duration
	mu           sync.Mutex
	peaks        map[string]float64
	dirty        bool
	lastWrite    time.Time
}

// newPowerPeakStore loads the peaks from path when the file belongs to bootID.
// A missing, corrupt, or other-boot file starts empty.
func newPowerPeakStore(path, bootID string, now func() time.Time) *powerPeakStore {
	s := &powerPeakStore{path: path, bootID: bootID, now: now, saveInterval: powerPeakSaveInterval(), peaks: map[string]float64{}}
	if path == "" || bootID == "" {
		return s
	}
	if f, ok := readPowerPeakFile(path); ok && f.BootID == bootID {
		for k, v := range f.Peaks {
			s.peaks[k] = v
		}
	}
	return s
}

// Peak returns the highest watts recorded for key since boot (0 when none).
func (s *powerPeakStore) Peak(key string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.peaks[key]
}

// Observe records watts for key and returns the peak after the update. A new
// peak marks the store dirty and is written once saveInterval has passed since
// the last write. Save errors are not returned: the peak is best-effort state,
// and the box keeps working from memory.
func (s *powerPeakStore) Observe(key string, watts float64) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if watts > s.peaks[key] {
		s.peaks[key] = watts
		s.dirty = true
	}
	now := s.now()
	if s.dirty && (s.lastWrite.IsZero() || now.Sub(s.lastWrite) >= s.saveInterval) {
		_ = s.saveLocked(now)
	}
	return s.peaks[key]
}

// Flush writes any pending peak now, ignoring the throttle.
func (s *powerPeakStore) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirty {
		return nil
	}
	return s.saveLocked(s.now())
}

// saveLocked merges the file with the in-memory peaks under an exclusive
// lock on a sidecar file, then replaces the file with a temp-and-rename write.
// The file's peaks are only kept when it belongs to this boot.
func (s *powerPeakStore) saveLocked(now time.Time) error {
	if s.path == "" || s.bootID == "" {
		s.dirty = false
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	lock, err := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) }()

	merged := make(map[string]float64, len(s.peaks))
	for k, v := range s.peaks {
		merged[k] = v
	}
	if f, ok := readPowerPeakFile(s.path); ok && f.BootID == s.bootID {
		for k, v := range f.Peaks {
			if v > merged[k] {
				merged[k] = v
			}
		}
	}
	data, err := json.Marshal(powerPeakFile{BootID: s.bootID, Peaks: merged})
	if err != nil {
		return err
	}
	if err := writeFileAtomic(s.path, data); err != nil {
		return err
	}
	s.peaks = merged
	s.dirty = false
	s.lastWrite = now
	return nil
}

// readPowerPeakFile parses the peak file, reporting ok=false when it is
// missing or unreadable.
func readPowerPeakFile(path string) (powerPeakFile, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return powerPeakFile{}, false
	}
	var f powerPeakFile
	if err := json.Unmarshal(data, &f); err != nil {
		return powerPeakFile{}, false
	}
	return f, true
}

// writeFileAtomic writes data to a temp file beside path and renames it over
// path, so a reader never sees a partial file.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName) // best-effort cleanup; failure does not change the result
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName) // best-effort cleanup; failure does not change the result
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName) // best-effort cleanup; failure does not change the result
		return err
	}
	return nil
}

// readBootID returns the kernel boot UUID at path, or "" when it cannot be read.
func readBootID(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// powerPeaksPath is the peak state file under the usage state dir.
func powerPeaksPath() string {
	return filepath.Join(StateDir(""), powerPeaksFilename)
}
