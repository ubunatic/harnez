package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/telemetry"
)

func TestTelemetryCollectorServiceRegistryOwnSessionAndLegacyRejection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "run"))
	t.Setenv("TAIA_SESSION_DIR", "")

	config, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	dbPath, err := telemetry.DefaultDBPath()
	if err != nil {
		t.Fatal(err)
	}
	registryDir := filepath.Join(home, ".harnez", "agents")
	service := &telemetryCollectorService{config: config, path: dbPath, registryDir: registryDir, interval: time.Hour}
	mux := http.NewServeMux()
	service.RegisterRoutes(mux)

	hostSocket, err := config.Socket()
	if err != nil {
		t.Fatal(err)
	}
	startUnixHTTPServer(t, hostSocket, mux, config.ReadHeaderTimeout)

	// Seed a legacy session file missing LaunchID directly in host registry.
	hostStore, err := subagent.NewSessionStore(registryDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := hostStore.Create(&subagent.Session{
		ID: "legacy-missing", Name: "legacy-missing", LaunchID: "", Provider: "codex", Model: "luna", Tier: "low", Status: "running",
	}); err != nil {
		t.Fatal(err)
	}
	// Clear LaunchID on disk to simulate pre-M12 legacy record lacking LaunchID.
	legacySess, err := hostStore.Get("legacy-missing")
	if err != nil {
		t.Fatal(err)
	}
	legacySess.LaunchID = ""
	if err := hostStore.Save(legacySess); err != nil {
		t.Fatal(err)
	}

	// Runner channel 1 tries to hijack legacy record missing LaunchID -> fails closed (403).
	runnerRoot := filepath.Join(t.TempDir(), "runner-legacy-probe")
	startRunnerRelayHarness(t, config, runnerRoot, hostSocket)
	if _, err := subagent.RegisterOwnSession(context.Background(), server.RunnerRegistryRequest{
		SessionID: "legacy-missing", Name: "legacy-missing", Status: "completed",
	}); !errors.Is(err, server.ErrRunnerForbidden) {
		t.Fatalf("legacy record hijacking err = %v, want ErrRunnerForbidden", err)
	}

	// Runner channel 2 registers, updates, and queries its own launch-bound record.
	runnerRoot2 := filepath.Join(t.TempDir(), "runner-own-sess")
	t.Setenv("TAIA_SESSION_DIR", "")
	hostEnd2, runnerEnd2, err := server.SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	done2 := make(chan error, 1)
	go func() { done2 <- server.RelayRunnerBroker(ctx2, hostEnd2, hostSocket, config) }()
	t.Cleanup(func() { cancel2(); _ = runnerEnd2.Close(); <-done2 })
	if err := os.Mkdir(runnerRoot2, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", runnerRoot2)
	t.Setenv(config.RunnerChannel.FDEnv, strconv.Itoa(int(runnerEnd2.Fd())))

	created, err := subagent.RegisterOwnSession(context.Background(), server.RunnerRegistryRequest{
		SessionID: "runner-sess-1", Name: "runner-w1", Provider: "codex", Model: "luna", Tier: "low", Status: "running",
	})
	if err != nil || created.LaunchID == "" || created.HarnessType != "runner" {
		t.Fatalf("RegisterOwnSession = (%+v, %v)", created, err)
	}
	updated, err := subagent.UpdateOwnSession(context.Background(), server.RunnerRegistryRequest{
		Status: "completed", Turn: 2, Response: "done",
	})
	if err != nil || updated.Status != "completed" || updated.Turn != 2 || updated.LaunchID != created.LaunchID {
		t.Fatalf("UpdateOwnSession = (%+v, %v)", updated, err)
	}
	queried, err := subagent.QueryOwnSession(context.Background(), "")
	if err != nil || queried.ID != "runner-sess-1" || queried.Response != "done" {
		t.Fatalf("QueryOwnSession = (%+v, %v)", queried, err)
	}

	// Verify immutable host record stayed intact and LaunchID preserved on disk.
	storedOnHost, err := hostStore.Get("runner-sess-1")
	if err != nil || storedOnHost.LaunchID != created.LaunchID || storedOnHost.Turn != 2 {
		t.Fatalf("storedOnHost = (%+v, %v)", storedOnHost, err)
	}
}

func TestRunnerModeRejectsCrossSessionLifecycleAndHostRegistry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	runnerRoot := filepath.Join(t.TempDir(), "runner-sess")
	if err := os.Mkdir(runnerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", runnerRoot)

	if canManageTarget("", &subagent.Session{ID: "any"}) {
		t.Fatal("canManageTarget returned true in runner mode")
	}

	for _, args := range [][]string{
		{"stop", "--name", "other"},
		{"stop", "--all"},
		{"resume", "--name", "other", "next"},
		{"list"},
		{"delete", "--name", "other"},
	} {
		cmd := newAgentCmd()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("harnez agent %v unexpectedly succeeded in runner mode", args)
		}
	}

	if err := stopSessionWithReader(&cobra.Command{}, nil, &subagent.Session{ID: "other"}, nil); err == nil || !strings.Contains(err.Error(), "host-only") {
		t.Fatalf("stopSessionWithReader in runner mode err = %v, want host-only error", err)
	}
	if err := runResume(context.Background(), &cobra.Command{}, agentDeps{}, resumeRequest{Name: "other"}); err == nil || !strings.Contains(err.Error(), "host-only") {
		t.Fatalf("runResume in runner mode err = %v, want host-only error", err)
	}
	if err := runAgentStats(os.Stdout, agentStatsOptions{Days: 7}); err == nil || !strings.Contains(err.Error(), "host-only") {
		t.Fatalf("runAgentStats in runner mode err = %v, want host-only error", err)
	}
}
