package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/server"
)

func TestPrintServerSystemdStatusShowsOwnerAndHandoff(t *testing.T) {
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	printServerSystemdStatus(&output, cfg, server.SystemdStatus{Available: true, Installed: true, Active: true, MainPID: 100, SocketPID: 200})
	for _, want := range []string{"MainPID=100", "socket owner: another process (PID 200)", "systemctl --user restart " + cfg.SystemdUnitName} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("status output missing %q:\n%s", want, output.String())
		}
	}
}

func TestStopLocalServerChoosesSystemdOwner(t *testing.T) {
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	called := false
	message, err := stopLocalServer(context.Background(), cfg, "", 42, server.SystemdStatus{Available: true, Installed: true, MainPID: 42}, func(context.Context) (bool, error) {
		called = true
		return true, nil
	})
	if err != nil || !called || !strings.Contains(message, "stopped "+cfg.SystemdUnitName) {
		t.Fatalf("stopLocalServer() = %q, %v; systemd called=%t", message, err, called)
	}
}

func TestStopLocalServerForegroundAndNotRunning(t *testing.T) {
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "server.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	shutdown := make(chan struct{})
	httpServer := &http.Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /shutdown", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		close(shutdown)
		go func() { _ = httpServer.Shutdown(context.Background()) }()
	})
	httpServer.Handler = mux
	go func() { _ = httpServer.Serve(listener) }()
	message, err := stopLocalServer(context.Background(), cfg, socket, 99, server.SystemdStatus{}, nil)
	if err != nil || !strings.Contains(message, "shutdown requested") {
		t.Fatalf("foreground stop = %q, %v", message, err)
	}
	select {
	case <-shutdown:
	case <-time.After(time.Second):
		t.Fatal("shutdown request was not received")
	}
	_ = httpServer.Close()

	message, err = stopLocalServer(context.Background(), cfg, socket+"-missing", 0, server.SystemdStatus{}, nil)
	if err != nil || message != "server not running" {
		t.Fatalf("idle stop = %q, %v", message, err)
	}
}

func TestStopLocalServerReportsWaitTimeout(t *testing.T) {
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.ShutdownTimeout = 2 * time.Millisecond
	cfg.StartupPoll = time.Millisecond
	socket := filepath.Join(t.TempDir(), "server.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /shutdown", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
	go func() { _ = http.Serve(listener, mux) }()
	_, err = stopLocalServer(context.Background(), cfg, socket, 99, server.SystemdStatus{}, nil)
	if err == nil || !strings.Contains(err.Error(), "timed out waiting for server socket") {
		t.Fatalf("timeout error = %v", err)
	}
	_ = os.Remove(socket)
}

func TestPrintServerSystemdStatusSuggestsInstallWhenUnitIsMissing(t *testing.T) {
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	printServerSystemdStatus(&output, cfg, server.SystemdStatus{Available: true, MainPID: 0, SocketPID: 200})
	if !strings.Contains(output.String(), "harnez server install") {
		t.Fatalf("missing install suggestion: %q", output.String())
	}
	if strings.Contains(output.String(), "systemctl --user restart") {
		t.Fatalf("uninstalled unit got restart suggestion: %q", output.String())
	}
}

func TestPrintServerSystemdStatusExplainsUsageFallback(t *testing.T) {
	var output bytes.Buffer
	printServerSystemdStatus(&output, server.Config{}, server.SystemdStatus{SocketPID: 200})
	if !strings.Contains(output.String(), "harnez usage -w") || !strings.Contains(output.String(), "PID 200") {
		t.Fatalf("unsupported status output = %q", output.String())
	}
}

func TestRestartLocalServerCases(t *testing.T) {
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	// Case 1: Systemd-owned running server -> restarts unit
	restartedUnit := false
	msg, err := restartLocalServer(context.Background(), cfg, "", 42, server.SystemdStatus{Available: true, Installed: true, MainPID: 42}, func(context.Context) (bool, error) {
		restartedUnit = true
		return true, nil
	}, nil, nil)
	if err != nil || !restartedUnit || !strings.Contains(msg, "restarted "+cfg.SystemdUnitName) {
		t.Fatalf("case 1 failed: msg=%q, err=%v, restarted=%t", msg, err, restartedUnit)
	}

	// Case 2: Server not running but unit installed -> starts unit
	startedUnit := false
	msg, err = restartLocalServer(context.Background(), cfg, "", 0, server.SystemdStatus{Available: true, Installed: true, MainPID: 0}, nil, func(context.Context) (bool, error) {
		startedUnit = true
		return true, nil
	}, nil)
	if err != nil || !startedUnit || !strings.Contains(msg, "started "+cfg.SystemdUnitName) {
		t.Fatalf("case 2 failed: msg=%q, err=%v, started=%t", msg, err, startedUnit)
	}

	// Case 3: Server running foreground -> stops it, reports no detached start
	stopped := false
	msg, err = restartLocalServer(context.Background(), cfg, "", 99, server.SystemdStatus{Available: true, Installed: false}, nil, nil, func(context.Context, server.Config, string, int, server.SystemdStatus, func(context.Context) (bool, error)) (string, error) {
		stopped = true
		return "shutdown requested", nil
	})
	if err != nil || !stopped || !strings.Contains(msg, "stopped foreground server (PID 99)") || !strings.Contains(msg, "harnez server") {
		t.Fatalf("case 3 failed: msg=%q, err=%v, stopped=%t", msg, err, stopped)
	}

	// Case 4: Not running, unit not installed -> reports server not running
	msg, err = restartLocalServer(context.Background(), cfg, "", 0, server.SystemdStatus{Available: true, Installed: false}, nil, nil, nil)
	if err != nil || !strings.Contains(msg, "server not running") || !strings.Contains(msg, "harnez server") {
		t.Fatalf("case 4 failed: msg=%q, err=%v", msg, err)
	}
}
