package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestAgyHooksCmd_Hidden(t *testing.T) {
	cmd := newAgyHooksCmd()
	if !cmd.Hidden {
		t.Errorf("expected agy-hooks command to be Hidden: true")
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
	want := "⚙ git status"
	if got.Overwrite.CommandLine != want {
		t.Errorf("Overwrite.CommandLine = %q, want %q", got.Overwrite.CommandLine, want)
	}
}

func TestRunAgyHooksHook_SkipsAlreadyRouted(t *testing.T) {
	original := "⚙ git status"
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
	want := "⚙ bash -c 'git status && echo done'"
	if got.Overwrite.CommandLine != want {
		t.Errorf("Overwrite.CommandLine = %q, want %q", got.Overwrite.CommandLine, want)
	}
}

func TestRunAgyHooksHook_SkipsGearCommand(t *testing.T) {
	original := "⚙ echo 'hello'"
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
		t.Errorf("expected no rewrite for gear command, got %q", got.Overwrite.CommandLine)
	}
}

