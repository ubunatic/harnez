package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

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
