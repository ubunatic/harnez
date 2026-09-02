package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"ubunatic.com/harnez/internal/codex"
)

func TestCodexHooksApplyAndStatusCmds(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	applyCmd := newCodexHooksApplyCmd()
	var applyOut bytes.Buffer
	applyCmd.SetOut(&applyOut)
	if err := applyCmd.RunE(applyCmd, nil); err != nil {
		t.Fatalf("apply: %v", err)
	}

	path := codex.HooksPath(home)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
	if got := applyOut.String(); !bytes.Contains([]byte(got), []byte("trust review")) {
		t.Errorf("apply output = %q, want hook-trust note", got)
	}

	statusCmd := newCodexHooksStatusCmd()
	var statusOut bytes.Buffer
	statusCmd.SetOut(&statusOut)
	if err := statusCmd.RunE(statusCmd, nil); err != nil {
		t.Fatalf("status: %v", err)
	}
	if got := statusOut.String(); !bytes.Contains([]byte(got), []byte("up to date")) {
		t.Errorf("status output = %q, want up-to-date", got)
	}
}

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
