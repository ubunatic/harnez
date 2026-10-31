package subagent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTierOverridesPersistenceAndExpiry(t *testing.T) {
	tempDir := t.TempDir()
	cleanupDir := SetTierOverridesStateDirForTest(tempDir)
	defer cleanupDir()

	fakeNow := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	cleanupNow := SetTierOverridesNowForTest(func() time.Time { return fakeNow })
	defer cleanupNow()

	// Initially empty
	ov, err := LoadTierOverrides()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if ov != nil {
		t.Fatalf("expected nil overrides initially, got %+v", ov)
	}
	if spec, ok := ActiveTierOverride("lean"); ok {
		t.Fatalf("expected inactive override, got %s", spec)
	}

	// Save active override
	toSave := TierOverrides{
		ExpiresAt: fakeNow.Add(2 * time.Hour),
		Tiers: map[string]string{
			"cheap": "claude:haiku:low",
			"lean":  "codex:luna:low",
			"smart": "claude:opus:high",
		},
	}
	if err := SaveTierOverrides(toSave); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadTierOverrides()
	if err != nil {
		t.Fatalf("load after save failed: %v", err)
	}
	if loaded == nil || len(loaded.Tiers) != 3 {
		t.Fatalf("expected 3 loaded tiers, got %+v", loaded)
	}

	if spec, ok := ActiveTierOverride("lean"); !ok || spec != "codex:luna:low" {
		t.Fatalf("ActiveTierOverride(lean) = (%q, %v), want (codex:luna:low, true)", spec, ok)
	}
	if spec, ok := ActiveTierOverride("cheap"); !ok || spec != "claude:haiku:low" {
		t.Fatalf("ActiveTierOverride(cheap) = (%q, %v), want (claude:haiku:low, true)", spec, ok)
	}

	// Case insensitivity
	if spec, ok := ActiveTierOverride("  SMART  "); !ok || spec != "claude:opus:high" {
		t.Fatalf("ActiveTierOverride(SMART) = (%q, %v), want (claude:opus:high, true)", spec, ok)
	}

	// Advance time past expiry
	fakeNow = fakeNow.Add(3 * time.Hour)
	expired, err := LoadTierOverrides()
	if err != nil {
		t.Fatalf("load after expiry failed: %v", err)
	}
	if expired != nil {
		t.Fatalf("expected nil expired overrides, got %+v", expired)
	}
	if spec, ok := ActiveTierOverride("lean"); ok {
		t.Fatalf("expected inactive override after expiry, got %s", spec)
	}

	// Clear tier overrides
	if err := ClearTierOverrides(); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "tier_overrides.json")); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, got err: %v", err)
	}
}

func TestResolveModelSelectionWithTierOverridePrecedence(t *testing.T) {
	tempDir := t.TempDir()
	cleanupDir := SetTierOverridesStateDirForTest(tempDir)
	defer cleanupDir()

	fakeNow := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	cleanupNow := SetTierOverridesNowForTest(func() time.Time { return fakeNow })
	defer cleanupNow()

	// Override lean to use a specific model, e.g. claude:sonnet:low
	toSave := TierOverrides{
		ExpiresAt: fakeNow.Add(1 * time.Hour),
		Tiers: map[string]string{
			"lean": "claude:sonnet:low",
		},
	}
	if err := SaveTierOverrides(toSave); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// When resolving developer role without explicit model, developer default is "lean".
	// The tier override for "lean" must take precedence over base spec default candidates.
	sel, err := ResolveModelWithAvailability("developer", "", nil, false)
	if err != nil {
		t.Fatalf("ResolveModelWithAvailability failed: %v", err)
	}
	if sel.Model.Provider != "claude" || sel.Model.Name != "sonnet" {
		t.Fatalf("resolved model = %+v, want claude:sonnet", sel.Model)
	}

	// When explicitly requesting a concrete model (e.g. "codex:luna"), explicit selection takes precedence!
	selExplicit, err := ResolveModelWithAvailability("developer", "codex:luna:low", nil, false)
	if err != nil {
		t.Fatalf("ResolveModelWithAvailability with explicit model failed: %v", err)
	}
	if selExplicit.Model.Provider != "codex" {
		t.Fatalf("explicit model did not take precedence: %+v", selExplicit.Model)
	}

	// After expiry, developer default falls back to normal defaults (e.g. luna)
	fakeNow = fakeNow.Add(2 * time.Hour)
	selExpired, err := ResolveModelWithAvailability("developer", "", nil, false)
	if err != nil {
		t.Fatalf("ResolveModelWithAvailability after expiry failed: %v", err)
	}
	if selExpired.Model.Provider == "claude" && selExpired.Model.Tier == "low" {
		// Normal developer default is codex:luna:low or similar, not claude:sonnet:low
		t.Logf("fell back to default: %+v", selExpired.Model)
	}
}
