package usage

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/server"
)

func TestStatuslineNoQuotaCountsReachServerAcrossProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("Unix HTTP endpoint: full tier")
	}
	drops := &PassiveNoQuotaTracker{}
	requests := make(chan PassiveRequest, 1)
	mux := http.NewServeMux()
	(PassiveUpdateServer{Path: "/passive", NoQuota: drops, Requests: requests, Timeout: time.Second, MaxBodyBytes: 4096}).RegisterRoutes(mux)
	listener, err := net.Listen("unix", filepath.Join(t.TempDir(), "server.sock"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(mux)
	srv.Listener.Close()
	srv.Listener = listener
	srv.Start()
	defer srv.Close()
	for _, tc := range []struct{ provider, payload string }{
		{"agy", `{"model":{"id":"gemini"},"context_window":{"used_percentage":12}}`},
		{"agy", `{}`},
		{"claude", `{"rate_limits":{"five_hour":{}}}`},
		{"claude", `{`}, // malformed JSON is not a missing-quota observation
		{"codex", `{}`}, // unsupported statusline source
	} {
		if got := ObserveStatuslineTo(context.Background(), tc.provider, []byte(tc.payload), listener.Addr().String(), "/passive", time.Second); got != server.UpdateDropped {
			t.Fatalf("result = %v", got)
		}
	}
	if got := drops.NoQuotaSnapshot(); got["agy"] != 2 || got["claude"] != 1 || len(got) != 2 {
		t.Fatalf("counts = %v", got)
	}
	copy := drops.NoQuotaSnapshot()
	copy["agy"] = 100
	if drops.NoQuotaSnapshot()["agy"] != 2 {
		t.Fatal("snapshot aliases mutable state")
	}
	done := make(chan struct{})
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop); <-done })
	go func() {
		defer close(done)
		select {
		case request := <-requests:
			request.Done <- nil
		case <-stop:
		}
	}()
	got := ObserveStatuslineTo(context.Background(), "agy", []byte(`{"quota":{"gemini-weekly":{"remaining_fraction":0.5}}}`), listener.Addr().String(), "/passive", time.Second)
	if got != server.UpdateDelivered || drops.NoQuotaSnapshot()["agy"] != 2 {
		t.Fatalf("quota result=%v counts=%v", got, drops.NoQuotaSnapshot())
	}
}

func TestPassiveNoQuotaMarkersRejectInvalidProvidersAndMixedWindows(t *testing.T) {
	drops := &PassiveNoQuotaTracker{}
	handler := PassiveUpdateServer{NoQuota: drops, MaxBodyBytes: 4096}
	for _, body := range []string{`{"dropped_provider":"unknown"}`, `{"dropped_provider":"agy","windows":[{}]}`} {
		response := httptest.NewRecorder()
		handler.handlePassive(response, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("response=%d", response.Code)
		}
	}
	if drops.NoQuotaSnapshot()["agy"] != 0 {
		t.Fatal("invalid marker counted")
	}
}

func TestDebugBoxShowsPassiveNoQuotaAndUnavailableCounts(t *testing.T) {
	now := time.Now()
	summary := UsageSummary{Agents: []AgentUsage{{AgentID: "agy"}, {AgentID: "claude"}}, PassiveNoQuota: map[string]uint64{"agy": 17}}
	text := strings.Join(buildUsageDebugBox(summary, 120, now).lines, "\n")
	for _, want := range []string{"passive_no_quota (since server start): 17", "passive_no_quota (since server start): unknown"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}
