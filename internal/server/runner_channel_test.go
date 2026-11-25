package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/sessioncontract"
	"ubunatic.com/harnez/internal/testutil/socktest"
)

func setupPrivateRunnerRoot(t *testing.T, cfg Config) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "runner-session")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "shared-state"))
	t.Setenv("XDG_RUNTIME_DIR", socktest.Dir(t))
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
		if err := <-done; err != nil {
			t.Logf("cleanup broker stop: %v", err)
		}
	})
	return runnerEnd
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

	const workers = 6
	var mu sync.Mutex
	seen := make(map[string]int)
	var arrived int
	allArrived := make(chan struct{})
	runnerEnd := startRunnerPairBroker(t, cfg, func(_ context.Context, rec TelemetryRecord) error {
		mu.Lock()
		seen[rec.RecordID]++
		arrived++
		if arrived == workers {
			close(allArrived)
		}
		mu.Unlock()
		select {
		case <-allArrived:
			return nil
		case <-time.After(3 * time.Second):
			return fmt.Errorf("concurrency barrier timed out")
		}
	})
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

func TestRunnerStatusClassificationForQuarantine(t *testing.T) {
	if isPermanentHTTPStatus(http.StatusRequestTimeout) || isPermanentHTTPStatus(http.StatusRequestEntityTooLarge) || isPermanentHTTPStatus(http.StatusTooManyRequests) || isPermanentHTTPStatus(http.StatusServiceUnavailable) {
		t.Fatal("transient HTTP statuses must not quarantine")
	}
	if !isPermanentHTTPStatus(http.StatusBadRequest) || !isPermanentHTTPStatus(http.StatusUnprocessableEntity) {
		t.Fatal("permanent 4xx must quarantine")
	}
}

func TestRunnerBrokerEOFClosesIdleStreams(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	hostEnd, runnerEnd, err := SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- ServeRunnerBroker(context.Background(), hostEnd, cfg, func(context.Context, TelemetryRecord) error { return nil })
	}()

	// Hand over a stream that stays idle, then close runnerEnd (EOF on SEQPACKET broker).
	idleConn, err := openPerRequestStream(context.Background(), int(runnerEnd.Fd()), cfg.HTTPClientTimeout)
	if err != nil {
		t.Fatal(err)
	}
	defer idleConn.Close()
	_ = runnerEnd.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("broker EOF returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("broker hung waiting for idle client stream after EOF")
	}
}

func TestExtractSCMStreamConnValidationAndClose(t *testing.T) {
	seqHost, seqClient, err := SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	defer seqHost.Close()
	defer seqClient.Close()

	// Duplicate a descriptor outside UnixRights? Instead verify parse+close on live FDs.
	r1Host, r1Peer, err := SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	defer r1Host.Close()
	defer r1Peer.Close()

	dupFD, err := syscall.Dup(int(r1Peer.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	rightsSeq := syscall.UnixRights(dupFD)
	if _, err := extractSCMStreamConn(rightsSeq, syscall.MSG_CTRUNC); err == nil {
		t.Fatal("expected error for MSG_CTRUNC")
	}
	if _, optErr := syscall.GetsockoptInt(dupFD, syscall.SOL_SOCKET, syscall.SO_TYPE); !errors.Is(optErr, syscall.EBADF) {
		t.Fatalf("MSG_CTRUNC rejected FD %d remained open (err=%v)", dupFD, optErr)
	}

	// Valid UnixRights over SOCK_SEQPACKET (not SOCK_STREAM) in an in-band message
	// so kernel allocates fresh FDs that extractSCMStreamConn must validate and close.
	if err := syscall.Sendmsg(int(seqClient.Fd()), []byte{1}, syscall.UnixRights(int(r1Peer.Fd())), nil, syscall.MSG_DONTWAIT); err != nil {
		t.Fatal(err)
	}
	oob := make([]byte, syscall.CmsgSpace(4*8))
	buf := make([]byte, 8)
	_, oobn, flags, _, err := syscall.Recvmsg(int(seqHost.Fd()), buf, oob, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := extractSCMStreamConn(oob[:oobn], flags); err == nil {
		t.Fatal("expected extractSCMStreamConn to reject SOCK_SEQPACKET FD")
	}
	// Multi-FD rights must also be rejected and both duped FDs closed.
	if err := syscall.Sendmsg(int(seqClient.Fd()), []byte{2}, syscall.UnixRights(int(r1Peer.Fd()), int(r1Peer.Fd())), nil, syscall.MSG_DONTWAIT); err != nil {
		t.Fatal(err)
	}
	_, oobn, flags, _, err = syscall.Recvmsg(int(seqHost.Fd()), buf, oob, 0)
	if err != nil {
		t.Fatal(err)
	}
	scms, _ := syscall.ParseSocketControlMessage(oob[:oobn])
	var parsedFDs []int
	for i := range scms {
		if fds, pErr := syscall.ParseUnixRights(&scms[i]); pErr == nil {
			parsedFDs = append(parsedFDs, fds...)
		}
	}
	if _, err := extractSCMStreamConn(oob[:oobn], flags); err == nil {
		t.Fatal("expected extractSCMStreamConn to reject multi-FD control message")
	}
	for _, fd := range parsedFDs {
		if _, optErr := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_TYPE); !errors.Is(optErr, syscall.EBADF) {
			t.Fatalf("rejected FD %d remained open (err=%v)", fd, optErr)
		}
	}
}

func TestOpenPerRequestStreamTimeoutAndCtxCancel(t *testing.T) {
	hostEnd, runnerEnd, err := SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	defer hostEnd.Close()
	defer runnerEnd.Close()

	// Shrink send buffer and never read hostEnd so Sendmsg hits EAGAIN cleanly.
	_ = syscall.SetsockoptInt(int(runnerEnd.Fd()), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 4096)
	var opened []net.Conn
	defer func() {
		for _, c := range opened {
			_ = c.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	for i := 0; i < 64; i++ {
		c, sendErr := openPerRequestStream(ctx, int(runnerEnd.Fd()), 0)
		if sendErr != nil {
			if !errors.Is(sendErr, ErrRunnerUnavailable) {
				t.Fatalf("saturated sendmsg error = %v, want ErrRunnerUnavailable", sendErr)
			}
			return
		}
		opened = append(opened, c)
	}
	t.Fatal("expected openPerRequestStream to hit saturated EAGAIN timeout")
}

func TestRunnerChannelConsumesSessionContractOperations(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	contract, err := sessioncontract.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{
		RunnerOpTelemetryIngest,
		RunnerOpTelemetryReplay,
		RunnerOpUsageRead,
		RunnerOpOwnRegister,
		RunnerOpOwnUpdate,
		RunnerOpOwnQuery,
	} {
		if !contract.AllowsRunnerOperation(op) {
			t.Fatalf("spec/session_isolation.yaml missing runner operation %q", op)
		}
	}
	for _, action := range cfg.RunnerChannel.AllowedUsageActions {
		if err := ValidateRunnerUsageQuery(cfg, RunnerUsageQuery{
			Action:   action,
			Provider: map[bool]string{true: "codex"}[action == RunnerUsageActionProviderQuota],
			Models:   map[bool][]string{true: {"luna"}}[action == RunnerUsageActionProviderQuota],
		}); err != nil {
			t.Fatalf("spec allowed_usage_actions entry %q failed validation: %v", action, err)
		}
	}
	resp := handleRunnerFrame(context.Background(), cfg, []byte(`{"op":"arbitrary_paths"}`), RunnerBrokerHandlers{})
	if resp.Status != http.StatusForbidden {
		t.Fatalf("disallowed op status = %d, want 403", resp.Status)
	}
}

func TestDispatchTelemetryUsesBrokerFDWithoutRunnerDirAndSkipsHostSpool(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.UpdateTimeout = 200 * time.Millisecond
	stateHome := t.TempDir()
	runtimeDir := socktest.Dir(t)
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)

	hostEnd, runnerEnd, err := SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	defer runnerEnd.Close()
	t.Setenv(cfg.RunnerChannel.FDEnv, strconv.Itoa(int(runnerEnd.Fd())))

	var mu sync.Mutex
	var delivered []string
	failBroker := false
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ServeRunnerBroker(ctx, hostEnd, cfg, func(_ context.Context, rec TelemetryRecord) error {
			mu.Lock()
			defer mu.Unlock()
			if failBroker {
				return errors.New("collector unavailable")
			}
			delivered = append(delivered, rec.RecordID)
			return nil
		})
	}()
	t.Cleanup(func() {
		cancel()
		_ = hostEnd.Close()
		if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("broker stopped with error: %v", err)
		}
	})

	rec, err := NewTelemetryRecord("cli_invocation", json.RawMessage(`{"command":"read"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := DispatchTelemetry(context.Background(), cfg, rec); got != UpdateDelivered {
		t.Fatalf("DispatchTelemetry with inherited broker FD = %q, want delivered", got)
	}
	mu.Lock()
	gotDelivered := len(delivered)
	failBroker = true
	mu.Unlock()
	if gotDelivered != 1 {
		t.Fatalf("broker delivered %d records, want 1", gotDelivered)
	}

	// On broker outage, DispatchTelemetry must not write to host spool on disk.
	rec2, err := NewTelemetryRecord("cli_invocation", json.RawMessage(`{"command":"find"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := DispatchTelemetry(context.Background(), cfg, rec2); got != UpdateDropped {
		t.Fatalf("DispatchTelemetry on broker outage without runner dir = %q, want dropped", got)
	}
	spoolDir := filepath.Join(stateHome, "harnez", cfg.TelemetrySpoolPath)
	if entries, err := os.ReadDir(spoolDir); err == nil && len(entries) > 0 {
		t.Fatalf("host spool %s unexpectedly received %d entries", spoolDir, len(entries))
	}
}
