package usage

import (
	"context"
	"io"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCollectCatchupFetchesOnlyProvidersOlderThanThreshold(t *testing.T) {
	threshold := time.Minute
	now := time.Now()

	// Provider 1: fresh (30s ago) -> should NOT be fetched
	// Provider 2: stale (2m ago)  -> should BE fetched
	// Provider 3: never fetched   -> should BE fetched

	var mu sync.Mutex
	fetched := map[string]int{}
	count := func(id string) {
		mu.Lock()
		defer mu.Unlock()
		fetched[id]++
	}
	registry, err := NewRegistry(
		Collector{
			ID: "p1", Provider: "p1", Cadence: 5 * time.Minute, Timeout: time.Second,
			Collect: func(ctx context.Context) (AgentUsage, error) {
				count("p1")
				return AgentUsage{AgentID: "p1"}, nil
			},
		},
		Collector{
			ID: "p2", Provider: "p2", Cadence: 5 * time.Minute, Timeout: time.Second,
			Collect: func(ctx context.Context) (AgentUsage, error) {
				count("p2")
				return AgentUsage{AgentID: "p2"}, nil
			},
		},
		Collector{
			ID: "p3", Provider: "p3", Cadence: 5 * time.Minute, Timeout: time.Second,
			Collect: func(ctx context.Context) (AgentUsage, error) {
				count("p3")
				return AgentUsage{AgentID: "p3"}, nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	registry.lastGood["p1"] = AgentUsage{AgentID: "p1", LastRefreshed: now.Add(-30 * time.Second)}
	registry.lastGood["p2"] = AgentUsage{AgentID: "p2", LastRefreshed: now.Add(-2 * time.Minute)}

	agents := registry.CollectCatchup(context.Background(), threshold, nil, nil)

	if fetched["p1"] != 0 {
		t.Fatalf("expected p1 not fetched, was fetched %d times", fetched["p1"])
	}
	if fetched["p2"] != 1 {
		t.Fatalf("expected p2 fetched once, was fetched %d times", fetched["p2"])
	}
	if fetched["p3"] != 1 {
		t.Fatalf("expected p3 fetched once, was fetched %d times", fetched["p3"])
	}

	// Make sure returned agents contains all providers (reusing lastGood for skipped ones)
	found := map[string]bool{}
	for _, a := range agents {
		found[a.AgentID] = true
	}
	if !found["p1"] || !found["p2"] || !found["p3"] {
		t.Fatalf("expected all providers in returned agents: %+v", found)
	}
}

func TestCollectorCatchupRetainsAgentMetadataFromStore(t *testing.T) {
	home := t.TempDir()
	dataHome := filepath.Join(home, "data")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))

	now := time.Now().UTC()
	window := func(name string, used float64) *QuotaWindow {
		return &QuotaWindow{Name: name, UsedPercent: used, RemainingPercent: 100 - used}
	}
	want := []AgentUsage{
		{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true, Account: "claude@example.test", PlanTier: "Max", Session: window("5-hour", 31)},
		{AgentID: "codex", Name: "OpenAI Codex", Installed: true, Authenticated: true, Account: "codex@example.test", PlanTier: "Plus", Session: window("5-hour", 42)},
		{AgentID: "agy", Name: "Antigravity (AGY)", Installed: true, Authenticated: true, Account: "agy@example.test", PlanTier: "Pro", Session: window("session", 53)},
	}
	summary := UsageSummary{Timestamp: now, Agents: want}
	dbPath := filepath.Join(dataHome, "harnez", "telemetry.sqlite")
	if err := InitializeUsageStore(context.Background(), home, dbPath); err != nil {
		t.Fatalf("initialize collector store: %v", err)
	}
	if err := persistUsage(context.Background(), home, dbPath, StateDir(home), io.Discard, false, summary); err != nil {
		t.Fatalf("persist collector summary: %v", err)
	}

	caughtUp := CollectRegisteredCatchup(context.Background(), home, nil, time.Hour, nil, nil)
	if len(caughtUp.Agents) != len(want) {
		t.Fatalf("catch-up agents = %d, want %d", len(caughtUp.Agents), len(want))
	}

	wantByID := make(map[string]AgentUsage, len(want))
	for _, agent := range want {
		wantByID[agent.AgentID] = agent
	}
	for _, agentID := range []string{"claude", "codex", "agy"} {
		snapshot, err := ReadAgentSnapshot(StateDir(home), agentID)
		if err != nil || snapshot == nil {
			t.Fatalf("read %s snapshot after catch-up: snapshot=%+v err=%v", agentID, snapshot, err)
		}
		got, expected := snapshot.Usage, wantByID[agentID]
		if got.AgentID != expected.AgentID || got.Name != expected.Name || !got.Installed || !got.Authenticated || got.Account != expected.Account || got.PlanTier != expected.PlanTier {
			t.Errorf("%s metadata after catch-up = %+v, want installed/authenticated metadata from %+v", agentID, got, expected)
		}
		if got.Session == nil || got.Session.Name != expected.Session.Name || got.Session.UsedPercent != expected.Session.UsedPercent {
			t.Errorf("%s quota after catch-up = %+v, want %+v", agentID, got.Session, expected.Session)
		}
	}
}

func TestCollectorSkipsInitialFetchWhenNoClients(t *testing.T) {
	var fetched atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // no real load.watch_host poll
	dbPath := filepath.Join(home, "usage.sqlite")
	// A stored snapshot exists, so the startup fetch is safe to skip (714 M3).
	if err := InitializeUsageStore(ctx, home, dbPath); err != nil {
		t.Fatal(err)
	}
	if err := PersistUsageSummary(ctx, home, dbPath, UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{AgentID: "claude", Installed: true}}}); err != nil {
		t.Fatal(err)
	}
	tracker := NewClientTracker(time.Second)

	done := make(chan error, 1)
	go func() {
		done <- RunCollectorWithClientTracker(ctx, home, dbPath, 100*time.Millisecond, io.Discard, false, func(ctx context.Context) (UsageSummary, error) {
			fetched.Add(1)
			return UsageSummary{Timestamp: time.Now()}, nil
		}, nil, nil, tracker)
	}()

	// Wait 25ms: without clients, it should NOT fetch immediately on start
	time.Sleep(25 * time.Millisecond)
	if got := fetched.Load(); got != 0 {
		t.Fatalf("expected 0 initial fetches without clients, got %d", got)
	}

	// Trigger a client heartbeat: now it should trigger an immediate catch-up fetch
	// (the client sends it only after the server reports ready, see
	// TestCollectorReadyBeforeFirstFetch).
	tracker.Heartbeat("client-1")
	time.Sleep(25 * time.Millisecond)
	if got := fetched.Load(); got != 1 {
		t.Fatalf("expected 1 fetch after client heartbeat, got %d", got)
	}

	cancel()
	<-done
}

// TestCollectorReadyBeforeFirstFetch guards the startup deadlock: clients wait
// for ready before their first heartbeat, and without clients the collector
// defers its first fetch, so ready must not wait for that fetch.
func TestCollectorReadyBeforeFirstFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // no real load.watch_host poll
	dbPath := filepath.Join(home, "usage.sqlite")
	tracker := NewClientTracker(time.Second)
	ready := make(chan struct{})

	done := make(chan error, 1)
	go func() {
		done <- RunCollectorWithClientTracker(ctx, home, dbPath, time.Hour, io.Discard, false, func(ctx context.Context) (UsageSummary, error) {
			return UsageSummary{Timestamp: time.Now()}, nil
		}, func() { close(ready) }, nil, tracker)
	}()

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("collector not ready without a fetch: clients would wait for the first interval")
	}

	cancel()
	<-done
}

// An empty store must not wait a full cadence for its first snapshot just
// because no client has attached yet (issue 991).
func TestCollectorInitialFetchWhenStoreEmptyEvenWithoutClients(t *testing.T) {
	var fetched atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dbPath := filepath.Join(home, "usage.sqlite")
	tracker := NewClientTracker(time.Second)

	done := make(chan error, 1)
	go func() {
		done <- RunCollectorWithClientTracker(ctx, home, dbPath, time.Hour, io.Discard, false, func(ctx context.Context) (UsageSummary, error) {
			fetched.Add(1)
			return UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{AgentID: "claude", Installed: true}}}, nil
		}, nil, nil, tracker)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := ReadStoredSummary(ctx, dbPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no snapshot persisted at startup with an empty store (fetches: %d)", fetched.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := fetched.Load(); got != 1 {
		t.Fatalf("initial fetches = %d, want 1", got)
	}
	cancel()
	<-done
}
