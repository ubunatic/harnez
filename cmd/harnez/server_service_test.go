package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/server"
)

func TestPrintServerSystemdStatusShowsOwnerAndHandoff(t *testing.T) {
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	printServerSystemdStatus(&output, cfg, server.SystemdStatus{Available: true, Installed: true, Active: true, MainPID: 100, SocketPID: 200}, localServerStatusResponse{PID: 200})
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
	printServerSystemdStatus(&output, cfg, server.SystemdStatus{Available: true, MainPID: 0, SocketPID: 200}, localServerStatusResponse{PID: 200})
	if !strings.Contains(output.String(), "harnez server install") {
		t.Fatalf("missing install suggestion: %q", output.String())
	}
	if strings.Contains(output.String(), "systemctl --user restart") {
		t.Fatalf("uninstalled unit got restart suggestion: %q", output.String())
	}
}

func TestPrintServerSystemdStatusExplainsUsageFallback(t *testing.T) {
	var output bytes.Buffer
	printServerSystemdStatus(&output, server.Config{}, server.SystemdStatus{SocketPID: 200}, localServerStatusResponse{PID: 200})
	if !strings.Contains(output.String(), "harnez usage -w") || !strings.Contains(output.String(), "PID 200") {
		t.Fatalf("unsupported status output = %q", output.String())
	}
}

func TestPrintServerUptimeAndVersion(t *testing.T) {
	// Matching version with uptime
	var buf1 bytes.Buffer
	start := time.Now().Add(-15 * time.Minute)
	printServerUptimeAndVersion(&buf1, localServerStatusResponse{
		PID:       123,
		Version:   harnez.Version,
		StartedAt: start,
	})
	if !strings.Contains(buf1.String(), "uptime: 15m0s") || !strings.Contains(buf1.String(), "version: "+harnez.Version) {
		t.Errorf("matching version output = %q", buf1.String())
	}
	if strings.Contains(buf1.String(), "restart to update") {
		t.Errorf("matching version shouldn't suggest restart: %q", buf1.String())
	}

	// Mismatched dev version
	var buf2 bytes.Buffer
	printServerUptimeAndVersion(&buf2, localServerStatusResponse{
		PID:       123,
		Version:   "0.1.23-dev",
		StartedAt: start,
	})
	if !strings.Contains(buf2.String(), "version: 0.1.23-dev (restart to update to "+harnez.Version+")") {
		t.Errorf("mismatch output = %q", buf2.String())
	}

	// Unknown uptime (empty StartedAt from older server)
	var buf3 bytes.Buffer
	printServerUptimeAndVersion(&buf3, localServerStatusResponse{
		PID:     123,
		Version: harnez.Version,
	})
	if !strings.Contains(buf3.String(), "uptime: unknown") {
		t.Errorf("unknown uptime output = %q", buf3.String())
	}

	// Server not running (PID 0)
	var buf4 bytes.Buffer
	printServerUptimeAndVersion(&buf4, localServerStatusResponse{
		PID: 0,
	})
	if buf4.Len() != 0 {
		t.Errorf("not running output = %q, want empty", buf4.String())
	}
}

func TestPrintServerStatusJSON(t *testing.T) {
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	// Running under systemd with matching version
	start := time.Now().Add(-100 * time.Second)
	var buf1 bytes.Buffer
	err = printServerStatusJSON(&buf1, cfg, server.SystemdStatus{
		Available: true,
		Installed: true,
		Active:    true,
		MainPID:   500,
		SocketPID: 500,
	}, localServerStatusResponse{
		PID:             500,
		Version:         harnez.Version,
		StartedAt:       start,
		PassiveNoQuota:  map[string]uint64{"agy": 12, "claude": 3},
		PassiveAccepted: map[string]uint64{"agy": 5, "claude": 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	var sj1 serverStatusJSON
	if err := json.Unmarshal(buf1.Bytes(), &sj1); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, buf1.String())
	}
	if !sj1.Running || sj1.PID != 500 || sj1.SocketOwner != "systemd" || sj1.Version != harnez.Version || sj1.VersionMismatch || sj1.UptimeSeconds == nil || *sj1.UptimeSeconds < 90 {
		t.Fatalf("unexpected running json: %+v", sj1)
	}
	if sj1.PassiveNoQuota["agy"] != 12 || sj1.PassiveNoQuota["claude"] != 3 {
		t.Fatalf("missing passive counters: %s", buf1.String())
	}
	if sj1.PassiveAccepted["agy"] != 5 || sj1.PassiveAccepted["claude"] != 2 {
		t.Fatalf("missing accepted counters: %s", buf1.String())
	}
	if sj1.Unit == nil || !sj1.Unit.Active || sj1.Unit.MainPID != 500 {
		t.Fatalf("unexpected unit json: %+v", sj1.Unit)
	}

	// Not running (exit 0, valid JSON with running=false)
	var buf2 bytes.Buffer
	err = printServerStatusJSON(&buf2, cfg, server.SystemdStatus{
		Available: true,
		Installed: true,
		Active:    false,
		MainPID:   0,
		SocketPID: 0,
	}, localServerStatusResponse{PID: 0})
	if err != nil {
		t.Fatal(err)
	}
	var sj2 serverStatusJSON
	if err := json.Unmarshal(buf2.Bytes(), &sj2); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, buf2.String())
	}
	if sj2.Running || sj2.PID != 0 || sj2.SocketOwner != "none" || sj2.VersionMismatch {
		t.Fatalf("unexpected stopped json: %+v", sj2)
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

func TestRunServerLogs(t *testing.T) {
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	// Case 1: Systemd installed & available -> calls logsFn
	var buf1 bytes.Buffer
	calledWithLines := 0
	calledWithFollow := false
	err = runServerLogs(context.Background(), &buf1, cfg, server.SystemdStatus{Available: true, Installed: true}, true, 20, func(_ context.Context, follow bool, lines int) ([]byte, bool, error) {
		calledWithFollow = follow
		calledWithLines = lines
		return []byte("sample log lines\n"), true, nil
	})
	if err != nil || !calledWithFollow || calledWithLines != 20 || buf1.String() != "sample log lines\n" {
		t.Fatalf("case 1 failed: err=%v, follow=%t, lines=%d, out=%q", err, calledWithFollow, calledWithLines, buf1.String())
	}

	// Case 2: Systemd not installed / unavailable -> prints fallback message, exit 0
	var buf2 bytes.Buffer
	err = runServerLogs(context.Background(), &buf2, cfg, server.SystemdStatus{Available: false, Installed: false}, false, 50, nil)
	if err != nil || !strings.Contains(buf2.String(), "foreground server logs are printed to its terminal") {
		t.Fatalf("case 2 failed: err=%v, out=%q", err, buf2.String())
	}
}

func TestPrintServerSystemdStatusReportsStrayServerWithoutKilling(t *testing.T) {
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	for name, status := range map[string]server.SystemdStatus{
		"not installed":     {Available: true, SocketPID: 200},
		"installed, active": {Available: true, Installed: true, Active: true, MainPID: 100, SocketPID: 200},
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			printServerSystemdStatus(&output, cfg, status, localServerStatusResponse{PID: 200})
			for _, want := range []string{"stray process (PID 200)", "harnez server stop"} {
				if !strings.Contains(output.String(), want) {
					t.Errorf("status output missing %q:\n%s", want, output.String())
				}
			}
		})
	}
	var managed bytes.Buffer
	printServerSystemdStatus(&managed, cfg, server.SystemdStatus{Available: true, Installed: true, Active: true, MainPID: 100, SocketPID: 100}, localServerStatusResponse{PID: 100})
	if strings.Contains(managed.String(), "stray") {
		t.Errorf("systemd-owned socket reported as stray:\n%s", managed.String())
	}
}

func TestPrintServerInstallResultWarnsAboutStrayServer(t *testing.T) {
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	printServerInstallResult(&output, cfg, server.SystemdStatus{Available: true, Installed: true, MainPID: 100, SocketPID: 200})
	for _, want := range []string{"installed and started " + cfg.SystemdUnitName, "stray process (PID 200)", "harnez server stop", "systemctl --user restart " + cfg.SystemdUnitName} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("install output missing %q:\n%s", want, output.String())
		}
	}
	var clean bytes.Buffer
	printServerInstallResult(&clean, cfg, server.SystemdStatus{Available: true, Installed: true, MainPID: 100, SocketPID: 100})
	if strings.Contains(clean.String(), "stray") {
		t.Errorf("clean install reported a stray server:\n%s", clean.String())
	}
}
