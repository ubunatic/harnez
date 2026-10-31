package subagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// TierOverridesPath returns the file path storing active tier overrides.
func TierOverridesPath() string {
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

// LoadTierOverrides reads unexpired tier overrides from disk. If missing or expired,
// it returns an empty map (or nil) and nil error.
func LoadTierOverrides() (*TierOverrides, error) {
	path := TierOverridesPath()
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
		// Expired
		return nil, nil
	}
	return &ov, nil
}

// SaveTierOverrides writes the given overrides to disk atomically or directly.
func SaveTierOverrides(ov TierOverrides) error {
	path := TierOverridesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ov, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + fmt.Sprintf(".tmp.%d", time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ClearTierOverrides removes the overrides file if present.
func ClearTierOverrides() error {
	path := TierOverridesPath()
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
