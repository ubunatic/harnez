package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func startMockTelemetryHTTP(t *testing.T, cfg Config) (string, *sync.Mutex, *[]string) {
	t.Helper()
	hostSocket, err := cfg.Socket()
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var received []string
	ln, err := net.Listen("unix", hostSocket)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rec TelemetryRecord
		if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		received = append(received, rec.RecordID)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}), ReadHeaderTimeout: cfg.ReadHeaderTimeout}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return hostSocket, &mu, &received
}

func runBrokerChildCmd(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	broker, err := PrepareCmdRunnerBroker(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		_ = broker.Close()
		t.Fatal(err)
	}
	broker.ReleaseChildFD()
	if err := cmd.Wait(); err != nil {
		_ = broker.Close()
		t.Fatalf("child command failed: %v", err)
	}
	if err := broker.Close(); err != nil {
		t.Fatalf("broker close: %v", err)
	}
}

func TestRunnerLaunchBwrapSurvivesExtraFiles(t *testing.T) {
	if os.Getenv("HARNEZ_TEST_BWRAP_CHILD") == "1" {
		cfg, err := LoadConfig()
		if err != nil {
			os.Exit(2)
		}
		rec := TelemetryRecord{RecordID: "bwrap-1", Kind: "tool_call", Payload: json.RawMessage(`{"tool_name":"bwrap"}`)}
		if SendRunnerTelemetry(context.Background(), cfg, rec) != UpdateDelivered {
			os.Exit(3)
		}
		os.Exit(0)
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		t.Skip("bwrap not installed")
	}
	probe := exec.Command(bwrap, "--ro-bind", "/", "/", "--dev", "/dev", "--proc", "/proc", "--tmpfs", "/tmp", "/bin/true")
	if out, err := probe.CombinedOutput(); err != nil {
		t.Skipf("bwrap unusable in current environment: %v (%s)", err, string(out))
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "run"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	_, mu, received := startMockTelemetryHTTP(t, cfg)

	sessDir := filepath.Join(t.TempDir(), "bwrap-sess")
	if err := os.Mkdir(sessDir, 0700); err != nil {
		t.Fatal(err)
	}
	exeDir := filepath.Dir(os.Args[0])
	cmd := exec.Command(bwrap, "--ro-bind", "/", "/", "--dev", "/dev", "--proc", "/proc", "--tmpfs", "/tmp", "--bind", exeDir, exeDir, "--bind", sessDir, sessDir, "--", os.Args[0], "-test.run=^TestRunnerLaunchBwrapSurvivesExtraFiles$")
	cmd.Env = append(os.Environ(), "HARNEZ_TEST_BWRAP_CHILD=1")
	runBrokerChildCmd(t, cmd)

	mu.Lock()
	defer mu.Unlock()
	if len(*received) != 1 || (*received)[0] != "bwrap-1" {
		t.Fatalf("bwrap relayed = %v, want [bwrap-1]", *received)
	}
}
