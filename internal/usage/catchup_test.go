package usage

import (
	"context"
	"io"
	"path/filepath"
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

	fetched := map[string]int{}
	registry, err := NewRegistry(
		Collector{
			ID: "p1", Provider: "p1", Cadence: 5 * time.Minute, Timeout: time.Second,
			Collect: func(ctx context.Context) (AgentUsage, error) {
				fetched["p1"]++
				return AgentUsage{AgentID: "p1"}, nil
			},
		},
		Collector{
			ID: "p2", Provider: "p2", Cadence: 5 * time.Minute, Timeout: time.Second,
			Collect: func(ctx context.Context) (AgentUsage, error) {
				fetched["p2"]++
				return AgentUsage{AgentID: "p2"}, nil
			},
		},
		Collector{
			ID: "p3", Provider: "p3", Cadence: 5 * time.Minute, Timeout: time.Second,
			Collect: func(ctx context.Context) (AgentUsage, error) {
				fetched["p3"]++
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

func TestCollectorSkipsInitialFetchWhenNoClients(t *testing.T) {
	var fetched atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	home := t.TempDir()
	dbPath := filepath.Join(home, "usage.sqlite")
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
