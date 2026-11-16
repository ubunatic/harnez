package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
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

func startRunnerPairBroker(t *testing.T, cfg Config, ingest func(context.Context, TelemetryRecord) error) *os.File {
	t.Helper()
	hostEnd, runnerEnd, err := SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(cfg.RunnerChannel.FDEnv, strconv.Itoa(int(runnerEnd.Fd())))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ServeRunnerBroker(ctx, hostEnd, cfg, ingest) }()
	t.Cleanup(func() {
		cancel()
		_ = runnerEnd.Close()
		<-done
	})
	return runnerEnd
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

	// Connected TCP socket must also be rejected (requires AF_UNIX SOCK_SEQPACKET).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		if c, err := ln.Accept(); err == nil {
			_ = c.Close()
		}
	}()
	tcpConn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer tcpConn.Close()
	tcpFile, err := tcpConn.(*net.TCPConn).File()
	if err != nil {
		t.Fatal(err)
	}
	defer tcpFile.Close()

	rec, err := NewTelemetryRecord("tool_call", json.RawMessage(`{"tool_name":"read"}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "not-an-int", "0", "1", "2", strconv.Itoa(int(regFile.Fd())), strconv.Itoa(int(tcpFile.Fd())), "999999"} {
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
	startRunnerPairBroker(t, cfg, func(_ context.Context, rec TelemetryRecord) error {
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

	hugePayload := []byte(`"` + strings.Repeat("a", int(cfg.UpdateMaxBodyBytes)+16) + `"`)
	hugeRec, err := NewTelemetryRecord("tool_call", hugePayload)
	if err != nil {
		t.Fatal(err)
	}
	if got := SendRunnerTelemetry(context.Background(), cfg, hugeRec); got != UpdateDropped {
		t.Fatalf("oversized telemetry = %q, want dropped", got)
	}

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

func TestRunnerChannelTimeoutDoesNotDesyncFollowUp(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	setupPrivateRunnerRoot(t, cfg)

	blockFirst := make(chan struct{})
	firstEntered := make(chan struct{})
	startRunnerPairBroker(t, cfg, func(_ context.Context, rec TelemetryRecord) error {
		if rec.RecordID == "slow-1" {
			close(firstEntered)
			<-blockFirst
		}
		return nil
	})

	slowCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	slowPayload, _ := json.Marshal(TelemetryRecord{RecordID: "slow-1", Kind: "tool_call", Payload: json.RawMessage(`{"tool_name":"slow"}`)})
	if _, err := SendRunnerRequest(slowCtx, cfg, RunnerRequest{Op: RunnerOpTelemetryIngest, Payload: slowPayload}); err == nil {
		t.Fatal("expected slow request to time out")
	}
	<-firstEntered

	// Follow-up fast request must get its own stream and clean response, never the slow response.
	fastRec := TelemetryRecord{RecordID: "fast-2", Kind: "tool_call", Payload: json.RawMessage(`{"tool_name":"fast"}`)}
	if got := SendRunnerTelemetry(context.Background(), cfg, fastRec); got != UpdateDelivered {
		t.Fatalf("follow-up after timeout = %q, want delivered", got)
	}
	close(blockFirst)
}

func TestRunnerChannelConcurrentChildProcessesOnInheritedFD(t *testing.T) {
	if id := os.Getenv("HARNEZ_TEST_RUNNER_CHILD"); id != "" {
		cfg, err := LoadConfig()
		if err != nil {
			os.Exit(2)
		}
		rec := TelemetryRecord{RecordID: "child-" + id, Kind: "tool_call", Payload: json.RawMessage(`{"tool_name":"worker"}`)}
		if SendRunnerTelemetry(context.Background(), cfg, rec) != UpdateDelivered {
			os.Exit(3)
		}
		os.Exit(0)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	setupPrivateRunnerRoot(t, cfg)

	var mu sync.Mutex
	seen := make(map[string]int)
	runnerEnd := startRunnerPairBroker(t, cfg, func(_ context.Context, rec TelemetryRecord) error {
		mu.Lock()
		defer mu.Unlock()
		seen[rec.RecordID]++
		return nil
	})

	const workers = 6
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestRunnerChannelConcurrentChildProcessesOnInheritedFD$")
			cmd.ExtraFiles = []*os.File{runnerEnd}
			cmd.Env = append(os.Environ(), cfg.RunnerChannel.FDEnv+"=3", "HARNEZ_TEST_RUNNER_CHILD="+strconv.Itoa(idx))
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- errors.New(err.Error() + ": " + string(out))
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != workers {
		t.Fatalf("concurrent runner deliveries = %v, want %d unique records", seen, workers)
	}
}

func TestRunnerChannelSpoolReplayCapsMixedQuarantineAndSiblingIsolation(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.RunnerChannel.ReplayMaxRecords = 3
	cfg.RunnerChannel.ReplayMaxBytes = 2048
	runnerRoot := setupPrivateRunnerRoot(t, cfg)

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
	startRunnerPairBroker(t, cfg, func(_ context.Context, rec TelemetryRecord) error {
		mu.Lock()
		defer mu.Unlock()
		if !collectorUp {
			return ErrRunnerUnavailable
		}
		if rec.RecordID == "poison-1" {
			return errors.Join(errors.New("bad row"), ErrQuarantine)
		}
		ingested = append(ingested, rec.RecordID)
		return nil
	})

	for _, id := range []string{"good-1", "poison-1", "good-2", "good-3"} {
		rec := TelemetryRecord{RecordID: id, Kind: "tool_call", Payload: json.RawMessage(`{"tool_name":"spooled"}`)}
		if id != "good-3" {
			if got := SendRunnerTelemetry(context.Background(), cfg, rec); got != UpdateSpooled {
				t.Fatalf("record %s = %q, want spooled", id, got)
			}
			continue
		}
		raw, _ := json.Marshal(rec)
		spoolDir, _ := cfg.TelemetrySpoolDir()
		if err := AppendTelemetrySpool(spoolDir, raw, cfg.RunnerChannel.SpoolMaxBytes); err != nil {
			t.Fatal(err)
		}
	}

	runnerSpool := filepath.Join(runnerRoot, "state", "telemetry-spool")
	mu.Lock()
	collectorUp = true
	mu.Unlock()

	// Batch 1 (limit 3): delivers good-1 + good-2, quarantines ONLY poison-1, leaves good-3.
	if err := ReplayRunnerSpool(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if n := countJSONFiles(t, runnerSpool); n != 1 {
		t.Fatalf("remaining after batch 1 = %d, want 1", n)
	}
	if q := countJSONFiles(t, filepath.Join(runnerSpool, "quarantine")); q != 1 {
		t.Fatalf("quarantined count = %d, want 1", q)
	}

	// Batch 2: drains good-3 without error.
	if err := ReplayRunnerSpool(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if n := countJSONFiles(t, runnerSpool); n != 0 {
		t.Fatalf("remaining after batch 2 = %d, want 0", n)
	}
	mu.Lock()
	gotCount := len(ingested)
	mu.Unlock()
	if gotCount != 3 {
		t.Fatalf("ingested = %v, want 3 valid records", ingested)
	}
	if _, err := os.Stat(siblingSentinel); err != nil {
		t.Fatalf("sibling spool touched: %v", err)
	}
}

func countJSONFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			n++
		}
	}
	return n
}

func TestRunnerChannelReplayWireByteLimitAnd413NoQuarantine(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	runnerRoot := setupPrivateRunnerRoot(t, cfg)
	// First record fits wire batch; second pushes wrapped JSON over cap so client splits cleanly.
	r1 := TelemetryRecord{RecordID: "byte-1", Kind: "tool_call", Payload: json.RawMessage(`{"tool_name":"a"}`)}
	r2 := TelemetryRecord{RecordID: "byte-2", Kind: "tool_call", Payload: json.RawMessage(`{"tool_name":"b"}`)}
	oneBatch, _ := json.Marshal(RunnerReplayBatch{Records: []TelemetryRecord{r1}})
	twoBatch, _ := json.Marshal(RunnerReplayBatch{Records: []TelemetryRecord{r1, r2}})
	cfg.RunnerChannel.ReplayMaxBytes = int64(len(twoBatch) - 1)
	if int64(len(oneBatch)) > cfg.RunnerChannel.ReplayMaxBytes {
		t.Fatalf("unexpected fixture size")
	}

	spoolDir, err := cfg.TelemetrySpoolDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range []TelemetryRecord{r1, r2} {
		raw, _ := json.Marshal(rec)
		if err := AppendTelemetrySpool(spoolDir, raw, cfg.RunnerChannel.SpoolMaxBytes); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	startRunnerPairBroker(t, cfg, func(_ context.Context, _ TelemetryRecord) error { count++; return nil })
	if err := ReplayRunnerSpool(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if n := countJSONFiles(t, filepath.Join(runnerRoot, "state", "telemetry-spool")); n != 1 || count != 1 {
		t.Fatalf("split batch remaining=%d count=%d, want 1 and 1", n, count)
	}
	// If broker itself rejects 413, record stays spooled and is NOT quarantined.
	if err := os.MkdirAll(filepath.Join(spoolDir, "quarantine"), 0700); err != nil {
		t.Fatal(err)
	}
	cfg.RunnerChannel.ReplayMaxBytes = int64(len(twoBatch) * 2)
	err = applyRunnerReplayResponse(spoolDir, []string{filepath.Join(spoolDir, "keep.json")}, RunnerResponse{Status: http.StatusRequestEntityTooLarge})
	if !errors.Is(err, ErrRunnerUnavailable) {
		t.Fatalf("413 error = %v, want ErrRunnerUnavailable", err)
	}
	if q := countJSONFiles(t, filepath.Join(spoolDir, "quarantine")); q != 0 {
		t.Fatalf("413 unexpectedly quarantined %d files", q)
	}
}
