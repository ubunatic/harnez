//go:build unix

package usage

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestOpenReadsPersistedSnapshotWithoutController(t *testing.T) {
	stateDir := t.TempDir()
	writeSnapshotFile(t, stateDir, "claude", `{"schema_version":1,"provider_id":"claude","fetched_at":"2026-09-27T10:00:00Z","status":"cached","usage":{"installed":true}}`)
	client, err := Open(Options{StateDir: stateDir, RuntimeDir: filepath.Join(t.TempDir(), "missing")})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	snapshot, err := client.Snapshot(context.Background(), ProviderClaude)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snapshot == nil || snapshot.Status != StatusCached {
		t.Fatalf("Snapshot = %+v, want persisted cached snapshot", snapshot)
	}
}

func TestConcurrentStartPublishesOneSocketAndServesSnapshots(t *testing.T) {
	stateDir := t.TempDir()
	runtimeDir := t.TempDir()
	writeSnapshotFile(t, stateDir, "codex", `{"schema_version":1,"provider_id":"codex","fetched_at":"2026-09-27T10:00:00Z","status":"live","usage":{"installed":true}}`)
	opts := Options{StateDir: stateDir, RuntimeDir: runtimeDir, StartIfAbsent: true}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, err := Open(opts)
			if err == nil {
				err = client.Close()
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Open: %v", err)
		}
	}
	client, err := Open(Options{StateDir: stateDir, RuntimeDir: runtimeDir})
	if err != nil {
		t.Fatalf("Open client: %v", err)
	}
	snapshot, err := client.Snapshot(context.Background(), ProviderCodex)
	if err != nil {
		t.Fatalf("controller Snapshot: %v", err)
	}
	if snapshot == nil || snapshot.ProviderID != ProviderCodex {
		t.Fatalf("controller Snapshot = %+v", snapshot)
	}
	if info, err := os.Stat(filepath.Join(runtimeDir, "controller.sock")); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("controller socket permissions = %v, %v", info, err)
	}
}

func TestStartRecoversStaleSocketAndRejectsMalformedRequest(t *testing.T) {
	stateDir := t.TempDir()
	runtimeDir := t.TempDir()
	stalePath := filepath.Join(runtimeDir, "controller.sock")
	listener, err := net.Listen("unix", stalePath)
	if err != nil {
		t.Fatalf("create stale socket: %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("close stale socket: %v", err)
	}
	client, err := Open(Options{StateDir: stateDir, RuntimeDir: runtimeDir, StartIfAbsent: true})
	if err != nil {
		t.Fatalf("Open stale recovery: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	conn, err := net.Dial("unix", stalePath)
	if err != nil {
		t.Fatalf("dial recovered socket: %v", err)
	}
	if _, err := conn.Write([]byte("not-json\n")); err != nil {
		t.Fatalf("write malformed request: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	var response wireResponse
	err = json.NewDecoder(conn).Decode(&response)
	_ = conn.Close()
	if err == nil {
		t.Fatalf("malformed request unexpectedly decoded response: %+v", response)
	}
}
