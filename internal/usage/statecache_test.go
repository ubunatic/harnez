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

// TestPersistAgentSnapshotKeepsRicherCacheOnLossyOverwrite is the write-side
// counterpart to TestCacheOrLivePreservesQuotaOnLossyRecollect: a collector
// tick's live result with no quota signal (e.g. AGY isn't running this tick)
// must not clobber an already-persisted snapshot that does carry quota, or
// cacheOrLive's read-side guard (issue 101) has nothing richer left on disk
// to fall back to on the next read.
func TestPersistAgentSnapshotKeepsRicherCacheOnLossyOverwrite(t *testing.T) {
	dir := t.TempDir()
	rich := AgentUsage{
		AgentID:       "agy",
		Name:          "Antigravity (AGY)",
		Installed:     true,
		Authenticated: true,
		Session:       &QuotaWindow{Name: "Session", UsedPercent: 42},
	}
	if err := WriteAgentSnapshot(dir, "agy", rich); err != nil {
		t.Fatalf("WriteAgentSnapshot: %v", err)
	}
	richSnap, err := ReadAgentSnapshot(dir, "agy")
	if err != nil || richSnap == nil {
		t.Fatalf("ReadAgentSnapshot after seeding: %v / %+v", err, richSnap)
	}

	lossy := AgentUsage{
		AgentID:       "agy",
		Name:          "Antigravity (AGY)",
		Installed:     true,
		Authenticated: true,
		// No Session/Weekly/Tokens/ModelGroups: agy process not running.
	}
	if err := PersistAgentSnapshot(dir, lossy); err != nil {
		t.Fatalf("PersistAgentSnapshot: %v", err)
	}

	got, err := ReadAgentSnapshot(dir, "agy")
	if err != nil || got == nil {
		t.Fatalf("ReadAgentSnapshot after lossy persist: %v / %+v", err, got)
	}
	if got.Usage.Session == nil {
		t.Fatal("lossy PersistAgentSnapshot clobbered the richer cached snapshot's quota data")
	}
	if !got.FetchedAt.Equal(richSnap.FetchedAt) {
		t.Errorf("FetchedAt changed on a skipped write: got %v, want unchanged %v", got.FetchedAt, richSnap.FetchedAt)
	}

	// A genuinely richer (or equal) live result must still win.
	richer := AgentUsage{
		AgentID:       "agy",
		Name:          "Antigravity (AGY)",
		Installed:     true,
		Authenticated: true,
		Session:       &QuotaWindow{Name: "Session", UsedPercent: 77},
	}
	if err := PersistAgentSnapshot(dir, richer); err != nil {
		t.Fatalf("PersistAgentSnapshot (richer): %v", err)
	}
	got2, err := ReadAgentSnapshot(dir, "agy")
	if err != nil || got2 == nil {
		t.Fatalf("ReadAgentSnapshot after richer persist: %v / %+v", err, got2)
	}
	if got2.Usage.Session == nil || got2.Usage.Session.UsedPercent != 77 {
		t.Errorf("richer live result should have overwritten the cache, got Session = %+v", got2.Usage.Session)
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

// TestCacheOrLivePreservesQuotaOnLossyRecollect checks issue 101's core fix:
// when a stale-for-live-recollect cached snapshot has quota/token data but a
// fresh live collect comes back without any (e.g. AGY's process isn't
// currently running, so only static fields repopulate), cacheOrLive keeps
// serving the last known snapshot — tagged with its true FetchedAt time via
// LastRefreshed — instead of blanking the quota numbers. This is a
// code-level simulation of the scenario described in issue 101 (AGY stops
// answering on its local port after being idle a while); a real 30+ minute
// wait-based repro against a live agy process was judged impractical, so
// this test ages a cache file directly instead of sleeping.
func TestCacheOrLivePreservesQuotaOnLossyRecollect(t *testing.T) {
	dir := t.TempDir()
	fetchedAt := time.Now().Add(-2 * DefaultCacheStaleness) // stale for live-recollect, but well within DefaultDisplayStaleness
	richCached := AgentUsage{
		AgentID:       "agy",
		Installed:     true,
		Authenticated: true,
		Session:       &QuotaWindow{Name: "5h", UsedPercent: 42},
	}
	snap := AgentSnapshot{FetchedAt: fetchedAt, Usage: richCached}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(snapshotPath(dir, "agy"), data, 0600); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	// Simulates a live recollect that finds the agy process not currently
	// running: static fields still populate, but Session/Weekly do not.
	lossyLive := func() AgentUsage {
		return AgentUsage{AgentID: "agy", Installed: true, Authenticated: true}
	}

	got := cacheOrLive(dir, "agy", DefaultCacheStaleness, lossyLive)

	if got.Session == nil || got.Session.UsedPercent != 42 {
		t.Errorf("expected the last known Session quota to be preserved, got %+v", got.Session)
	}
	if !got.LastRefreshed.Equal(fetchedAt) {
		t.Errorf("LastRefreshed = %v, want the cached snapshot's FetchedAt %v", got.LastRefreshed, fetchedAt)
	}
	if got.IsStale(DefaultDisplayStaleness) {
		t.Error("2x DefaultCacheStaleness old should not be stale by the 7-day display threshold")
	}

	// A live recollect that actually finds richer/equal data (the process IS
	// running again) must win over the cache.
	richerLive := func() AgentUsage {
		return AgentUsage{AgentID: "agy", Installed: true, Authenticated: true, Session: &QuotaWindow{Name: "5h", UsedPercent: 7}}
	}
	got2 := cacheOrLive(dir, "agy", DefaultCacheStaleness, richerLive)
	if got2.Session == nil || got2.Session.UsedPercent != 7 {
		t.Errorf("expected a live recollect with real quota data to win over the cache, got %+v", got2.Session)
	}
}

// TestAgentUsageIsStale table-drives the 7-day display-hide threshold
// (issue 101), including the "zero LastRefreshed is never stale" backstop
// for callers (tests, older code paths) that build an AgentUsage directly.
func TestAgentUsageIsStale(t *testing.T) {
	cases := []struct {
		name   string
		agent  AgentUsage
		maxAge time.Duration
		want   bool
	}{
		{"zero LastRefreshed", AgentUsage{}, DefaultDisplayStaleness, false},
		{"just refreshed", AgentUsage{LastRefreshed: time.Now()}, DefaultDisplayStaleness, false},
		{"31 minutes old, well within 7d", AgentUsage{LastRefreshed: time.Now().Add(-31 * time.Minute)}, DefaultDisplayStaleness, false},
		{"6 days old", AgentUsage{LastRefreshed: time.Now().Add(-6 * 24 * time.Hour)}, DefaultDisplayStaleness, false},
		{"8 days old", AgentUsage{LastRefreshed: time.Now().Add(-8 * 24 * time.Hour)}, DefaultDisplayStaleness, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.agent.IsStale(tc.maxAge); got != tc.want {
				t.Errorf("IsStale(%s) = %v, want %v", tc.maxAge, got, tc.want)
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
