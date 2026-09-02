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
	if got.PermissionDecision != "allow" {
		t.Errorf("PermissionDecision = %q, want allow", got.PermissionDecision)
	}
	want := "harnez exec --tool git -- bash -c 'git status'"
	if got.UpdatedInput["command"] != want {
		t.Errorf("UpdatedInput[command] = %q, want %q", got.UpdatedInput["command"], want)
	}
}

func TestRunCodexHooksHook_SkipsAlreadyRouted(t *testing.T) {
	original := "harnez exec --tool git -- bash -c 'git status'"
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
	if got.PermissionDecision != "allow" {
		t.Errorf("PermissionDecision = %q, want allow", got.PermissionDecision)
	}
	if len(got.UpdatedInput) != 0 {
		t.Errorf("expected no rewrite for already-routed command, got %v", got.UpdatedInput)
	}
}

func TestRunCodexHooksHook_SkipsEmptyCommand(t *testing.T) {
	in := bytes.NewBufferString(`{"tool_name":"Bash","tool_input":{"command":""}}`)
	var out bytes.Buffer
	if err := runCodexHooksHook(in, &out); err != nil {
		t.Fatalf("runCodexHooksHook: %v", err)
	}
	if got := out.String(); got != "{\"permissionDecision\":\"allow\"}\n" {
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
	want := "harnez exec --tool git -- bash -c 'git status && echo done'"
	if got.UpdatedInput["command"] != want {
		t.Errorf("UpdatedInput[command] = %q, want %q", got.UpdatedInput["command"], want)
	}
}
