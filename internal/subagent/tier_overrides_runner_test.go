package subagent

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTierOverridesRunnerOverlayPreservesGlobalReadOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("TAIA_SESSION_DIR", "")

	now := time.Date(2026, 11, 16, 19, 0, 0, 0, time.UTC)
	defer SetTierOverridesNowForTest(func() time.Time { return now })()

	if err := SaveTierOverrides(TierOverrides{
		ExpiresAt: now.Add(3 * time.Hour),
		Tiers: map[string]string{
			"cheap": "claude:haiku:low",
			"smart": "claude:opus:high",
		},
	}); err != nil {
		t.Fatalf("save global overrides: %v", err)
	}
	globalPath := GlobalTierOverridesPath()
	globalBefore, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatal(err)
	}

	runnerRoot := filepath.Join(t.TempDir(), "runner-sess")
	if err := os.Mkdir(runnerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", runnerRoot)

	overlayPath, err := ResolvedTierOverridesPath()
	if err != nil {
		t.Fatalf("ResolvedTierOverridesPath: %v", err)
	}
	if overlayPath == globalPath {
		t.Fatalf("expected runner overlay path distinct from global %q", globalPath)
	}

	// Runner reads global overrides read-only initially.
	if got, ok := ActiveTierOverride("smart"); !ok || got != "claude:opus:high" {
		t.Fatalf("ActiveTierOverride(smart) = (%q, %v), want claude:opus:high", got, ok)
	}

	// Runner SaveTierOverrides writes only to overlay, leaving global intact.
	if err := SaveTierOverrides(TierOverrides{
		ExpiresAt: now.Add(time.Hour),
		Tiers: map[string]string{
			"cheap": "codex:luna:low",
			"lean":  "codex:spark:low",
		},
	}); err != nil {
		t.Fatalf("save runner overlay: %v", err)
	}
	if got, ok := ActiveTierOverride("cheap"); !ok || got != "codex:luna:low" {
		t.Fatalf("overlay cheap = (%q, %v), want codex:luna:low", got, ok)
	}
	if got, ok := ActiveTierOverride("smart"); !ok || got != "claude:opus:high" {
		t.Fatalf("inherited smart = (%q, %v), want claude:opus:high", got, ok)
	}

	// ClearTierOverrides deletes only the overlay, keeping global intact.
	if err := ClearTierOverrides(); err != nil {
		t.Fatalf("ClearTierOverrides: %v", err)
	}
	if _, ok := ActiveTierOverride("lean"); ok {
		t.Fatal("lean should be cleared from overlay")
	}
	if got, ok := ActiveTierOverride("cheap"); !ok || got != "claude:haiku:low" {
		t.Fatalf("global cheap after clear = (%q, %v), want claude:haiku:low", got, ok)
	}

	globalAfter, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(globalBefore, globalAfter) {
		t.Fatal("global tier_overrides.json was mutated in runner mode")
	}
}

func TestTierOverridesRunnerFailsClosedOnInvalidRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	badRoot := filepath.Join(t.TempDir(), "bad-perms")
	if err := os.Mkdir(badRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", badRoot)

	if err := SaveTierOverrides(TierOverrides{Tiers: map[string]string{"lean": "codex:luna:low"}}); err == nil {
		t.Fatal("expected SaveTierOverrides on invalid runner root to fail closed")
	}
	if _, err := LoadTierOverrides(); err == nil {
		t.Fatal("expected LoadTierOverrides on invalid runner root to fail closed")
	}
	if _, err := os.Stat(GlobalTierOverridesPath()); !os.IsNotExist(err) {
		t.Fatalf("invalid runner root touched global path: %v", err)
	}
}
