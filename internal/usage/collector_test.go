package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/telemetry"
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

func TestPassivePostIsNotBlockedByStalledFetch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	dbPath := filepath.Join(home, "usage.sqlite")
	requests := make(chan PassiveRequest)
	fetchStarted := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runCollectorWithRequests(ctx, home, dbPath, time.Hour, io.Discard, false, func(ctx context.Context) (UsageSummary, error) {
			close(fetchStarted)
			<-ctx.Done()
			return UsageSummary{}, ctx.Err()
		}, nil, requests)
	}()
	<-fetchStarted
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	path, err := PassiveUpdatePath()
	if err != nil {
		t.Fatal(err)
	}
	endpoint := PassiveUpdateServer{Path: path, Requests: requests, Timeout: cfg.UpdateTimeout, MaxBodyBytes: cfg.UpdateMaxBodyBytes}
	mux := http.NewServeMux()
	endpoint.RegisterRoutes(mux)
	window := usagestore.Window{Provider: "claude", Key: "weekly", Name: "7-day", Source: "statusline", Freshness: "fresh", UsedFraction: .25, ObservedAt: time.Now()}
	body, err := json.Marshal(struct {
		Windows []usagestore.Window `json:"windows"`
	}{[]usagestore.Window{window}})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/usage/passive", bytes.NewReader(body))
	mux.ServeHTTP(response, req)
	if response.Code != http.StatusNoContent {
		t.Fatalf("POST status = %d, body %s", response.Code, response.Body)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	store, err := usagestore.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rows, err := store.CurrentFor(context.Background(), "claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("passive update was not persisted while fetch stalled")
	}
}

func TestPassiveRoutePersistsToProductionDefaultStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	dbPath, err := telemetry.DefaultDBPath()
	if err != nil {
		t.Fatal(err)
	}
	requests := make(chan PassiveRequest)
	fetchStarted := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runCollectorWithRequests(ctx, home, "", time.Hour, io.Discard, false, func(ctx context.Context) (UsageSummary, error) {
			close(fetchStarted)
			<-ctx.Done()
			return UsageSummary{}, ctx.Err()
		}, nil, requests)
	}()
	<-fetchStarted
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	path, err := PassiveUpdatePath()
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	(PassiveUpdateServer{Path: path, Requests: requests, Timeout: cfg.UpdateTimeout, MaxBodyBytes: cfg.UpdateMaxBodyBytes}).RegisterRoutes(mux)
	window := usagestore.Window{Provider: "claude", Key: "five_hour", Name: "5-hour", Source: "statusline", Freshness: "fresh", UsedFraction: .42, ObservedAt: time.Now().UTC()}
	body, err := json.Marshal(struct {
		Windows []usagestore.Window `json:"windows"`
	}{[]usagestore.Window{window}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`"used_fraction"`)) || bytes.Contains(body, []byte(`"UsedFraction"`)) {
		t.Fatalf("passive update wire format is not lower-case JSON: %s", body)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)))
	if response.Code != http.StatusNoContent {
		t.Fatalf("POST status = %d, body %s", response.Code, response.Body)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	store, err := usagestore.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var stored float64
	if err := store.QueryRow(context.Background(), `SELECT q.used_fraction FROM quota_windows q JOIN usage_observations o ON o.id=q.observation_id WHERE o.provider=? AND o.source=? AND q.window_key=? ORDER BY o.id DESC LIMIT 1`, "claude", "statusline", "five_hour").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != window.UsedFraction {
		t.Fatalf("stored used fraction = %v, want %v", stored, window.UsedFraction)
	}
}

func TestPassiveUpdateRejectsMalformedAndOversizedBodies(t *testing.T) {
	requests := make(chan PassiveRequest)
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	path, err := PassiveUpdatePath()
	if err != nil {
		t.Fatal(err)
	}
	endpoint := PassiveUpdateServer{Path: path, Requests: requests, Timeout: cfg.UpdateTimeout, MaxBodyBytes: 8}
	mux := http.NewServeMux()
	endpoint.RegisterRoutes(mux)
	for _, tc := range []struct {
		name, body string
		want       int
	}{
		{"malformed", "{", http.StatusBadRequest},
		{"oversized", `{"windows": []}`, http.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/usage/passive", strings.NewReader(tc.body)))
			if response.Code != tc.want {
				t.Fatalf("status = %d, want %d", response.Code, tc.want)
			}
		})
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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // no real load.watch_host poll
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
