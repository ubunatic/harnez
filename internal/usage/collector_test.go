package usage

import (
	"context"
	"io"
	"testing"
	"time"
)

// TestRunCollectorWritesSnapshotsOnce checks that a single collectTick (as
// used by RunCollector's initial tick and by `harnez agent-collector
// --once`) writes one snapshot file per agent, readable back via
// ReadAgentSnapshot (issue 082).
func TestRunCollectorWritesSnapshotsOnce(t *testing.T) {
	home := t.TempDir()
	stateDir := StateDir(home)

	collectTick(context.Background(), home, nil, stateDir, io.Discard)

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

// TestRunCollectorStopsOnContextCancel checks the timer loop returns
// promptly (without waiting for a full tick interval) once ctx is
// cancelled, so the daemon shuts down cleanly on SIGTERM/SIGINT.
func TestRunCollectorStopsOnContextCancel(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- RunCollector(ctx, home, nil, time.Hour, io.Discard)
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
