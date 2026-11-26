package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/hookdispatch"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/telemetry"
)

// isolateHookEnv points HOME and every XDG base at a temp dir. A scratch HOME
// alone does not isolate hook telemetry: Config.Socket() prefers
// XDG_RUNTIME_DIR, so the runtime dir must be set too (issue 1303 M1b).
func isolateHookEnv(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "run"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	// A broker FD inherited from an agent session wins over the socket (issue
	// 1303 M1b). Clear it so no hook test can reach the live collector (M1c).
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	t.Setenv(cfg.RunnerChannel.FDEnv, "")
	if server.RunnerChannelStatus(cfg) == nil {
		t.Fatalf("broker FD still valid after clearing %s", cfg.RunnerChannel.FDEnv)
	}
	// Issue 1303 M3: a fill armed by a test is stopped when the test ends, so no
	// real timer fires into the next test's state.
	t.Cleanup(server.StopClaudeFills)
	return root
}

// telemetrySinkTo writes each record the way the collector does, in process.
// It stands in for the socket round trip: marshal, NewTelemetryRecord, then
// processTelemetryWrite into the DB.
func telemetrySinkTo(t *testing.T, db *telemetry.DB) func(kind string, payload any) {
	return func(kind string, payload any) {
		body, err := json.Marshal(payload)
		if err != nil {
			t.Errorf("marshal %s payload: %v", kind, err)
			return
		}
		record, err := server.NewTelemetryRecord(kind, body)
		if err != nil {
			t.Errorf("new %s record: %v", kind, err)
			return
		}
		if err := processTelemetryWrite(record, db); err != nil {
			t.Errorf("write %s record: %v", kind, err)
		}
	}
}

// runClaudeCompactChain runs the real Claude hook registry through
// hookdispatch.Dispatch: PreCompact, then PostCompact with the boundary already
// on disk. It returns the telemetry DB path, already closed; the diagnostic note
// sits beside it (issue 1303 M1c).
func runClaudeCompactChain(t *testing.T) string {
	t.Helper()
	isolateHookEnv(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "telemetry.sqlite")
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("telemetry.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	reg, err := claudeHookRegistry(hookDeps{Telemetry: telemetrySinkTo(t, db), StateDir: t.TempDir()})
	if err != nil {
		t.Fatalf("claudeHookRegistry: %v", err)
	}
	budget, err := hookdispatch.LoadBudget()
	if err != nil {
		t.Fatalf("LoadBudget: %v", err)
	}
	opts := hookdispatch.Options{Budget: budget}

	transcript := filepath.Join(dir, "session.jsonl")
	appendLines(t, transcript, claudeAssistantLine(time.Now(), "claude-opus-5-5"))

	if err := hookdispatch.Dispatch(context.Background(), reg, "PreCompact", []byte(claudeHookLine("PreCompact", transcript, true)), io.Discard, opts); err != nil {
		t.Fatalf("PreCompact dispatch: %v", err)
	}
	appendLines(t, transcript, claudeBoundaryLine(time.Now(), 166948, 27143))
	if err := hookdispatch.Dispatch(context.Background(), reg, "PostCompact", []byte(claudeHookLine("PostCompact", transcript, true)), io.Discard, opts); err != nil {
		t.Fatalf("PostCompact dispatch: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close: %v", err)
	}
	return dbPath
}

// TestClaudeCompactDispatchChainFillsPostCompactRow documents that the dispatch,
// sink and writer chain is not the cause of the empty live rows: the failure is
// in what PostCompact sees when it reads the transcript.
func TestClaudeCompactDispatchChainFillsPostCompactRow(t *testing.T) {
	dbPath := runClaudeCompactChain(t)

	rawDB := openRawCompactDB(t, dbPath)
	var input, total sql.NullInt64
	var model string
	if err := rawDB.QueryRow(`SELECT input_tokens, total_tokens, model FROM compaction_events WHERE session_id = ? AND event_type = 'postcompact'`, "claude-compact").Scan(&input, &total, &model); err != nil {
		t.Fatalf("postcompact row: %v", err)
	}
	if !total.Valid || total.Int64 != 27143 {
		t.Errorf("postcompact total_tokens = %v, want 27143", total)
	}
	if model != "claude-opus-5-5" {
		t.Errorf("postcompact model = %q, want claude-opus-5-5", model)
	}
}

// TestHookTelemetryStaysInIsolatedRuntimeDir checks that, under a scratch
// environment, the telemetry socket resolves inside the scratch tree and the
// record is not delivered to a live collector.
func TestHookTelemetryStaysInIsolatedRuntimeDir(t *testing.T) {
	root := isolateHookEnv(t)
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	// An agent session inherits a broker FD, and DispatchTelemetry prefers it over
	// the socket: it reaches the live collector whatever HOME says. Clear it so the
	// send can only reach the scratch socket.
	t.Setenv(cfg.RunnerChannel.FDEnv, "")
	if server.RunnerChannelStatus(cfg) == nil {
		t.Fatalf("broker FD still valid after clearing %s", cfg.RunnerChannel.FDEnv)
	}
	sock, err := cfg.Socket()
	if err != nil {
		t.Fatalf("Socket: %v", err)
	}
	if !strings.HasPrefix(sock, root) {
		t.Fatalf("telemetry socket %q is outside the scratch root %q", sock, root)
	}
	result := sendTelemetryRecordTo(io.Discard, "codex", map[string]string{"session_id": "isolation-check"})
	if result == server.UpdateDelivered {
		t.Fatalf("record reached a collector inside a scratch environment")
	}
}

// TestClaudeCompactDispatchChainWritesDiagnosticNote checks the note the collector
// writes beside the telemetry DB for the same chain (issue 1303 M1c).
func TestClaudeCompactDispatchChainWritesDiagnosticNote(t *testing.T) {
	dbPath := runClaudeCompactChain(t)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(dbPath), "compact-diagnostics.jsonl"))
	if err != nil {
		t.Fatalf("diagnostic note: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var note struct {
		Kind   string `json:"kind"`
		Reason string `json:"reason"`
		Hook   *struct {
			Side string   `json:"side"`
			Keys []string `json:"keys"`
		} `json:"hook"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &note); err != nil {
		t.Fatalf("decode note: %v", err)
	}
	if note.Kind != "postcompact" || note.Reason != "applied" {
		t.Errorf("note kind %q reason %q, want postcompact applied", note.Kind, note.Reason)
	}
	if note.Hook == nil || note.Hook.Side != "hook" || !slices.Contains(note.Hook.Keys, "compact_summary") {
		t.Errorf("note hook %+v, want the hook side with compact_summary among its keys", note.Hook)
	}
}
