package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunDistillHook_DisabledByDefault(t *testing.T) {
	t.Setenv(distillAutopipeEnv, "")

	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"go test ./..."}}`)
	var out bytes.Buffer
	if err := runDistillHook(in, &out); err != nil {
		t.Fatalf("runDistillHook() error = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output when disabled, got %q", out.String())
	}
}

func TestRunDistillHook_RewritesNoisyCommand(t *testing.T) {
	t.Setenv(distillAutopipeEnv, "true")

	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"go test ./..."}}`)
	var out bytes.Buffer
	if err := runDistillHook(in, &out); err != nil {
		t.Fatalf("runDistillHook() error = %v", err)
	}

	var got hookOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out.String())
	}
	want := "set -o pipefail; ( go test ./... ) 2>&1 | harnez distill"
	if got.HookSpecificOutput.UpdatedInput["command"] != want {
		t.Errorf("updatedInput.command = %q, want %q", got.HookSpecificOutput.UpdatedInput["command"], want)
	}
	if got.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("hookEventName = %q, want PreToolUse", got.HookSpecificOutput.HookEventName)
	}
}

func TestRunDistillHook_IgnoresNonBashTool(t *testing.T) {
	t.Setenv(distillAutopipeEnv, "true")

	in := strings.NewReader(`{"tool_name":"Read","tool_input":{"command":"go test ./..."}}`)
	var out bytes.Buffer
	if err := runDistillHook(in, &out); err != nil {
		t.Fatalf("runDistillHook() error = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for non-Bash tool, got %q", out.String())
	}
}

func TestRunDistillHook_IgnoresNonNoisyCommand(t *testing.T) {
	t.Setenv(distillAutopipeEnv, "1")

	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls -la"}}`)
	var out bytes.Buffer
	if err := runDistillHook(in, &out); err != nil {
		t.Fatalf("runDistillHook() error = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for non-noisy command, got %q", out.String())
	}
}
