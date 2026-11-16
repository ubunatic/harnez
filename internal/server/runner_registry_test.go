package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"ubunatic.com/harnez/internal/procs"
)

func TestRunnerRegistryLaunchBindingAndRejections(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "run"))
	t.Setenv("TAIA_SESSION_DIR", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	root := setupPrivateRunnerRoot(t, cfg)

	// Missing FD fails closed.
	t.Setenv(cfg.RunnerChannel.FDEnv, "")
	if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnQuery, RunnerRegistryRequest{SessionID: "s1"}, nil); !errors.Is(err, ErrInvalidRunnerChannel) {
		t.Fatalf("missing FD err = %v, want ErrInvalidRunnerChannel", err)
	}

	t.Setenv("TAIA_SESSION_DIR", "")
	hostSocket, err := cfg.Socket()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", root)

	var mu sync.Mutex
	stored := map[string]map[string]any{}
	ln, err := net.Listen("unix", hostSocket)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != cfg.RegistryPath {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var relay RunnerRegistryRelayPayload
		if err := json.NewDecoder(r.Body).Decode(&relay); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch relay.Action {
		case RunnerOpOwnRegister:
			stored[relay.SessionID] = map[string]any{
				"id":        relay.SessionID,
				"launch_id": relay.LaunchID,
				"status":    relay.Request.Status,
			}
			_ = json.NewEncoder(w).Encode(stored[relay.SessionID])
		case RunnerOpOwnUpdate:
			rec := stored[relay.SessionID]
			if relay.Request.Status != "" {
				rec["status"] = relay.Request.Status
			}
			_ = json.NewEncoder(w).Encode(rec)
		case RunnerOpOwnQuery:
			_ = json.NewEncoder(w).Encode(stored[relay.SessionID])
		default:
			http.Error(w, "forbidden", http.StatusForbidden)
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
	t.Cleanup(func() { cancel(); _ = runnerEnd.Close(); <-done })
	t.Setenv(cfg.RunnerChannel.FDEnv, strconv.Itoa(int(runnerEnd.Fd())))

	// Unregistered own_update / own_query fails closed with 403.
	if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnUpdate, RunnerRegistryRequest{SessionID: "s1", Status: "done"}, nil); !errors.Is(err, ErrRunnerForbidden) {
		t.Fatalf("unregistered own_update err = %v, want ErrRunnerForbidden", err)
	}
	if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnQuery, RunnerRegistryRequest{SessionID: "s1"}, nil); !errors.Is(err, ErrRunnerForbidden) {
		t.Fatalf("unregistered own_query err = %v, want ErrRunnerForbidden", err)
	}

	// Forged paths/unknown fields rejected cleanly (400).
	resp, err := SendRunnerRequest(context.Background(), cfg, RunnerRequest{
		Op:      RunnerOpOwnRegister,
		Payload: json.RawMessage(`{"session_id":"s1","stdout_log":"/tmp/evil","store_dir":"/etc"}`),
	})
	if err != nil || resp.Status != http.StatusBadRequest {
		t.Fatalf("unknown fields resp=%+v err=%v, want 400", resp, err)
	}

	// Forged / unsafe session IDs rejected (403).
	for _, badID := range []string{"", "../escape", "a/b", "with space"} {
		if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnRegister, RunnerRegistryRequest{SessionID: badID}, nil); !errors.Is(err, ErrRunnerForbidden) {
			t.Fatalf("bad session_id %q err = %v, want ErrRunnerForbidden", badID, err)
		}
	}

	// Forged PID 1, unbound PID, role escalation, and invalid status rejected (403).
	if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnRegister, RunnerRegistryRequest{
		SessionID: "s1", ProcessPID: 1, ProcessStarttime: 123,
	}, nil); !errors.Is(err, ErrRunnerForbidden) {
		t.Fatalf("forged PID 1 err = %v, want ErrRunnerForbidden", err)
	}
	if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnRegister, RunnerRegistryRequest{
		SessionID: "s1", ProcessPID: os.Getpid(), ProcessStarttime: procs.ProcessStarttime(os.Getpid()),
	}, nil); !errors.Is(err, ErrRunnerForbidden) {
		t.Fatalf("unbound PID err = %v, want ErrRunnerForbidden", err)
	}
	if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnRegister, RunnerRegistryRequest{
		SessionID: "s1", Role: "orchestrator",
	}, nil); !errors.Is(err, ErrRunnerForbidden) {
		t.Fatalf("escalated role err = %v, want ErrRunnerForbidden", err)
	}
	if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnRegister, RunnerRegistryRequest{
		SessionID: "s1", Status: "invented-status",
	}, nil); !errors.Is(err, ErrRunnerForbidden) {
		t.Fatalf("invalid status err = %v, want ErrRunnerForbidden", err)
	}

	// Valid own_register binds launch identity to s1.
	var regOut map[string]any
	if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnRegister, RunnerRegistryRequest{
		SessionID: "s1", Name: "worker-1", Status: "running",
	}, &regOut); err != nil {
		t.Fatalf("own_register s1 failed: %v", err)
	}
	if regOut["id"] != "s1" || strings.TrimSpace(regOut["launch_id"].(string)) == "" {
		t.Fatalf("unexpected own_register result: %+v", regOut)
	}

	// Attempting to register or query/update a second session on same broker FD fails (403).
	if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnRegister, RunnerRegistryRequest{SessionID: "s2"}, nil); !errors.Is(err, ErrRunnerForbidden) {
		t.Fatalf("cross-session register s2 err = %v, want ErrRunnerForbidden", err)
	}
	if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnUpdate, RunnerRegistryRequest{SessionID: "s2", Status: "pwn"}, nil); !errors.Is(err, ErrRunnerForbidden) {
		t.Fatalf("cross-session update s2 err = %v, want ErrRunnerForbidden", err)
	}
	if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnQuery, RunnerRegistryRequest{SessionID: "s2"}, nil); !errors.Is(err, ErrRunnerForbidden) {
		t.Fatalf("cross-session query s2 err = %v, want ErrRunnerForbidden", err)
	}

	// Valid own_update + own_query on bound session succeed.
	var updOut map[string]any
	if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnUpdate, RunnerRegistryRequest{Status: "completed"}, &updOut); err != nil || updOut["status"] != "completed" {
		t.Fatalf("own_update failed: out=%+v err=%v", updOut, err)
	}
	var qryOut map[string]any
	if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnQuery, RunnerRegistryRequest{}, &qryOut); err != nil || qryOut["id"] != "s1" {
		t.Fatalf("own_query failed: out=%+v err=%v", qryOut, err)
	}
}

func TestRunnerLaunchBindChildPIDEnforcesProcessTree(t *testing.T) {
	if os.Getenv("HARNEZ_TEST_REG_CHILD") == "1" {
		cfg, err := LoadConfig()
		if err != nil {
			os.Exit(2)
		}
		pid := os.Getpid()
		start := procs.ProcessStarttime(pid)
		if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnRegister, RunnerRegistryRequest{
			SessionID: "bound-child", ProcessPID: pid, ProcessStarttime: start, Status: "running",
		}, nil); err != nil {
			os.Exit(3)
		}
		// Parent PID or PID 1 must be rejected as foreign/forged provider_pid.
		ppid := os.Getppid()
		if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnUpdate, RunnerRegistryRequest{
			ProviderPID: ppid, ProviderStarttime: procs.ProcessStarttime(ppid),
		}, nil); !errors.Is(err, ErrRunnerForbidden) {
			os.Exit(4)
		}
		// Wrong starttime on self PID must also be rejected.
		if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnUpdate, RunnerRegistryRequest{
			ProviderPID: pid, ProviderStarttime: start + 999999,
		}, nil); !errors.Is(err, ErrRunnerForbidden) {
			os.Exit(5)
		}
		// Dead/nonexistent PID (liveStart == 0) must fail closed.
		if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnUpdate, RunnerRegistryRequest{
			ProviderPID: 99999999, ProviderStarttime: 12345,
		}, nil); !errors.Is(err, ErrRunnerForbidden) {
			os.Exit(6)
		}
		// Valid self ProviderPID inside bound process tree succeeds.
		if err := SendRunnerRegistry(context.Background(), cfg, RunnerOpOwnUpdate, RunnerRegistryRequest{
			ProviderPID: pid, ProviderStarttime: start, Status: "completed",
		}, nil); err != nil {
			os.Exit(7)
		}
		os.Exit(0)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "run"))
	t.Setenv("TAIA_SESSION_DIR", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	hostSocket, err := cfg.Socket()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", hostSocket)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var relay RunnerRegistryRelayPayload
		if err := json.NewDecoder(r.Body).Decode(&relay); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": relay.SessionID, "launch_id": relay.LaunchID})
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	child := exec.Command(os.Args[0], "-test.run=^TestRunnerLaunchBindChildPIDEnforcesProcessTree$")
	child.Env = append(os.Environ(), "HARNEZ_TEST_REG_CHILD=1")
	broker, err := PrepareCmdRunnerBroker(context.Background(), child)
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		_ = broker.Close()
		t.Fatal(err)
	}
	broker.ReleaseChildFD()
	broker.BindChildPID(child.Process.Pid)
	err = child.Wait()
	_ = broker.Close()
	if err != nil {
		t.Fatalf("bound child test failed: %v", err)
	}
}
