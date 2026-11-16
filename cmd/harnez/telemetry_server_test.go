package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/telemetry"
)

func TestTelemetryRouteStoresAndServerDrainsSpool(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	config, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	dbPath, err := telemetry.DefaultDBPath()
	if err != nil {
		t.Fatal(err)
	}
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	service := &telemetryCollectorService{config: config, path: dbPath, interval: time.Hour}
	mux := http.NewServeMux()
	service.RegisterRoutes(mux)
	call := telemetry.ToolCall{CreatedAt: time.Now().UTC(), SessionID: "m3-route-session", AgentID: "codex", ToolName: "route-test", CallType: "hook:post"}
	post := func(call telemetry.ToolCall) []byte {
		t.Helper()
		payload, err := json.Marshal(call)
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(server.TelemetryRecord{Kind: "tool_call", Payload: payload})
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, config.TelemetryPath, bytes.NewReader(post(call))))
	if response.Code != http.StatusNoContent {
		t.Fatalf("telemetry POST status = %d, body %s", response.Code, response.Body)
	}
	second := call
	second.SessionID = "m3-spool-session"
	spoolDir, err := config.TelemetrySpoolDir()
	if err != nil {
		t.Fatal(err)
	}
	if got := server.PostTelemetry(context.Background(), filepath.Join(home, "missing.sock"), config.TelemetryPath, post(second), config.UpdateTimeout, spoolDir, config.TelemetrySpoolMaxBytes); got != server.UpdateSpooled {
		t.Fatalf("PostTelemetry result = %q, want spooled", got)
	}
	if err := service.drainSpool(context.Background()); err != nil {
		t.Fatal(err)
	}
	db, err = telemetry.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, sessionID := range []string{call.SessionID, second.SessionID} {
		rows, err := db.Query(telemetry.Filter{SessionID: sessionID})
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("session %s rows = %d, want 1", sessionID, len(rows))
		}
	}
}

func TestTelemetryObserversDoNotOpenOrWriteStoreDirectly(t *testing.T) {
	for _, name := range []string{"rate.go", "hook.go", "codexhooks.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, forbidden := range []string{"telemetry.Open(", ".Insert(", "InsertTokenSnapshot(", "InsertSessionBoundary(", "InsertCompactionEvent(", "UpdateLatestToolCallMetrics("} {
			if strings.Contains(text, forbidden) {
				t.Errorf("%s still contains direct telemetry write %q", name, forbidden)
			}
		}
	}
}

func TestTelemetrySpoolDrainsPreThenPostTool(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", home)
	config, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "telemetry.sqlite")
	service := &telemetryCollectorService{config: config, path: path}
	dir, err := config.TelemetrySpoolDir()
	if err != nil {
		t.Fatal(err)
	}
	call := telemetry.ToolCall{CreatedAt: time.Now().UTC(), SessionID: "offline-hooks", ToolName: "view_file", AgentID: "agy", CallType: "hook:pre"}
	raw, _ := json.Marshal(call)
	for _, record := range []server.TelemetryRecord{
		{Kind: "tool_call", Payload: raw},
		{Kind: "agy_post_tool", Payload: json.RawMessage(`{"conversationId":"offline-hooks","output":"offline transcript bytes"}`)},
	} {
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := server.AppendTelemetrySpool(dir, data, config.TelemetrySpoolMaxBytes); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.drainSpool(context.Background()); err != nil {
		t.Fatal(err)
	}
	db, err := telemetry.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(telemetry.Filter{SessionID: "offline-hooks"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].OutputBytes == nil || *rows[0].OutputBytes != int64(len("offline transcript bytes")) || rows[0].ActualTokens == nil {
		t.Fatalf("post-tool metrics = %+v", rows)
	}
}

func TestTelemetrySpoolQuarantinesConstraintViolationAndContinues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", home)
	config, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "telemetry.sqlite")
	service := &telemetryCollectorService{config: config, path: path}
	dir, err := config.TelemetrySpoolDir()
	if err != nil {
		t.Fatal(err)
	}
	invalidScore := 0
	badCall := telemetry.ToolCall{CreatedAt: time.Now().UTC(), SessionID: "bad-call", ToolName: "bad", AgentID: "claude", CallType: "internal", Score: &invalidScore}
	badRaw, _ := json.Marshal(badCall)

	goodCall := telemetry.ToolCall{CreatedAt: time.Now().UTC(), SessionID: "good-call", ToolName: "good", AgentID: "claude", CallType: "internal"}
	goodRaw, _ := json.Marshal(goodCall)

	for _, record := range []server.TelemetryRecord{
		{Kind: "tool_call", Payload: badRaw},
		{Kind: "tool_call", Payload: goodRaw},
	} {
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := server.AppendTelemetrySpool(dir, data, config.TelemetrySpoolMaxBytes); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.drainSpool(context.Background()); err != nil {
		t.Fatalf("drainSpool returned error: %v", err)
	}
	db, err := telemetry.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(telemetry.Filter{SessionID: "good-call"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("good-call rows = %d, err = %v", len(rows), err)
	}
	quarantineEntries, err := os.ReadDir(filepath.Join(dir, "quarantine"))
	if err != nil {
		t.Fatal(err)
	}
	if len(quarantineEntries) != 1 {
		t.Fatalf("quarantined entries = %d, want 1", len(quarantineEntries))
	}
}

func TestTelemetrySpoolRateLimitsNoToolCallWarning(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", home)
	config, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	config.TelemetryWarningRateLimit = 100 * time.Millisecond
	path := filepath.Join(home, "telemetry.sqlite")
	service := &telemetryCollectorService{config: config, path: path}
	dir, err := config.TelemetrySpoolDir()
	if err != nil {
		t.Fatal(err)
	}
	// Post-tool update for a session with no prior tool calls
	postRaw := json.RawMessage(`{"conversationId":"missing-session","output":"some output"}`)
	for i := 0; i < 3; i++ {
		record := server.TelemetryRecord{Kind: "agy_post_tool", Payload: postRaw}
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := server.AppendTelemetrySpool(dir, data, config.TelemetrySpoolMaxBytes); err != nil {
			t.Fatal(err)
		}
	}
	// Drain should process all records without failing, rate-limiting the warning
	if err := service.drainSpool(context.Background()); err != nil {
		t.Fatalf("drainSpool error: %v", err)
	}
	remaining, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range remaining {
		if strings.HasSuffix(entry.Name(), ".json") {
			t.Fatalf("unprocessed spool entry found: %s", entry.Name())
		}
	}
}

func TestTelemetryMaintenanceRecordsAndMergeRoute(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "telemetry.sqlite")
	if _, _, err := server.EnsureTelemetrySchema(dbPath); err != nil {
		t.Fatal(err)
	}
	call := telemetry.ToolCall{CreatedAt: time.Now().UTC(), SessionID: "maintenance-legacy", AgentID: "claude", ToolName: "legacy", CallType: "shell"}
	legacyPath := filepath.Join(dir, "legacy.sqlite")
	legacy, err := telemetry.Open(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Insert(call); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(struct {
		DBPath  string    `json:"db_path"`
		Sources []string  `json:"sources"`
		Now     time.Time `json:"now"`
	}{DBPath: dbPath, Sources: []string{legacyPath}, Now: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	body, err := processTelemetryRecordResult(context.Background(), server.TelemetryRecord{Kind: "merge_legacy", Payload: payload}, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response.Output, "tool calls before: 0") || !strings.Contains(response.Output, "tool calls after: 1") {
		t.Fatalf("merge response = %q", response.Output)
	}
	for _, record := range []server.TelemetryRecord{
		{Kind: "cli_invocation", Payload: mustJSON(t, telemetry.CLIInvocation{SessionID: "maintenance-cli", Command: "test"})},
		{Kind: "issue_snapshot", Payload: mustJSON(t, telemetry.IssueStatusSnapshot{ProjectName: "maintenance", OpenCount: 1})},
	} {
		if _, err := processTelemetryRecordResult(context.Background(), record, dbPath); err != nil {
			t.Fatal(err)
		}
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTelemetryOfflineSpoolDoesNotAcquireStoreWriteLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.sqlite")
	db, err := telemetry.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	blocker, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	conn, err := blocker.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() //nolint:errcheck // best-effort cleanup; error not actionable
	if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(context.Background(), "ROLLBACK") //nolint:errcheck // best-effort cleanup; error not actionable
	config, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"kind":"tool_call","payload":{"SessionID":"locked","ToolName":"Read","CallType":"internal"}}`)
	// Delivery must finish while the blocker still owns the write lock.
	got := server.PostTelemetry(context.Background(), filepath.Join(dir, "absent.sock"), config.TelemetryPath, body, config.UpdateTimeout, filepath.Join(dir, "spool"), config.TelemetrySpoolMaxBytes)
	if got != server.UpdateSpooled {
		t.Fatalf("locked-store submission = %q", got)
	}
}

func TestTelemetryReceiptMultiRowReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	payload, err := json.Marshal("receipt-session")
	if err != nil {
		t.Fatal(err)
	}
	record := server.TelemetryRecord{RecordID: "pair-retry", Kind: "agy_compaction", Payload: payload}
	for i := 0; i < 2; i++ {
		if err := processTelemetryRecord(record, path); err != nil {
			t.Fatal(err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT count(*) FROM compaction_events WHERE session_id = ?", "receipt-session").Scan(&count); err != nil || count != 2 {
		t.Fatalf("events = %d, %v", count, err)
	}
}

func TestTelemetryLostAckSpoolReplay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", home)
	config, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "telemetry.sqlite")
	payload, err := json.Marshal(telemetry.ToolCall{SessionID: "lost-ack", AgentID: "test", ToolName: "read", CallType: "hook:post"})
	if err != nil {
		t.Fatal(err)
	}
	record, err := server.NewTelemetryRecord("tool_call", payload)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	service := &telemetryCollectorService{config: config, path: path}
	response := httptest.NewRecorder()
	service.handleRecord(response, httptest.NewRequest(http.MethodPost, config.TelemetryPath, bytes.NewReader(body)))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status: %d", response.Code)
	}
	spool, err := config.TelemetrySpoolDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := server.AppendTelemetrySpool(spool, body, config.TelemetrySpoolMaxBytes); err != nil {
		t.Fatal(err)
	}
	if err := service.drainSpool(context.Background()); err != nil {
		t.Fatal(err)
	}
	db, err := telemetry.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(telemetry.Filter{SessionID: "lost-ack"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %d, %v", len(rows), err)
	}
}

func TestCodexReceiptDeduplicatesSnapshotAndCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry.sqlite")
	raw := []byte(`{"session_id":"codex-retry","hook_event_name":"PostToolUse","tool_name":"Bash","tool_use_id":"call-1","token_usage":{"total_tokens":107,"input_tokens":100,"output_tokens":7}}`)
	prepared, err := server.PrepareCodexTelemetry(raw)
	if err != nil {
		t.Fatal(err)
	}
	record := server.TelemetryRecord{RecordID: "codex-retry-id", Kind: "codex", Payload: prepared}
	for i := 0; i < 2; i++ {
		if err := processTelemetryRecord(record, path); err != nil {
			t.Fatal(err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range []string{"tool_calls", "token_snapshots"} {
		var count int
		if err := db.QueryRow("SELECT count(*) FROM "+table+" WHERE session_id = ?", "codex-retry").Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s count = %d, %v", table, count, err)
		}
	}
}

func startUnixHTTPServer(t *testing.T, socket string, handler http.Handler, readHeaderTimeout time.Duration) *http.Server {
	t.Helper()
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: readHeaderTimeout}
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() { _ = srv.Close() })
	return srv
}

func startRunnerRelayHarness(t *testing.T, config server.Config, runnerRoot, hostSocket string) {
	t.Helper()
	if err := os.Mkdir(runnerRoot, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", runnerRoot)
	hostEnd, runnerEnd, err := server.SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	hostConn, err := net.FileConn(hostEnd)
	_ = hostEnd.Close()
	if err != nil {
		_ = runnerEnd.Close()
		t.Fatal(err)
	}
	t.Setenv(config.RunnerChannel.FDEnv, strconv.Itoa(int(runnerEnd.Fd())))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.RelayRunnerConn(ctx, hostConn, hostSocket, config) }()
	t.Cleanup(func() {
		cancel()
		_ = hostConn.Close()
		_ = runnerEnd.Close()
		<-done
	})
}

func TestRunnerRelayCollectorAndHostAdminBoundary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "run"))
	t.Setenv("TAIA_SESSION_DIR", "")

	config, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	dbPath, err := telemetry.DefaultDBPath()
	if err != nil {
		t.Fatal(err)
	}
	hostSocket, err := config.Socket()
	if err != nil {
		t.Fatal(err)
	}

	service := &telemetryCollectorService{config: config, path: dbPath, interval: time.Hour}
	mux := http.NewServeMux()
	service.RegisterRoutes(mux)
	srv := startUnixHTTPServer(t, hostSocket, mux, config.ReadHeaderTimeout)

	if _, err := server.PostTelemetryCommand(context.Background(), hostSocket, config.TelemetryPath, mustTelemetryRecord("ensure_schema", json.RawMessage(`{}`)), config.HTTPClientTimeout); err != nil {
		t.Fatalf("host ensure_schema failed: %v", err)
	}

	runnerRoot := filepath.Join(t.TempDir(), "runner-session")
	startRunnerRelayHarness(t, config, runnerRoot, hostSocket)

	for _, op := range []string{"/shutdown", "merge_legacy", "ensure_schema"} {
		resp, err := server.SendRunnerRequest(context.Background(), config, server.RunnerRequest{Op: op})
		if err != nil || resp.Status != http.StatusForbidden {
			t.Fatalf("runner op %q got status=%d err=%v, want 403", op, resp.Status, err)
		}
	}
	if got := sendTelemetryRecord("tool_call", telemetry.ToolCall{SessionID: "runner-live", AgentID: "codex", ToolName: "bash", CallType: "hook:post"}); got != server.UpdateDelivered {
		t.Fatalf("sendTelemetryRecord result = %q, want delivered", got)
	}

	_ = srv.Close()
	if got := sendTelemetryRecord("tool_call", telemetry.ToolCall{SessionID: "runner-spooled", AgentID: "codex", ToolName: "read", CallType: "hook:post"}); got != server.UpdateSpooled {
		t.Fatalf("sendTelemetryRecord during outage = %q, want spooled", got)
	}
	spoolEntries, err := os.ReadDir(filepath.Join(runnerRoot, "state", "telemetry-spool"))
	if err != nil {
		t.Fatal(err)
	}
	jsonFiles := 0
	for _, entry := range spoolEntries {
		if filepath.Ext(entry.Name()) == ".json" {
			jsonFiles++
		}
	}
	if jsonFiles != 1 {
		t.Fatalf("runner spool files = %d, want 1", jsonFiles)
	}

	startUnixHTTPServer(t, hostSocket, mux, config.ReadHeaderTimeout)
	if err := server.ReplayRunnerSpool(context.Background(), config); err != nil {
		t.Fatalf("ReplayRunnerSpool: %v", err)
	}
	if err := server.ReplayRunnerSpool(context.Background(), config); err != nil {
		t.Fatalf("second ReplayRunnerSpool: %v", err)
	}

	db, err := telemetry.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, sid := range []string{"runner-live", "runner-spooled"} {
		rows, err := db.Query(telemetry.Filter{SessionID: sid})
		if err != nil || len(rows) != 1 {
			t.Fatalf("session %s rows = %d, %v, want 1", sid, len(rows), err)
		}
	}
}
