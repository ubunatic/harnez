package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestRunnerUsageQueryRelayValidationAndFailClosed(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	setupPrivateRunnerRoot(t, cfg)

	// Missing FD fails closed.
	t.Setenv(cfg.RunnerChannel.FDEnv, "")
	var out map[string]any
	if err := QueryRunnerUsage(context.Background(), cfg, RunnerUsageQuery{Action: RunnerUsageActionLatestSummary}, &out); !errors.Is(err, ErrInvalidRunnerChannel) {
		t.Fatalf("missing FD error = %v, want ErrInvalidRunnerChannel", err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", home+"/run")
	hostSocket, err := cfg.Socket()
	if err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("unix", hostSocket)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != cfg.UsageQueryPath {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var q RunnerUsageQuery
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			http.Error(w, "bad query", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch q.Action {
		case RunnerUsageActionLatestSummary:
			_, _ = w.Write([]byte(`{"ok":true,"action":"latest_summary"}`))
		case RunnerUsageActionProviderQuota:
			if q.Provider == "oversize" {
				_, _ = w.Write([]byte(`{"blob":"` + strings.Repeat("x", int(cfg.RunnerChannel.MaxFrameBytes)) + `"}`))
				return
			}
			_, _ = w.Write([]byte(`{"provider":"` + q.Provider + `"}`))
		default:
			http.Error(w, "unsupported", http.StatusForbidden)
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	hostEnd, runnerEnd, err := SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- RelayRunnerBroker(ctx, hostEnd, hostSocket, cfg) }()
	t.Cleanup(func() {
		cancel()
		_ = runnerEnd.Close()
		if err := <-done; err != nil {
			t.Logf("cleanup broker stop: %v", err)
		}
	})
	t.Setenv(cfg.RunnerChannel.FDEnv, strconv.Itoa(int(runnerEnd.Fd())))

	// Valid query succeeds through relay.
	if err := QueryRunnerUsage(context.Background(), cfg, RunnerUsageQuery{Action: RunnerUsageActionLatestSummary}, &out); err != nil {
		t.Fatalf("latest_summary query failed: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("unexpected query result: %+v", out)
	}

	// Oversize result triggers 413 cleanly.
	err = QueryRunnerUsage(context.Background(), cfg, RunnerUsageQuery{
		Action:   RunnerUsageActionProviderQuota,
		Provider: "oversize",
		Models:   []string{"m1"},
	}, &out)
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize response err = %v, want HTTPStatusError 413", err)
	}

	// Unknown JSON field on wire & forbidden route/action rejected by broker.
	resp, err := SendRunnerRequest(context.Background(), cfg, RunnerRequest{
		Op:      RunnerOpUsageRead,
		Payload: json.RawMessage(`{"action":"latest_summary","db_path":"/etc/passwd"}`),
	})
	if err != nil || resp.Status != http.StatusBadRequest {
		t.Fatalf("unknown field resp=%+v err=%v, want 400", resp, err)
	}
	resp, err = SendRunnerRequest(context.Background(), cfg, RunnerRequest{
		Op:      RunnerOpUsageRead,
		Route:   "/shutdown",
		Payload: json.RawMessage(`{"action":"latest_summary"}`),
	})
	if err != nil || resp.Status != http.StatusForbidden {
		t.Fatalf("non-empty route resp=%+v err=%v, want 403", resp, err)
	}
	if err := ValidateRunnerUsageQuery(cfg, RunnerUsageQuery{Action: "merge_legacy"}); !errors.Is(err, ErrRunnerForbidden) {
		t.Fatalf("unexpected validation err = %v", err)
	}
	if err := ValidateRunnerUsageQuery(cfg, RunnerUsageQuery{
		Action: RunnerUsageActionLatestSummary, Provider: "codex",
	}); err == nil {
		t.Fatal("expected unexpected provider on latest_summary to fail")
	}
	manyModels := make([]string, cfg.RunnerChannel.UsageQueryMaxModels+1)
	for i := range manyModels {
		manyModels[i] = "m"
	}
	if err := ValidateRunnerUsageQuery(cfg, RunnerUsageQuery{
		Action: RunnerUsageActionProviderQuota, Provider: "codex", Models: manyModels,
	}); err == nil {
		t.Fatal("expected over-limit models query to fail")
	}
	if _, err := os.Stat(hostSocket); err != nil {
		t.Fatalf("expected mock socket %s: %v", hostSocket, err)
	}
}
