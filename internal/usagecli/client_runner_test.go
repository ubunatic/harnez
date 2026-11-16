package usagecli

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/usage"
)

func TestLoadLocalUsageRunnerUsesBrokerAndRejectsOfflineSharedStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "run"))

	// Create an incompatible poison file at shared telemetry.sqlite to prove
	// runner mode never opens it directly.
	sharedDB := usage.UsageStorePath()
	if err := os.MkdirAll(filepath.Dir(sharedDB), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sharedDB, []byte("not-a-sqlite-db"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	hostSocket, err := cfg.Socket()
	if err != nil {
		t.Fatal(err)
	}

	wantTime := time.Date(2026, 11, 16, 20, 0, 0, 0, time.UTC)
	ln, err := net.Listen("unix", hostSocket)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != cfg.UsageQueryPath {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		summary := usage.UsageSummary{
			Timestamp: wantTime,
			Agents:    []usage.AgentUsage{{AgentID: "codex", Installed: true, Authenticated: true}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(summary)
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	hostEnd, runnerEnd, err := server.SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.RelayRunnerBroker(ctx, hostEnd, hostSocket, cfg) }()
	t.Cleanup(func() {
		cancel()
		_ = runnerEnd.Close()
		if err := <-done; err != nil {
			_ = err
		}
	})

	runnerRoot := filepath.Join(t.TempDir(), "runner-sess")
	if err := os.Mkdir(runnerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", runnerRoot)
	t.Setenv(cfg.RunnerChannel.FDEnv, strconv.Itoa(int(runnerEnd.Fd())))

	var ensureCalls atomic.Int32
	prevEnsure := EnsureLocalServerFunc
	EnsureLocalServerFunc = func(context.Context, server.Config) ServerCheckResult {
		ensureCalls.Add(1)
		return ServerCheckResult{}
	}
	t.Cleanup(func() { EnsureLocalServerFunc = prevEnsure })

	got, status, err := LoadLocalUsageWithStatus(context.Background(), false)
	if err != nil {
		t.Fatalf("LoadLocalUsageWithStatus(runner online) err = %v", err)
	}
	if status != "" || ensureCalls.Load() != 0 {
		t.Fatalf("expected no host server probe in runner mode (status=%q ensureCalls=%d)", status, ensureCalls.Load())
	}
	if len(got.Agents) != 1 || got.Agents[0].AgentID != "codex" {
		t.Fatalf("unexpected runner usage summary: %+v", got)
	}

	// Offline in runner mode fails closed without touching poisoned shared DB.
	if _, _, err := LoadLocalUsageWithStatus(context.Background(), true); err == nil {
		t.Fatal("expected offline runner mode to fail closed")
	}
}
