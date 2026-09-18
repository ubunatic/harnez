package main

import (
	"bytes"
	"encoding/json"
	"testing"
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
