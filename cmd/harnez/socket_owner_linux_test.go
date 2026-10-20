//go:build linux

package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/server"
)

func TestLocalServerPIDReadsPeerCredentialsFromRealSocket(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "server.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"server":"harnez"}`))
		}))
	}()
	defer func() {
		_ = listener.Close()
		<-done
	}()

	got := localServerPID(context.Background(), server.Config{HTTPClientTimeout: time.Second}, socketPath)
	if got != os.Getpid() {
		t.Fatalf("localServerPID() = %d, want listening process PID %d", got, os.Getpid())
	}
}
