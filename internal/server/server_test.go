package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type waitingService struct{ started chan struct{} }

func (s waitingService) Run(ctx context.Context) error { close(s.started); <-ctx.Done(); return nil }
func (waitingService) Ready() bool                     { return false }

func TestStatusEndpoint(t *testing.T) {
	response := httptest.NewRecorder()
	statusHandler("test-version").ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/status", nil))
	var got map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || got["health"] != "ok" || got["server"] != "harnez" || got["version"] != "test-version" {
		t.Fatalf("status = %d %v", response.Code, got)
	}
}

func TestStatusEndpointReportsStartingUntilCollectorReady(t *testing.T) {
	response := httptest.NewRecorder()
	statusHandlerReady("test-version", false).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/status", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["health"] != "starting" {
		t.Fatalf("health = %q", got["health"])
	}
}

func TestSecondServerRecognizesHarnezStatus(t *testing.T) {
	port := freePort(t)
	cfg := Config{Host: "127.0.0.1", Port: port, ReadHeaderTimeout: time.Second, ShutdownTimeout: time.Second, HTTPClientTimeout: time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	started := make(chan struct{})
	go func() {
		done <- (Server{Config: cfg, Version: "test", Services: []Service{waitingService{started}}}).Run(ctx)
	}()
	<-started
	if err := (Server{Config: cfg, Version: "test"}).Run(context.Background()); err != nil {
		t.Fatalf("second server: %v", err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("first server: %v", err)
	}
}

func TestOccupiedPortWithoutHarnezStatusFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	cfg := Config{Host: "127.0.0.1", Port: port, ReadHeaderTimeout: time.Second, ShutdownTimeout: time.Second, HTTPClientTimeout: time.Second}
	if err := (Server{Config: cfg, Version: "test"}).Run(context.Background()); err == nil {
		t.Fatal("occupied non-Harnez port unexpectedly succeeded")
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
