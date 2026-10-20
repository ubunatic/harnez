package usage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usagestore"
)

// TestRunCollectorWritesSnapshotsOnce checks that a single collectTick (as
// used by RunCollector's initial tick and by `harnez agent-collector
// --once`) writes one snapshot file per agent, readable back via
// ReadAgentSnapshot (issue 082).
func TestRunCollectorWritesSnapshotsOnce(t *testing.T) {
	home := t.TempDir()
	stateDir := StateDir(home)

	collectTick(context.Background(), home, nil, stateDir, io.Discard, false)

	for _, agentID := range []string{"claude", "agy", "codex"} {
		snap, err := ReadAgentSnapshot(stateDir, agentID)
		if err != nil {
			t.Fatalf("ReadAgentSnapshot(%s): %v", agentID, err)
		}
		if snap == nil {
			t.Fatalf("expected a snapshot for agent %s after collectTick, got none", agentID)
		}
		if snap.Usage.AgentID != agentID {
			t.Errorf("agent %s: snapshot AgentID = %q", agentID, snap.Usage.AgentID)
		}
	}
}

func TestCollectorTickPersistsStoreAndSnapshotWithInjectedFetch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	dbPath := filepath.Join(home, "usage.sqlite")
	called := false
	ctx := context.Background()
	if err := InitializeUsageStore(ctx, home, dbPath); err != nil {
		t.Fatal(err)
	}
	err := collectTickWithFetch(ctx, home, dbPath, StateDir(home), io.Discard, false, func(context.Context) (UsageSummary, error) {
		called = true
		return UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{
			AgentID: "claude", Name: "Claude Code", Installed: true,
			Session: &QuotaWindow{Name: "5-hour", UsedPercent: 42},
		}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("injected fetch was not called")
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rows, err := store.CurrentFor(context.Background(), "claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("collector tick wrote no normalized usage observations")
	}
	snapshot, err := ReadAgentSnapshot(StateDir(home), "claude")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot == nil {
		t.Fatal("collector tick wrote no JSON snapshot")
	}
}

func TestReadStoredSummaryUsesChmodReadOnlyStore(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, "usage.sqlite")
	ctx := context.Background()
	if err := InitializeUsageStore(ctx, home, dbPath); err != nil {
		t.Fatal(err)
	}
	want := UsageSummary{Timestamp: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC), Agents: []AgentUsage{{AgentID: "claude", Name: "Claude Code", Installed: true}}}
	if err := PersistUsageSummary(ctx, home, dbPath, want); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dbPath, 0o444); err != nil {
		t.Fatal(err)
	}
	got, err := ReadStoredSummary(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Timestamp.Equal(want.Timestamp) || len(got.Agents) != 1 || got.Agents[0].AgentID != "claude" {
		t.Fatalf("summary = %+v", got)
	}
}

// TestRunCollectorStopsOnContextCancel checks the timer loop returns
// promptly (without waiting for a full tick interval) once ctx is
// cancelled, so the daemon shuts down cleanly on SIGTERM/SIGINT.
func TestRunCollectorStopsOnContextCancel(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- RunCollector(ctx, home, nil, time.Hour, io.Discard, false)
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("RunCollector returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunCollector did not return within 5s of context cancellation")
	}
}
