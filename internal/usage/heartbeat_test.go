package usage

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func skipShort(t *testing.T, why string) {
	t.Helper()
	if testing.Short() {
		t.Skip("full test tier: " + why)
	}
}

type heartbeatClock struct{ now time.Time }

func (c *heartbeatClock) advance(d time.Duration) { c.now = c.now.Add(d) }

func testClientTracker(timeout time.Duration, onFirst ...func()) (*ClientTracker, *heartbeatClock) {
	tracker := NewClientTracker(timeout, onFirst...)
	clock := &heartbeatClock{now: time.Unix(1, 0)}
	tracker.now = func() time.Time { return clock.now }
	return tracker, clock
}

func TestClientTrackerCountsActiveClients(t *testing.T) {
	tracker, clock := testClientTracker(50 * time.Millisecond)

	if got := tracker.Count(); got != 0 {
		t.Fatalf("expected 0 clients initially, got %d", got)
	}

	tracker.Heartbeat("client-1")
	tracker.Heartbeat("client-2")

	if got := tracker.Count(); got != 2 {
		t.Fatalf("expected 2 clients, got %d", got)
	}

	// Refresh client-1, let client-2 expire
	clock.advance(30 * time.Millisecond)
	tracker.Heartbeat("client-1")

	clock.advance(30 * time.Millisecond)
	if got := tracker.Count(); got != 1 {
		t.Fatalf("expected 1 active client after timeout, got %d", got)
	}

	clock.advance(30 * time.Millisecond)
	if got := tracker.Count(); got != 0 {
		t.Fatalf("expected 0 active clients after all timed out, got %d", got)
	}
}

func TestClientTrackerOnFirstClientCallback(t *testing.T) {
	var triggered atomic.Int32
	tracker, clock := testClientTracker(50*time.Millisecond, func() {
		triggered.Add(1)
	})

	// First heartbeat triggers callback
	tracker.Heartbeat("client-1")
	if got := triggered.Load(); got != 1 {
		t.Fatalf("expected callback triggered once, got %d", got)
	}

	// Subsequent heartbeats while active do NOT trigger callback
	tracker.Heartbeat("client-1")
	tracker.Heartbeat("client-2")
	if got := triggered.Load(); got != 1 {
		t.Fatalf("expected callback still 1, got %d", got)
	}

	// Wait for expiry back to 0
	clock.advance(60 * time.Millisecond)
	if got := tracker.Count(); got != 0 {
		t.Fatalf("expected 0 clients, got %d", got)
	}

	// Next heartbeat triggers callback again
	tracker.Heartbeat("client-2")
	if got := triggered.Load(); got != 2 {
		t.Fatalf("expected callback triggered a second time, got %d", got)
	}
}

func TestHeartbeatServerHTTPRoute(t *testing.T) {
	tracker := NewClientTracker(time.Second)
	srv := HeartbeatServer{
		Path:    "/usage/heartbeat",
		Tracker: tracker,
		Timeout: time.Second,
	}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// Valid POST
	req := httptest.NewRequest(http.MethodPost, "/usage/heartbeat", strings.NewReader(`{"client_id":"c1"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := tracker.Count(); got != 1 {
		t.Fatalf("expected 1 active client, got %d", got)
	}

	// Empty body POST generates client ID or accepts
	req2 := httptest.NewRequest(http.MethodPost, "/usage/heartbeat", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 for empty body, got %d: %s", rec2.Code, rec2.Body.String())
	}
	if got := tracker.Count(); got != 2 {
		t.Fatalf("expected 2 active clients, got %d", got)
	}
}

func TestClientRestartWithinTimeoutDoesNotTriggerCatchup(t *testing.T) {
	var triggered atomic.Int32
	// client timeout is 100ms
	tracker, clock := testClientTracker(100*time.Millisecond, func() {
		triggered.Add(1)
	})

	// Initial client connects -> triggers catchup
	tracker.Heartbeat("viewer-1")
	if got := triggered.Load(); got != 1 {
		t.Fatalf("expected 1 trigger on initial connect, got %d", got)
	}

	// Viewer restarts 30ms later with a new client ID (viewer-2).
	// Since 30ms < 100ms, viewer-1 has not timed out, active client count is >= 1,
	// so count never dropped to 0.
	clock.advance(30 * time.Millisecond)
	tracker.Heartbeat("viewer-2")

	// Verify callback did not fire again
	if got := triggered.Load(); got != 1 {
		t.Fatalf("expected trigger count to remain 1 after restart within timeout, got %d", got)
	}

	// Wait another 30ms (60ms from start). viewer-2 sends heartbeat again.
	clock.advance(30 * time.Millisecond)
	tracker.Heartbeat("viewer-2")

	// Wait until viewer-1 has expired (>100ms since start, e.g. 50ms more)
	clock.advance(50 * time.Millisecond)
	// viewer-2 is still active because it sent a heartbeat at 60ms
	if got := tracker.Count(); got != 1 {
		t.Fatalf("expected exactly 1 active client (viewer-2), got %d", got)
	}

	// Still should have never dropped to 0, so triggered remains 1
	if got := triggered.Load(); got != 1 {
		t.Fatalf("expected trigger count to remain 1 throughout seamless restart, got %d", got)
	}
}
