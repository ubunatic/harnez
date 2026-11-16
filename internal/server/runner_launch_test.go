package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

func TestRunnerLaunchHostRelayAndForwardChild(t *testing.T) {
	if role := os.Getenv("HARNEZ_TEST_LAUNCH_CHILD"); role != "" {
		cfg, err := LoadConfig()
		if err != nil {
			os.Exit(2)
		}
		if role == "strip-check" || role == "cloexec-check" {
			if os.Getenv(cfg.RunnerChannel.FDEnv) != "" || SendRunnerTelemetry(context.Background(), cfg, TelemetryRecord{RecordID: "x", Kind: "tool_call", Payload: json.RawMessage(`{}`)}) != UpdateDropped {
				os.Exit(3)
			}
			if role == "cloexec-check" {
				if _, optErr := syscall.GetsockoptInt(3, syscall.SOL_SOCKET, syscall.SO_TYPE); optErr == nil {
					os.Exit(5)
				}
			}
			os.Exit(0)
		}
		if role == "spawn-plain-child" {
			child := exec.Command(os.Args[0], "-test.run=^TestRunnerLaunchHostRelayAndForwardChild$")
			child.Env = setEnvKey(os.Environ(), cfg.RunnerChannel.FDEnv, "", false)
			child.Env = setEnvKey(child.Env, "HARNEZ_TEST_LAUNCH_CHILD", "cloexec-check", true)
			if err := child.Run(); err != nil {
				os.Exit(6)
			}
			os.Exit(0)
		}
		rec := TelemetryRecord{RecordID: "launch-" + role, Kind: "tool_call", Payload: json.RawMessage(`{"tool_name":"subagent"}`)}
		if got := SendRunnerTelemetry(context.Background(), cfg, rec); got != UpdateDelivered {
			os.Exit(4)
		}
		os.Exit(0)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "run"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("TAIA_SESSION_DIR", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	hostSocket, mu, received := startMockTelemetryHTTP(t, cfg)

	childSess := filepath.Join(t.TempDir(), "child-sess")
	if err := os.Mkdir(childSess, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRunnerLaunchHostRelayAndForwardChild$")
	cmd.Env = append(os.Environ(), "TAIA_SESSION_DIR="+childSess, "HARNEZ_TEST_LAUNCH_CHILD=host")
	runBrokerChildCmd(t, cmd)

	setupPrivateRunnerRoot(t, cfg)
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "run"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	t.Setenv(cfg.RunnerChannel.FDEnv, "not-valid")
	stripCmd := exec.Command(os.Args[0], "-test.run=^TestRunnerLaunchHostRelayAndForwardChild$")
	stripCmd.Env = append(os.Environ(), "HARNEZ_TEST_LAUNCH_CHILD=strip-check")
	stripBroker, err := PrepareCmdRunnerBroker(context.Background(), stripCmd)
	if err != nil || stripBroker != nil {
		t.Fatalf("strip mode broker=%v err=%v, want nil/nil", stripBroker, err)
	}
	if err := stripCmd.Run(); err != nil {
		t.Fatalf("strip child failed: %v", err)
	}

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

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	t.Setenv(cfg.RunnerChannel.FDEnv, strconv.Itoa(int(runnerEnd.Fd())))
	fwdCmd := exec.Command(os.Args[0], "-test.run=^TestRunnerLaunchHostRelayAndForwardChild$")
	fwdCmd.ExtraFiles = []*os.File{devNull} // forces forwarded FD onto 4 (3 + len(ExtraFiles))
	fwdCmd.Env = append(os.Environ(), "HARNEZ_TEST_LAUNCH_CHILD=forwarded")
	runBrokerChildCmd(t, fwdCmd)

	// Verify CLOEXEC on inherited broker FD (M1): runner spawned with ExtraFiles FD=3
	// starts a plain child without PrepareCmdRunnerBroker; child must not see FD 3 open.
	plainParent := exec.Command(os.Args[0], "-test.run=^TestRunnerLaunchHostRelayAndForwardChild$")
	plainParent.ExtraFiles = []*os.File{runnerEnd}
	plainParent.Env = setEnvKey(os.Environ(), cfg.RunnerChannel.FDEnv, "3", true)
	plainParent.Env = setEnvKey(plainParent.Env, "HARNEZ_TEST_LAUNCH_CHILD", "spawn-plain-child", true)
	if out, err := plainParent.CombinedOutput(); err != nil {
		t.Fatalf("cloexec propagation test failed: %v (%s)", err, string(out))
	}

	// Verify PrepareCmdRunnerBroker strips stale FD env on error (M2).
	t.Setenv("TAIA_SESSION_DIR", "")
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, strings.Repeat("long-", 24)))
	t.Setenv(cfg.RunnerChannel.FDEnv, "99")
	errCmd := exec.Command(os.Args[0])
	errCmd.Env = os.Environ()
	if errBroker, err := PrepareCmdRunnerBroker(context.Background(), errCmd); err == nil || errBroker != nil {
		t.Fatalf("expected error and nil handle for invalid socket dir, got broker=%v err=%v", errBroker, err)
	}
	for _, entry := range errCmd.Env {
		if strings.HasPrefix(entry, cfg.RunnerChannel.FDEnv+"=") {
			t.Fatalf("PrepareCmdRunnerBroker error left %s in cmd.Env", cfg.RunnerChannel.FDEnv)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if !slices.Contains(*received, "launch-host") || !slices.Contains(*received, "launch-forwarded") || len(*received) != 2 {
		t.Fatalf("received = %v, want [launch-host launch-forwarded]", *received)
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
	t.Setenv("TAIA_SESSION_DIR", "")
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
	cmd.Env = append(os.Environ(), "TAIA_SESSION_DIR="+sessDir, "HARNEZ_TEST_BWRAP_CHILD=1")
	runBrokerChildCmd(t, cmd)

	mu.Lock()
	defer mu.Unlock()
	if len(*received) != 1 || (*received)[0] != "bwrap-1" {
		t.Fatalf("bwrap relayed = %v, want [bwrap-1]", *received)
	}
}
