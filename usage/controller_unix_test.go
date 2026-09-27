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

func TestRefreshCoalescesAndThrottlesWithoutBypass(t *testing.T) {
	stateDir := t.TempDir()
	runtimeDir := t.TempDir()
	var mu sync.Mutex
	calls := 0
	collector := CollectorFunc(func(ctx context.Context, provider ProviderID) (Snapshot, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		select {
		case <-ctx.Done():
			return Snapshot{}, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
		return Snapshot{Usage: UsageData{Installed: true}}, nil
	})
	opts := Options{StateDir: stateDir, RuntimeDir: runtimeDir, StartIfAbsent: true, Collector: collector}
	owner, err := Open(opts)
	if err != nil {
		t.Fatalf("Open owner: %v", err)
	}
	defer owner.Close()
	var wg sync.WaitGroup
	results := make(chan []Snapshot, 6)
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, err := Open(Options{StateDir: stateDir, RuntimeDir: runtimeDir})
			if err != nil {
				t.Errorf("Open attached client: %v", err)
				return
			}
			snapshots, err := client.Refresh(context.Background(), ProviderClaude)
			if err != nil {
				t.Errorf("Refresh: %v", err)
				return
			}
			results <- snapshots
		}()
	}
	wg.Wait()
	close(results)
	mu.Lock()
	gotCalls := calls
	mu.Unlock()
	if gotCalls != 1 {
		t.Fatalf("collector calls = %d, want 1", gotCalls)
	}
	for snapshots := range results {
		if len(snapshots) != 1 || (snapshots[0].Status != StatusLive && snapshots[0].Status != StatusThrottled) {
			t.Errorf("refresh result = %+v", snapshots)
		}
	}
	throttled, err := owner.Refresh(context.Background(), ProviderClaude)
	if err != nil {
		t.Fatalf("throttled Refresh: %v", err)
	}
	if len(throttled) != 1 || throttled[0].Status != StatusThrottled || throttled[0].Error == nil || throttled[0].Error.Category != ErrorThrottled {
		t.Fatalf("throttled result = %+v", throttled)
	}
}

func TestRefreshPreservesStaleSnapshotAndPublishesSubscription(t *testing.T) {
	stateDir := t.TempDir()
	runtimeDir := t.TempDir()
	writeSnapshotFile(t, stateDir, "agy", `{"schema_version":1,"provider_id":"agy","fetched_at":"2026-09-27T10:00:00Z","status":"live","usage":{"installed":true}}`)
	collector := CollectorFunc(func(context.Context, ProviderID) (Snapshot, error) {
		return Snapshot{}, ProviderError{Category: ErrorProviderFetch, RetryAfter: time.Now().Add(time.Minute)}
	})
	client, err := Open(Options{StateDir: stateDir, RuntimeDir: runtimeDir, StartIfAbsent: true, Collector: collector})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := client.Subscribe(ctx, ProviderAGY)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	select {
	case event := <-events:
		if event.Snapshot.Status != StatusLive {
			t.Fatalf("initial subscription event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("subscription did not receive initial snapshot")
	}
	if _, err := client.Refresh(context.Background(), ProviderAGY); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	select {
	case event := <-events:
		if event.Snapshot.Status != StatusStale || event.Snapshot.Error == nil || event.Snapshot.Error.RetryAfter == nil {
			t.Fatalf("published event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("subscription did not receive refresh event")
	}
}

func TestForegroundControllerShutsDownAfterIdleTimeout(t *testing.T) {
	runtimeDir := t.TempDir()
	client, err := Open(Options{StateDir: t.TempDir(), RuntimeDir: runtimeDir, StartIfAbsent: true, IdleTimeout: 30 * time.Millisecond})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer client.Close()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(runtimeDir, "controller.sock")); os.IsNotExist(err) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("foreground controller socket remained after idle timeout")
}
