package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type waitingService struct{ started chan struct{} }

func (s waitingService) Run(ctx context.Context) error { close(s.started); <-ctx.Done(); return nil }
func (waitingService) Ready() bool                     { return true }

func testConfig(t *testing.T) Config {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	return Config{SocketPath: "server.sock", ReadHeaderTimeout: time.Second, ShutdownTimeout: time.Second, HTTPClientTimeout: time.Second, TakeoverTimeout: time.Second}
}

func TestStatusEndpointReportsVersionAndBuildIdentity(t *testing.T) {
	response := httptest.NewRecorder()
	writeStatus(response, "1.2.3", "build-a", true)
	var got statusInfo
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || got.Health != "ok" || got.Server != "harnez" || got.Version != "1.2.3" || got.BuildIdentity != "build-a" || got.PID != os.Getpid() {
		t.Fatalf("status = %d %+v", response.Code, got)
	}
}

func TestPostUpdateDropsWhenServerDoesNotAnswer(t *testing.T) {
	if got := PostUpdate(context.Background(), filepath.Join(t.TempDir(), "server.sock"), "/usage/passive", []byte(`{"windows":[]}`), time.Second); got != UpdateDropped {
		t.Fatalf("result = %q, want dropped", got)
	}
}

func TestServerCreatesPrivateSocketAndEqualBuildReturnsAlreadyRunning(t *testing.T) {
	cfg := testConfig(t)
	socket, _ := cfg.Socket()
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- (Server{Config: cfg, Version: "1.2.3", BuildIdentity: "b", Services: []Service{waitingService{started}}}).Run(ctx)
	}()
	<-started
	info, err := os.Stat(socket)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("socket mode = %o", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(socket))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Fatalf("socket directory mode = %o", dirInfo.Mode().Perm())
	}
	var output bytes.Buffer
	if err := (Server{Config: cfg, Version: "1.2.3", BuildIdentity: "b", Output: &output}).Run(context.Background()); err != nil {
		t.Fatalf("equal server: %v", err)
	}
	if !strings.Contains(output.String(), "already running (v1.2.3)") {
		t.Fatalf("equal output = %q", output.String())
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestNewerBuildTakesOverAndOldServerExitsSuccessfully(t *testing.T) {
	cfg := testConfig(t)
	socket, _ := cfg.Socket()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executableInfo, err := os.Stat(executable)
	if err != nil {
		t.Fatal(err)
	}
	oldIdentity := executableInfo.ModTime().UTC().Format(time.RFC3339Nano)
	newMTime := executableInfo.ModTime().Add(time.Minute)
	newIdentity := newMTime.UTC().Format(time.RFC3339Nano)
	defer func() { _ = os.Chtimes(executable, executableInfo.ModTime(), executableInfo.ModTime()) }()
	oldCtx, stopOld := context.WithCancel(context.Background())
	defer stopOld()
	oldStarted := make(chan struct{})
	oldDone := make(chan error, 1)
	go func() {
		oldDone <- (Server{Config: cfg, Version: "1.2.3", BuildIdentity: oldIdentity, Services: []Service{waitingService{oldStarted}}}).Run(oldCtx)
	}()
	<-oldStarted
	if err := os.Chtimes(executable, executableInfo.ModTime(), newMTime); err != nil {
		t.Fatal(err)
	}
	if status := requestStatus(t, socket); status.BuildIdentity != oldIdentity {
		t.Fatalf("old identity changed after binary mtime advanced: %q", status.BuildIdentity)
	}
	newCtx, stopNew := context.WithCancel(context.Background())
	defer stopNew()
	newStarted := make(chan struct{})
	newDone := make(chan error, 1)
	go func() {
		newDone <- (Server{Config: cfg, Version: "1.2.3", BuildIdentity: newIdentity, Services: []Service{waitingService{newStarted}}}).Run(newCtx)
	}()
	select {
	case err := <-oldDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("old server did not stop")
	}
	<-newStarted
	status := requestStatus(t, socket)
	if status.BuildIdentity != newIdentity {
		t.Fatalf("new identity = %q", status.BuildIdentity)
	}
	stopOld()
	stopNew()
	if err := <-newDone; err != nil {
		t.Fatal(err)
	}
}

func TestNewerBuildHandsOverToSystemdInsteadOfTakingSocket(t *testing.T) {
	cfg := testConfig(t)
	socket, _ := cfg.Socket()
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- (Server{Config: cfg, Version: "1.0.0", BuildIdentity: "a", Services: []Service{waitingService{started}}}).Run(ctx)
	}()
	<-started
	gotPID := 0
	var output bytes.Buffer
	handOver := func(_ context.Context, oldPID int) (bool, error) { gotPID = oldPID; return true, nil }
	if err := (Server{Config: cfg, Version: "1.1.0", BuildIdentity: "a", Output: &output, HandOver: handOver}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotPID != os.Getpid() || !strings.Contains(output.String(), "restarted systemd service (was v1.0.0)") {
		t.Fatalf("hand-over PID %d, output %q", gotPID, output.String())
	}
	if status := requestStatus(t, socket); status.Version != "1.0.0" {
		t.Fatalf("socket was taken: running version %q", status.Version)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestOlderBuildReturnsAlreadyRunning(t *testing.T) {
	cfg := testConfig(t)
	socket, _ := cfg.Socket()
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- (Server{Config: cfg, Version: "2.0.0", BuildIdentity: "a", Services: []Service{waitingService{started}}}).Run(ctx)
	}()
	<-started
	var output bytes.Buffer
	if err := (Server{Config: cfg, Version: "1.9.9", BuildIdentity: "z", Output: &output}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "already running (v2.0.0)") {
		t.Fatalf("older output = %q", output.String())
	}
	if status := requestStatus(t, socket); status.Version != "2.0.0" {
		t.Fatalf("running version = %q", status.Version)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestStaleSocketIsReplacedAndForeignListenerErrors(t *testing.T) {
	cfg := testConfig(t)
	socket, _ := cfg.Socket()
	stale, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	stale.(*net.UnixListener).SetUnlinkOnClose(false)
	if err := stale.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- (Server{Config: cfg, Version: "1.0.0", BuildIdentity: "a", Services: []Service{waitingService{started}}}).Run(ctx)
	}()
	<-started
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	foreign := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "foreign", http.StatusOK) })}
	go func() { _ = foreign.Serve(listener) }()
	defer foreign.Close()
	if err := (Server{Config: cfg, Version: "1.0.0", BuildIdentity: "a"}).Run(context.Background()); err == nil || errors.Is(err, errAlreadyRunning) {
		t.Fatalf("foreign listener result = %v", err)
	}
}

func requestStatus(t *testing.T, socket string) statusInfo {
	t.Helper()
	resp, err := UnixHTTPClient(socket, time.Second).Get("http://unix/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var status statusInfo
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	return status
}
