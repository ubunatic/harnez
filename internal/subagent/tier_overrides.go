package subagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/runstate"
)

// TierOverrides holds temporary model overrides for agent effort tiers.
type TierOverrides struct {
	ExpiresAt time.Time         `json:"expires_at"`
	Tiers     map[string]string `json:"tiers"`
}

var (
	tierOverridesStateDirForTest string
	tierOverridesNowForTest      func() time.Time
)

// SetTierOverridesStateDirForTest sets a temporary state dir for unit tests.
func SetTierOverridesStateDirForTest(dir string) func() {
	prev := tierOverridesStateDirForTest
	tierOverridesStateDirForTest = dir
	return func() { tierOverridesStateDirForTest = prev }
}

// SetTierOverridesNowForTest overrides the time function for unit tests.
func SetTierOverridesNowForTest(fn func() time.Time) func() {
	prev := tierOverridesNowForTest
	tierOverridesNowForTest = fn
	return func() { tierOverridesNowForTest = prev }
}

func tierOverridesNow() time.Time {
	if tierOverridesNowForTest != nil {
		return tierOverridesNowForTest()
	}
	return time.Now()
}

// GlobalTierOverridesPath returns the shared host tier overrides path.
// It is read-only when running confined under $TAIA_SESSION_DIR.
func GlobalTierOverridesPath() string {
	if tierOverridesStateDirForTest != "" {
		return filepath.Join(tierOverridesStateDirForTest, "tier_overrides.json")
	}
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, _ := os.UserHomeDir()
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, "harnez", "models", "tier_overrides.json")
}

// ResolvedTierOverridesPath returns the writable tier overrides path: the
// global path on the host (or test dir), or a validated runner overlay under
// $TAIA_SESSION_DIR when running confined.
func ResolvedTierOverridesPath() (string, error) {
	if tierOverridesStateDirForTest != "" || os.Getenv("TAIA_SESSION_DIR") == "" {
		return GlobalTierOverridesPath(), nil
	}
	dir, err := runstate.PolicyDir("")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tier_overrides.json"), nil
}

// TierOverridesPath returns the writable tier overrides path, or "" if
// TAIA_SESSION_DIR is invalid.
func TierOverridesPath() string {
	path, err := ResolvedTierOverridesPath()
	if err != nil {
		return ""
	}
	return path
}

func loadTierOverridesFile(path string) (*TierOverrides, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ov TierOverrides
	if err := json.Unmarshal(data, &ov); err != nil {
		return nil, fmt.Errorf("read tier overrides %s: %w", path, err)
	}
	now := tierOverridesNow()
	if !ov.ExpiresAt.IsZero() && now.After(ov.ExpiresAt) {
		return nil, nil
	}
	return &ov, nil
}

// LoadTierOverrides reads unexpired tier overrides from disk. If missing or expired,
// it returns nil, nil. In runner mode ($TAIA_SESSION_DIR set without test dir
// override), read-only global overrides are merged beneath runner overlay entries.
func LoadTierOverrides() (*TierOverrides, error) {
	overlayPath, err := ResolvedTierOverridesPath()
	if err != nil {
		return nil, err
	}
	if tierOverridesStateDirForTest != "" || os.Getenv("TAIA_SESSION_DIR") == "" {
		return loadTierOverridesFile(overlayPath)
	}
	global, err := loadTierOverridesFile(GlobalTierOverridesPath())
	if err != nil {
		return nil, err
	}
	overlay, err := loadTierOverridesFile(overlayPath)
	if err != nil {
		return nil, err
	}
	if global == nil && overlay == nil {
		return nil, nil
	}
	merged := &TierOverrides{Tiers: make(map[string]string)}
	if global != nil {
		merged.ExpiresAt = global.ExpiresAt
		for k, v := range global.Tiers {
			merged.Tiers[k] = v
		}
	}
	if overlay != nil {
		merged.ExpiresAt = overlay.ExpiresAt
		for k, v := range overlay.Tiers {
			merged.Tiers[k] = v
		}
	}
	return merged, nil
}

// SaveTierOverrides writes the given overrides to the writable tier overrides
// path atomically (the runner overlay when confined, never the global file).
func SaveTierOverrides(ov TierOverrides) error {
	path, err := ResolvedTierOverridesPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ov, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// ClearTierOverrides removes the writable overrides file if present (runner
// overlay only in runner mode, leaving global overrides intact).
func ClearTierOverrides() error {
	path, err := ResolvedTierOverridesPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ActiveTierOverride reports the overridden model spec for a given trait/tier name, if active.
func ActiveTierOverride(trait string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(trait))
	ov, err := LoadTierOverrides()
	if err != nil || ov == nil || ov.Tiers == nil {
		return "", false
	}
	spec, ok := ov.Tiers[normalized]
	if !ok || strings.TrimSpace(spec) == "" {
		return "", false
	}
	return spec, true
}
