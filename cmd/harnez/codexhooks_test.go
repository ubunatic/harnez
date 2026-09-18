package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
	"ubunatic.com/harnez/internal/telemetry"
)

func TestRunCodexHooksHook_RewritesCommand(t *testing.T) {
	in := bytes.NewBufferString(`{"hookEventName":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status"},"tool_use_id":"1","session_id":"s","turn_id":"t","cwd":"/tmp","permission_mode":"default","model":"gpt"}`)
	var out bytes.Buffer

	if err := runCodexHooksHook(in, &out); err != nil {
		t.Fatalf("runCodexHooksHook: %v", err)
	}

	var got codexPreToolUseOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v (raw=%s)", err, out.String())
	}
	if got.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("HookEventName = %q, want PreToolUse", got.HookSpecificOutput.HookEventName)
	}
	if got.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("PermissionDecision = %q, want allow", got.HookSpecificOutput.PermissionDecision)
	}
	want := "⚙ git status"
	if got.HookSpecificOutput.UpdatedInput["command"] != want {
		t.Errorf("UpdatedInput[command] = %q, want %q", got.HookSpecificOutput.UpdatedInput["command"], want)
	}
}

func TestRunCodexTelemetry_PersistsPostToolResult(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	payload := `{"hook_event_name":"PostToolUse","session_id":"codex-session","tool_use_id":"call-1","tool_name":"Bash","tool_input":{"command":"false"},"tool_output":"failed","success":false,"exit_code":2,"duration_ms":17}`
	if err := runCodexTelemetryAt(bytes.NewBufferString(payload), dbPath); err != nil {
		t.Fatalf("runCodexTelemetryAt: %v", err)
	}
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("telemetry.Open: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(telemetry.Filter{SessionID: "codex-session"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.ExitCode == nil || *row.ExitCode != 2 {
		t.Fatalf("ExitCode = %v, want 2", row.ExitCode)
	}
	if row.DurationMs != 17 {
		t.Errorf("DurationMs = %d, want 17", row.DurationMs)
	}
	if row.OutputBytes == nil || *row.OutputBytes != int64(len("failed")) {
		t.Errorf("OutputBytes = %v, want %d", row.OutputBytes, len("failed"))
	}
	if row.CallType != "hook:failure" {
		t.Errorf("CallType = %q, want hook:failure", row.CallType)
	}
}

func TestRunCodexTelemetry_PersistsCompactionAndBoundary(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	payload := `{"hookEventName":"PreCompact","session_id":"compact-session","turn_id":"turn-1","trigger":"auto","reason":"context_limit","token_usage":{"input_tokens":100,"cached_input_tokens":40,"output_tokens":5,"reasoning_tokens":2,"total_tokens":107}}`
	if err := runCodexTelemetryAt(bytes.NewBufferString(payload), dbPath); err != nil {
		t.Fatalf("runCodexTelemetryAt: %v", err)
	}
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("telemetry.Open: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("telemetry.Close: %v", err)
	}
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer rawDB.Close()
	var eventType, trigger, reason string
	var total int64
	if err := rawDB.QueryRow(`SELECT event_type, trigger, reason, total_tokens FROM compaction_events WHERE session_id = ?`, "compact-session").Scan(&eventType, &trigger, &reason, &total); err != nil {
		t.Fatalf("compaction event: %v", err)
	}
	if eventType != "precompact" || trigger != "auto" || reason != "context_limit" || total != 107 {
		t.Fatalf("unexpected compaction row: %q %q %q %d", eventType, trigger, reason, total)
	}
	var boundaryType string
	if err := rawDB.QueryRow(`SELECT boundary_type FROM session_boundaries WHERE session_id = ?`, "compact-session").Scan(&boundaryType); err != nil {
		t.Fatalf("session boundary: %v", err)
	}
	if boundaryType != "precompact" {
		t.Fatalf("boundary_type = %q, want precompact", boundaryType)
	}
}

func TestRunCodexTelemetry_CompactionPayloadsArePartialAndMalformedSafe(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	partial := `{"hook_event_name":"PostCompact","session_id":"partial-session","reason":"unknown"}`
	if err := runCodexTelemetryAt(bytes.NewBufferString(partial), dbPath); err != nil {
		t.Fatalf("partial payload: %v", err)
	}
	if err := runCodexTelemetryAt(bytes.NewBufferString("not json"), dbPath); err != nil {
		t.Fatalf("malformed payload: %v", err)
	}
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("telemetry.Open: %v", err)
	}
	defer db.Close()
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer rawDB.Close()
	var count int
	if err := rawDB.QueryRow(`SELECT count(*) FROM compaction_events WHERE session_id = ?`, "partial-session").Scan(&count); err != nil {
		t.Fatalf("partial count: %v", err)
	}
	if count != 1 {
		t.Fatalf("partial event count = %d, want 1", count)
	}
	var total any
	if err := rawDB.QueryRow(`SELECT total_tokens FROM compaction_events WHERE session_id = ?`, "partial-session").Scan(&total); err != nil {
		t.Fatalf("partial snapshot: %v", err)
	}
	if total != nil {
		t.Fatalf("partial total_tokens = %v, want NULL", total)
	}
	if err := rawDB.QueryRow(`SELECT count(*) FROM compaction_events`).Scan(&count); err != nil {
		t.Fatalf("total count: %v", err)
	}
	if count != 1 {
		t.Fatalf("total event count = %d, want malformed payload ignored", count)
	}
}

func TestRunCodexHooksHook_SkipsAlreadyRouted(t *testing.T) {
	original := "⚙ git status"
	payload, _ := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": original},
	})
	var out bytes.Buffer
	if err := runCodexHooksHook(bytes.NewReader(payload), &out); err != nil {
		t.Fatalf("runCodexHooksHook: %v", err)
	}

	var got codexPreToolUseOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v (raw=%s)", err, out.String())
	}
	if got.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("PermissionDecision = %q, want allow", got.HookSpecificOutput.PermissionDecision)
	}
	if len(got.HookSpecificOutput.UpdatedInput) != 0 {
		t.Errorf("expected no rewrite for already-routed command, got %v", got.HookSpecificOutput.UpdatedInput)
	}
}

func TestRunCodexHooksHook_SkipsEmptyCommand(t *testing.T) {
	in := bytes.NewBufferString(`{"tool_name":"Bash","tool_input":{"command":""}}`)
	var out bytes.Buffer
	if err := runCodexHooksHook(in, &out); err != nil {
		t.Fatalf("runCodexHooksHook: %v", err)
	}
	if got := out.String(); got != "{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"allow\"}}\n" {
		t.Errorf("output = %q, want allow no-op envelope", got)
	}
}

func TestRunCodexHooksHook_PreservesShellMetacharacters(t *testing.T) {
	in := bytes.NewBufferString(`{"tool_name":"Bash","tool_input":{"command":"git status && echo done"}}`)
	var out bytes.Buffer
	if err := runCodexHooksHook(in, &out); err != nil {
		t.Fatalf("runCodexHooksHook: %v", err)
	}

	var got codexPreToolUseOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v (raw=%s)", err, out.String())
	}
	want := "⚙ bash -c 'git status && echo done'"
	if got.HookSpecificOutput.UpdatedInput["command"] != want {
		t.Errorf("UpdatedInput[command] = %q, want %q", got.HookSpecificOutput.UpdatedInput["command"], want)
	}
}

func TestRunCodexHooksHook_SkipsGearCommand(t *testing.T) {
	original := "⚙ echo 'hello'"
	payload, _ := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": original},
	})
	var out bytes.Buffer
	if err := runCodexHooksHook(bytes.NewReader(payload), &out); err != nil {
		t.Fatalf("runCodexHooksHook: %v", err)
	}

	var got codexPreToolUseOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v (raw=%s)", err, out.String())
	}
	if got.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("PermissionDecision = %q, want allow", got.HookSpecificOutput.PermissionDecision)
	}
	if len(got.HookSpecificOutput.UpdatedInput) != 0 {
		t.Errorf("expected no rewrite for gear command, got %v", got.HookSpecificOutput.UpdatedInput)
	}
}
