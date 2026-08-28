package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWriteReadAgentSnapshotRoundTrip checks the atomic write-tmp-then-rename
// helper produces a file ReadAgentSnapshot can parse back unchanged, and
// leaves no leftover .tmp file (issue 082).
func TestWriteReadAgentSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := AgentUsage{
		AgentID:       "claude",
		Name:          "Claude Code",
		Installed:     true,
		Authenticated: true,
		PlanTier:      "Max",
	}

	if err := WriteAgentSnapshot(dir, "claude", want); err != nil {
		t.Fatalf("WriteAgentSnapshot: %v", err)
	}

	if _, err := os.Stat(snapshotPath(dir, "claude") + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected no leftover .tmp file, stat err = %v", err)
	}

	got, err := ReadAgentSnapshot(dir, "claude")
	if err != nil {
		t.Fatalf("ReadAgentSnapshot: %v", err)
	}
	if got == nil {
		t.Fatal("ReadAgentSnapshot returned nil, want a snapshot")
	}
	if got.Usage.AgentID != want.AgentID || got.Usage.PlanTier != want.PlanTier {
		t.Errorf("Usage = %+v, want AgentID/PlanTier from %+v", got.Usage, want)
	}
	if got.FetchedAt.IsZero() {
		t.Error("FetchedAt should be set by WriteAgentSnapshot")
	}
}

// TestReadAgentSnapshotMissing checks a missing snapshot file is reported as
// a nil result, not an error the caller has to unwrap.
func TestReadAgentSnapshotMissing(t *testing.T) {
	dir := t.TempDir()
	got, err := ReadAgentSnapshot(dir, "codex")
	if err != nil {
		t.Fatalf("ReadAgentSnapshot: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for missing snapshot, got %+v", got)
	}
}

// TestReadAgentSnapshotCorrupt checks a corrupt/unparsable snapshot file
// surfaces as an error rather than a false-positive nil (which cacheOrLive
// would otherwise treat identically to "not collected yet").
func TestReadAgentSnapshotCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(snapshotPath(dir, "agy"), []byte("not json"), 0600); err != nil {
		t.Fatalf("seed corrupt snapshot: %v", err)
	}
	if _, err := ReadAgentSnapshot(dir, "agy"); err == nil {
		t.Error("expected an error for a corrupt snapshot file, got nil")
	}
}

// TestAgentSnapshotIsFresh table-drives the staleness check.
func TestAgentSnapshotIsFresh(t *testing.T) {
	cases := []struct {
		name   string
		snap   *AgentSnapshot
		maxAge time.Duration
		wantOK bool
	}{
		{"nil snapshot", nil, time.Hour, false},
		{"just collected", &AgentSnapshot{FetchedAt: time.Now()}, time.Hour, true},
		{"within max age", &AgentSnapshot{FetchedAt: time.Now().Add(-30 * time.Minute)}, time.Hour, true},
		{"exactly at boundary (excluded)", &AgentSnapshot{FetchedAt: time.Now().Add(-time.Hour - time.Second)}, time.Hour, false},
		{"stale", &AgentSnapshot{FetchedAt: time.Now().Add(-2 * time.Hour)}, time.Hour, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.snap.IsFresh(tc.maxAge); got != tc.wantOK {
				t.Errorf("IsFresh(%s) = %v, want %v", tc.maxAge, got, tc.wantOK)
			}
		})
	}
}

// TestStateDirXDGResolution table-drives StateDir's XDG Base Directory
// resolution: an explicit XDG_STATE_HOME wins, otherwise the path falls
// back to ~/.local/state under the given home dir (issue 082).
func TestStateDirXDGResolution(t *testing.T) {
	cases := []struct {
		name     string
		xdgState string // "" leaves XDG_STATE_HOME unset
		homeDir  string
		wantSuf  string // suffix to check with strings.HasSuffix semantics via filepath.Join comparison
	}{
		{
			name:     "XDG_STATE_HOME set",
			xdgState: "/custom/state",
			homeDir:  "/home/someone",
			wantSuf:  filepath.Join("/custom/state", "harnez", "agents", "usage"),
		},
		{
			name:    "XDG_STATE_HOME unset falls back to ~/.local/state",
			homeDir: "/home/someone",
			wantSuf: filepath.Join("/home/someone", ".local", "state", "harnez", "agents", "usage"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", tc.xdgState)
			got := StateDir(tc.homeDir)
			if got != tc.wantSuf {
				t.Errorf("StateDir(%q) with XDG_STATE_HOME=%q = %q, want %q", tc.homeDir, tc.xdgState, got, tc.wantSuf)
			}
		})
	}
}

// TestCollectAllCacheFirst checks CollectAll's cache-first behavior end to
// end: with no daemon-written snapshot, it falls back to a live collect
// (matching pre-082 behavior exactly, so existing callers see no change
// when the daemon has never run); with a fresh snapshot present, it returns
// the cached reading without needing the agent's live source directory to
// exist at all; with a stale snapshot present, it falls back to live again.
func TestCollectAllCacheFirst(t *testing.T) {
	ctx := context.Background()

	t.Run("no cache: falls back to live collection", func(t *testing.T) {
		// Assert CollectAll matches CollectAllLive's result for the same
		// homeDir rather than asserting Installed==false directly: AGY's
		// collector falls back to the real machine's ~/.gemini when its
		// passed dir is absent (a pre-existing quirk unrelated to issue
		// 082), so Installed can legitimately be true on a dev machine that
		// has AGY configured even with an empty, isolated homeDir.
		home := t.TempDir()
		gotViaCache := CollectAll(ctx, home, nil)
		wantLive := CollectAllLive(ctx, home, nil)
		for i := range gotViaCache.Agents {
			got, want := gotViaCache.Agents[i], wantLive.Agents[i]
			if got.AgentID != want.AgentID || got.Installed != want.Installed || got.Authenticated != want.Authenticated {
				t.Errorf("agent %d: got %+v, want (matching live collect) %+v", i, got, want)
			}
		}
	})

	t.Run("fresh cache: used instead of live collection", func(t *testing.T) {
		home := t.TempDir()
		stateDir := StateDir(home)
		cached := AgentUsage{
			AgentID:       "claude",
			Name:          "Claude Code",
			Installed:     true,
			Authenticated: true,
			PlanTier:      "Max (cached)",
		}
		if err := WriteAgentSnapshot(stateDir, "claude", cached); err != nil {
			t.Fatalf("WriteAgentSnapshot: %v", err)
		}

		summary := CollectAll(ctx, home, nil)
		var got *AgentUsage
		for i := range summary.Agents {
			if summary.Agents[i].AgentID == "claude" {
				got = &summary.Agents[i]
			}
		}
		if got == nil {
			t.Fatal("no claude agent in summary")
		}
		// The claude dir doesn't exist under home, so a live collect would
		// report Installed=false; seeing Installed=true here proves the
		// cached snapshot was used instead of a live collect.
		if !got.Installed || got.PlanTier != "Max (cached)" {
			t.Errorf("got %+v, want the cached snapshot's fields", got)
		}
	})

	t.Run("stale cache: falls back to live collection", func(t *testing.T) {
		home := t.TempDir()
		stateDir := StateDir(home)
		cached := AgentUsage{AgentID: "claude", Installed: true, PlanTier: "should not be used"}
		if err := WriteAgentSnapshot(stateDir, "claude", cached); err != nil {
			t.Fatalf("WriteAgentSnapshot: %v", err)
		}
		// Backdate the snapshot file's content past the staleness window by
		// writing it directly (bypassing WriteAgentSnapshot's time.Now()).
		stale := AgentSnapshot{FetchedAt: time.Now().Add(-2 * DefaultCacheStaleness), Usage: cached}
		data, err := json.MarshalIndent(stale, "", "  ")
		if err != nil {
			t.Fatalf("marshal stale snapshot: %v", err)
		}
		if err := os.WriteFile(snapshotPath(stateDir, "claude"), data, 0600); err != nil {
			t.Fatalf("seed stale snapshot: %v", err)
		}

		summary := CollectAll(ctx, home, nil)
		for _, a := range summary.Agents {
			if a.AgentID == "claude" && a.Installed {
				t.Errorf("expected stale cache to be ignored (live Installed=false), got Installed=true")
			}
		}
	})

	t.Run("CollectAllLive ignores cache even when fresh", func(t *testing.T) {
		home := t.TempDir()
		stateDir := StateDir(home)
		cached := AgentUsage{AgentID: "claude", Installed: true, PlanTier: "should not be used"}
		if err := WriteAgentSnapshot(stateDir, "claude", cached); err != nil {
			t.Fatalf("WriteAgentSnapshot: %v", err)
		}

		summary := CollectAllLive(ctx, home, nil)
		for _, a := range summary.Agents {
			if a.AgentID == "claude" && a.Installed {
				t.Errorf("CollectAllLive should ignore the cache, got Installed=true")
			}
		}
	})
}
