package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"ubunatic.com/harnez/internal/agy"
)

func TestAgyHooksApplyAndStatusCmds(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	applyCmd := newAgyHooksApplyCmd()
	var applyOut bytes.Buffer
	applyCmd.SetOut(&applyOut)
	if err := applyCmd.RunE(applyCmd, nil); err != nil {
		t.Fatalf("apply: %v", err)
	}

	path := agy.HooksPath(home)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}

	statusCmd := newAgyHooksStatusCmd()
	var statusOut bytes.Buffer
	statusCmd.SetOut(&statusOut)
	if err := statusCmd.RunE(statusCmd, nil); err != nil {
		t.Fatalf("status: %v", err)
	}
	if got := statusOut.String(); !bytes.Contains([]byte(got), []byte("up to date")) {
		t.Errorf("status output = %q, want up-to-date", got)
	}
}

func TestRunAgyHooksHook_RewritesCommand(t *testing.T) {
	in := bytes.NewBufferString(`{"toolCall":{"name":"run_command","args":{"CommandLine":"git status"}},"stepIdx":1}`)
	var out bytes.Buffer

	if err := runAgyHooksHook(in, &out); err != nil {
		t.Fatalf("runAgyHooksHook: %v", err)
	}

	var got agyPreToolUseOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v (raw=%s)", err, out.String())
	}
	if got.Decision != "allow" {
		t.Errorf("Decision = %q, want allow", got.Decision)
	}
	want := "harnez exec --tool git -- bash -c 'git status'"
	if got.Overwrite.CommandLine != want {
		t.Errorf("Overwrite.CommandLine = %q, want %q", got.Overwrite.CommandLine, want)
	}
}

func TestRunAgyHooksHook_SkipsAlreadyRouted(t *testing.T) {
	original := "harnez exec --tool git -- bash -c 'git status'"
	payload, _ := json.Marshal(map[string]any{
		"toolCall": map[string]any{
			"name": "run_command",
			"args": map[string]any{"CommandLine": original},
		},
	})
	var out bytes.Buffer
	if err := runAgyHooksHook(bytes.NewReader(payload), &out); err != nil {
		t.Fatalf("runAgyHooksHook: %v", err)
	}

	var got agyPreToolUseOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v (raw=%s)", err, out.String())
	}
	if got.Decision != "allow" {
		t.Errorf("Decision = %q, want allow", got.Decision)
	}
	if got.Overwrite.CommandLine != "" {
		t.Errorf("expected no rewrite for already-routed command, got %q", got.Overwrite.CommandLine)
	}
}

func TestRunAgyHooksHook_SkipsEmptyCommand(t *testing.T) {
	in := bytes.NewBufferString(`{"toolCall":{"name":"run_command","args":{"CommandLine":""}}}`)
	var out bytes.Buffer
	if err := runAgyHooksHook(in, &out); err != nil {
		t.Fatalf("runAgyHooksHook: %v", err)
	}
	if got := out.String(); got != "{\"decision\":\"allow\"}\n" {
		t.Errorf("output = %q, want allow no-op envelope", got)
	}
}

func TestRunAgyHooksHook_PreservesShellMetacharacters(t *testing.T) {
	in := bytes.NewBufferString(`{"toolCall":{"name":"run_command","args":{"CommandLine":"git status && echo done"}}}`)
	var out bytes.Buffer
	if err := runAgyHooksHook(in, &out); err != nil {
		t.Fatalf("runAgyHooksHook: %v", err)
	}

	var got agyPreToolUseOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v (raw=%s)", err, out.String())
	}
	want := "harnez exec --tool git -- bash -c 'git status && echo done'"
	if got.Overwrite.CommandLine != want {
		t.Errorf("Overwrite.CommandLine = %q, want %q", got.Overwrite.CommandLine, want)
	}
}
