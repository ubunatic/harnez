package modelblock

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunnerOverlayReadsGlobalReadOnlyAndIsolatesWrites(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("TAIA_SESSION_DIR", "")

	now := time.Now().UTC().Truncate(time.Second)
	globalPath := GlobalPath()
	globalBlock := Block{
		Provider:  "codex",
		Model:     "gpt-5",
		Until:     now.Add(2 * time.Hour),
		Reason:    "host block",
		CreatedAt: now,
	}
	if err := Add(globalPath, globalBlock, now); err != nil {
		t.Fatalf("seed global block: %v", err)
	}
	globalBefore, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(globalPath + ".lock")

	runnerRoot := filepath.Join(t.TempDir(), "runner-sess")
	if err := os.Mkdir(runnerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", runnerRoot)

	overlayPath, err := ResolvedPath()
	if err != nil {
		t.Fatalf("ResolvedPath: %v", err)
	}
	if overlayPath != Path() || filepath.Clean(overlayPath) == filepath.Clean(globalPath) {
		t.Fatalf("expected distinct overlay path, got %q vs global %q", overlayPath, globalPath)
	}

	// Runner Add writes only to overlay, never touching GlobalPath or global .lock.
	localBlock := Block{
		Provider:  "agy",
		Model:     "gemini-pro",
		Until:     now.Add(time.Hour),
		Reason:    "runner block",
		CreatedAt: now,
	}
	if err := Add(Path(), localBlock, now); err != nil {
		t.Fatalf("runner Add: %v", err)
	}

	active := Active(Path(), now)
	if len(active) != 2 {
		t.Fatalf("merged active blocks = %+v, want 2", active)
	}
	// Runner sees both global + overlay, and cannot unblock the global block.
	if _, err := Remove(Path(), "codex", "gpt-5"); err == nil {
		t.Fatal("expected error unblocking global model block from runner")
	}
	// Runner can unblock its own overlay block.
	removed, err := Remove(Path(), "agy", "gemini-pro")
	if err != nil || !removed {
		t.Fatalf("Remove overlay block = (%v, %v), want (true, nil)", removed, err)
	}
	// Direct Add/Remove against GlobalPath in runner mode fails closed.
	if err := Add(globalPath, localBlock, now); err == nil {
		t.Fatal("expected direct Add to GlobalPath in runner mode to fail")
	}

	globalAfter, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(globalBefore, globalAfter) {
		t.Fatal("global model-blocks.json was modified in runner mode")
	}
	if _, err := os.Stat(globalPath + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("global lockfile created in runner mode: %v", err)
	}
}

func TestRunnerOverlayFailsClosedOnMalformedRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	badRoot := filepath.Join(t.TempDir(), "world-writable")
	if err := os.Mkdir(badRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", badRoot)

	if _, err := ResolvedPath(); err == nil {
		t.Fatal("expected ResolvedPath error on mode 0755 runner root")
	}
	if Path() != "" {
		t.Fatalf("Path() = %q on invalid root, want empty", Path())
	}
	now := time.Now()
	if err := Add(Path(), Block{Provider: "codex", Model: "m", Until: now.Add(time.Hour)}, now); err == nil {
		t.Fatal("expected Add on invalid runner root to fail closed")
	}

	// Global blocks remain enforced even when runner root is malformed (M1).
	t.Setenv("TAIA_SESSION_DIR", "")
	if err := Add(GlobalPath(), Block{
		Provider: "codex", Model: "global-kept", Until: now.Add(time.Hour), Reason: "global", CreatedAt: now,
	}, now); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", badRoot)
	active := Active(Path(), now)
	if len(active) != 1 || active[0].Model != "global-kept" {
		t.Fatalf("expected global block active on malformed runner root, got %+v", active)
	}
}
