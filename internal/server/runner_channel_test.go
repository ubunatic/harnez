package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func setupPrivateRunnerRoot(t *testing.T, cfg Config) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "runner-session")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "shared-state"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(t.TempDir(), "shared-runtime"))
	return root
}

func startRunnerPairServer(t *testing.T, cfg Config, ingest func(context.Context, TelemetryRecord) error) (hostDone <-chan error) {
	t.Helper()
	hostEnd, runnerEnd, err := SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	hostConn, err := net.FileConn(hostEnd)
	_ = hostEnd.Close()
	if err != nil {
		_ = runnerEnd.Close()
		t.Fatal(err)
	}
	t.Setenv(cfg.RunnerChannel.FDEnv, strconv.Itoa(int(runnerEnd.Fd())))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ServeRunnerConn(ctx, hostConn, cfg, ingest)
	}()
	t.Cleanup(func() {
		cancel()
		_ = hostConn.Close()
		_ = runnerEnd.Close()
		<-done
	})
	return done
}

func TestRunnerChannelFailsClosedWithoutValidFD(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	setupPrivateRunnerRoot(t, cfg)
	if _, err := cfg.Socket(); err == nil {
		t.Fatal("expected Config.Socket() to fail closed in runner mode")
	}

	regFile, err := os.CreateTemp(t.TempDir(), "not-a-socket")
	if err != nil {
		t.Fatal(err)
	}
	defer regFile.Close()

	rec, err := NewTelemetryRecord("tool_call", json.RawMessage(`{"tool_name":"read"}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "not-an-int", "0", "1", "2", strconv.Itoa(int(regFile.Fd())), "999999"} {
		t.Setenv(cfg.RunnerChannel.FDEnv, value)
		if _, err := SendRunnerRequest(context.Background(), cfg, RunnerRequest{Op: RunnerOpTelemetryIngest}); !errors.Is(err, ErrInvalidRunnerChannel) {
			t.Fatalf("FD=%q error = %v, want ErrInvalidRunnerChannel", value, err)
		}
		if got := SendRunnerTelemetry(context.Background(), cfg, rec); got != UpdateDropped {
			t.Fatalf("FD=%q SendRunnerTelemetry = %q, want dropped", value, got)
		}
	}
}

func TestRunnerChannelRejectsAdminOpsUnknownFieldsAndOversize(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	setupPrivateRunnerRoot(t, cfg)

	var mu sync.Mutex
	var ingested []TelemetryRecord
	startRunnerPairServer(t, cfg, func(_ context.Context, rec TelemetryRecord) error {
		mu.Lock()
		defer mu.Unlock()
		ingested = append(ingested, rec)
		return nil
	})

	for _, bad := range []RunnerRequest{
		{Op: "/shutdown"},
		{Op: "shutdown"},
		{Op: "merge_legacy", Payload: json.RawMessage(`{"db_path":"/tmp/x.sqlite","sources":[]}`)},
		{Op: "ensure_schema"},
		{Op: RunnerOpTelemetryIngest, Route: "/shutdown", Payload: json.RawMessage(`{"record_id":"r1","kind":"tool_call","payload":{}}`)},
		{Op: RunnerOpTelemetryIngest, Payload: json.RawMessage(`{"record_id":"r2","kind":"merge_legacy","payload":{"db_path":"/tmp/host.sqlite"}}`)},
		{Op: RunnerOpTelemetryIngest, Payload: json.RawMessage(`{"record_id":"r3","kind":"ensure_schema","payload":{}}`)},
		{Op: RunnerOpTelemetryIngest, Payload: json.RawMessage(`{"record_id":"r4","kind":"tool_call","payload":{},"db_path":"/tmp/forged.sqlite"}`)},
	} {
		resp, err := SendRunnerRequest(context.Background(), cfg, bad)
		if err != nil {
			t.Fatalf("SendRunnerRequest(%q) err = %v", bad.Op, err)
		}
		if resp.Status != http.StatusForbidden && resp.Status != http.StatusBadRequest {
			t.Fatalf("request %+v status = %d, want 403/400", bad, resp.Status)
		}
	}
	overBatch := make([]TelemetryRecord, cfg.RunnerChannel.ReplayMaxRecords+1)
	for i := range overBatch {
		overBatch[i] = TelemetryRecord{RecordID: strconv.Itoa(i + 1), Kind: "tool_call", Payload: json.RawMessage(`{}`)}
	}
	batchPayload, _ := json.Marshal(RunnerReplayBatch{Records: overBatch})
	resp, err := SendRunnerRequest(context.Background(), cfg, RunnerRequest{Op: RunnerOpTelemetryReplay, Payload: batchPayload})
	if err != nil || resp.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized replay batch = %+v, err=%v, want 413", resp, err)
	}

	// Oversized telemetry payload > UpdateMaxBodyBytes must be rejected.
	hugePayload := []byte(`"` + strings.Repeat("a", int(cfg.UpdateMaxBodyBytes)+16) + `"`)
	hugeRec, err := NewTelemetryRecord("tool_call", hugePayload)
	if err != nil {
		t.Fatal(err)
	}
	if got := SendRunnerTelemetry(context.Background(), cfg, hugeRec); got != UpdateDropped {
		t.Fatalf("oversized telemetry = %q, want dropped", got)
	}

	// Channel remains usable after rejected admin & oversize attempts.
	validRec, err := NewTelemetryRecord("tool_call", json.RawMessage(`{"tool_name":"bash"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := SendRunnerTelemetry(context.Background(), cfg, validRec); got != UpdateDelivered {
		t.Fatalf("valid SendRunnerTelemetry = %q, want delivered", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(ingested) != 1 || ingested[0].RecordID != validRec.RecordID {
		t.Fatalf("ingested = %+v, want single valid record %q", ingested, validRec.RecordID)
	}
}

func TestRunnerChannelSpoolOutageAndBoundedReplayIsolatesSiblings(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.RunnerChannel.ReplayMaxRecords = 2
	runnerRoot := setupPrivateRunnerRoot(t, cfg)

	// Create a sibling session spool that must never be touched by this runner.
	siblingSpool := filepath.Join(t.TempDir(), "sibling", "state", "telemetry-spool")
	if err := os.MkdirAll(siblingSpool, 0700); err != nil {
		t.Fatal(err)
	}
	siblingSentinel := filepath.Join(siblingSpool, "0001-sibling.json")
	if err := os.WriteFile(siblingSentinel, []byte(`{"record_id":"sib-1","kind":"tool_call","payload":{"tool_name":"sib"}}`), 0600); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	collectorUp := false
	var ingested []string
	startRunnerPairServer(t, cfg, func(_ context.Context, rec TelemetryRecord) error {
		mu.Lock()
		defer mu.Unlock()
		if !collectorUp {
			return ErrRunnerUnavailable
		}
		ingested = append(ingested, rec.RecordID)
		return nil
	})

	// During outage, 3 records must spool into runner's own private spool only.
	for i := 0; i < 3; i++ {
		rec, err := NewTelemetryRecord("tool_call", json.RawMessage(`{"tool_name":"spooled"}`))
		if err != nil {
			t.Fatal(err)
		}
		if got := SendRunnerTelemetry(context.Background(), cfg, rec); got != UpdateSpooled {
			t.Fatalf("record %d result = %q, want spooled", i, got)
		}
	}
	sharedSpoolEntries, _ := os.ReadDir(filepath.Join(os.Getenv("XDG_STATE_HOME"), "harnez", cfg.TelemetrySpoolPath))
	if len(sharedSpoolEntries) != 0 {
		t.Fatalf("host shared spool unexpectedly populated: %d entries", len(sharedSpoolEntries))
	}

	runnerSpool := filepath.Join(runnerRoot, "state", "telemetry-spool")
	jsonCount := func(dir string) int {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".json" {
				n++
			}
		}
		return n
	}
	if n := jsonCount(runnerSpool); n != 3 {
		t.Fatalf("spooled files = %d, want 3", n)
	}

	// Bring collector back up: first bounded replay delivers 2, leaves 1.
	mu.Lock()
	collectorUp = true
	mu.Unlock()
	if err := ReplayRunnerSpool(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if n := jsonCount(runnerSpool); n != 1 {
		t.Fatalf("remaining after first bounded replay = %d, want 1", n)
	}
	if err := ReplayRunnerSpool(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if n := jsonCount(runnerSpool); n != 0 {
		t.Fatalf("remaining after second replay = %d, want 0", n)
	}
	mu.Lock()
	gotCount := len(ingested)
	mu.Unlock()
	if gotCount != 3 {
		t.Fatalf("replayed records = %d, want 3", gotCount)
	}
	if _, err := os.Stat(siblingSentinel); err != nil {
		t.Fatalf("sibling spool was touched or removed: %v", err)
	}
}
