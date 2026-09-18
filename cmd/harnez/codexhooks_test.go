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

func TestRunCodexTelemetry_ReconcilesTokenSnapshotsAcrossCompactions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	fixtures := []string{
		`{"hookEventName":"PreCompact","session_id":"fixture-session","model":"gpt-5","token_usage":{"input_tokens":100,"cached_input_tokens":40,"output_tokens":5,"reasoning_tokens":2,"total_tokens":107}}`,
		`{"hookEventName":"PostCompact","session_id":"fixture-session","model":"gpt-5","token_usage":{"input_tokens":20,"cached_input_tokens":10,"output_tokens":1,"reasoning_tokens":0,"total_tokens":21}}`,
		`{"hookEventName":"PreCompact","session_id":"fixture-session","model":"gpt-5","token_usage":{"input_tokens":80,"cached_input_tokens":60,"output_tokens":3,"reasoning_tokens":1,"total_tokens":84}}`,
		`{"hookEventName":"SessionEnd","session_id":"fixture-session","model":"gpt-5","token_usage":{"input_tokens":4,"cached_input_tokens":0,"output_tokens":2,"reasoning_tokens":0,"total_tokens":6}}`,
	}
	for _, fixture := range fixtures {
		if err := runCodexTelemetryAt(bytes.NewBufferString(fixture), dbPath); err != nil {
			t.Fatalf("run fixture %s: %v", fixture, err)
		}
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	var events, snapshots, boundaries int
	if err := db.QueryRow(`SELECT count(*) FROM compaction_events WHERE session_id = ?`, "fixture-session").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM token_snapshots WHERE session_id = ?`, "fixture-session").Scan(&snapshots); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM session_boundaries WHERE session_id = ?`, "fixture-session").Scan(&boundaries); err != nil {
		t.Fatal(err)
	}
	if events != 3 || snapshots != 4 || boundaries != 4 {
		t.Fatalf("events=%d snapshots=%d boundaries=%d, want 3, 4, 4", events, snapshots, boundaries)
	}
	rows, err := db.Query(`SELECT source, input_tokens, cached_input_tokens, uncached_input_tokens, total_tokens FROM token_snapshots WHERE session_id = ? ORDER BY id`, "fixture-session")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	want := []struct {
		source, input, cached, uncached, total any
	}{
		{"precompact", int64(100), int64(40), int64(60), int64(107)},
		{"postcompact", int64(20), int64(10), int64(10), int64(21)},
		{"precompact", int64(80), int64(60), int64(20), int64(84)},
		{"sessionend", int64(4), int64(0), int64(4), int64(6)},
	}
	for i, expected := range want {
		var source string
		var input, cached, uncached, total any
		if !rows.Next() {
			t.Fatalf("snapshot %d missing", i)
		}
		if err := rows.Scan(&source, &input, &cached, &uncached, &total); err != nil {
			t.Fatal(err)
		}
		if source != expected.source || input != expected.input || cached != expected.cached || uncached != expected.uncached || total != expected.total {
			t.Fatalf("snapshot %d = %q %v %v %v %v, want %q %v %v %v %v", i, source, input, cached, uncached, total, expected.source, expected.input, expected.cached, expected.uncached, expected.total)
		}
	}
	if rows.Next() {
		t.Fatal("unexpected extra snapshot")
	}
	var model, revision, status, note string
	var savings sql.NullInt64
	if err := db.QueryRow(`SELECT model, pricing_revision, status, savings_micros, note FROM compaction_economics WHERE session_id = ?`, "fixture-session").Scan(&model, &revision, &status, &savings, &note); err != nil {
		t.Fatalf("compaction economics: %v", err)
	}
	if model != "gpt-5" || revision != telemetry.RecordedPricingRevision || status != "complete" || !savings.Valid || savings.Int64 <= 0 {
		t.Fatalf("economics = %q %q %q %v %q, want complete row with positive savings", model, revision, status, savings, note)
	}

	noCompactPath := filepath.Join(t.TempDir(), "no-compaction.sqlite")
	if err := runCodexTelemetryAt(bytes.NewBufferString(`{"hookEventName":"SessionEnd","session_id":"no-compact","model":"gpt-5","token_usage":{"total_tokens":10}}`), noCompactPath); err != nil {
		t.Fatal(err)
	}
	noCompact, err := sql.Open("sqlite", noCompactPath)
	if err != nil {
		t.Fatal(err)
	}
	defer noCompact.Close()
	if err := noCompact.QueryRow(`SELECT model, pricing_revision, status FROM compaction_economics WHERE session_id = ?`, "no-compact").Scan(&model, &revision, &status); err != nil {
		t.Fatalf("no-compaction economics: %v", err)
	}
	if model != "gpt-5" || revision != telemetry.RecordedPricingRevision || status != "insufficient_data" {
		t.Fatalf("no-compaction economics = %q %q %q", model, revision, status)
	}
}

func TestRunCodexTelemetry_PreservesMissingSnapshotFields(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	payload := `{"hookEventName":"PostCompact","session_id":"nullable-session","token_usage":{"total_tokens":0}}`
	if err := runCodexTelemetryAt(bytes.NewBufferString(payload), dbPath); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var input, cached, total any
	if err := db.QueryRow(`SELECT input_tokens, cached_input_tokens, total_tokens FROM token_snapshots WHERE session_id = ?`, "nullable-session").Scan(&input, &cached, &total); err != nil {
		t.Fatal(err)
	}
	if input != nil || cached != nil || total != int64(0) {
		t.Fatalf("snapshot fields = %v %v %v, want NULL NULL 0", input, cached, total)
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
