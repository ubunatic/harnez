package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(context.Background(), "ROLLBACK")
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
