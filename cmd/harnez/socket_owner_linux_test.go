//go:build linux

package main

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/testutil/socktest"
)

func TestLocalServerPIDReadsPeerCredentialsFromRealSocket(t *testing.T) {
	socketPath := socktest.Path(t, "server.sock")
	listener, err := socktest.Listen(t, socketPath)
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
		<-done //nolint:errcheck // wait for goroutine or process exit; the value is not needed
	}()

	got := localServerPID(context.Background(), server.Config{HTTPClientTimeout: time.Second}, socketPath)
	if got != os.Getpid() {
		t.Fatalf("localServerPID() = %d, want listening process PID %d", got, os.Getpid())
	}
}
