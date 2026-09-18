package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/readcard"
)

func TestReadAdaptiveOverridesAndStdin(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_HARNESS", "claude")
	for _, tc := range []struct {
		args  []string
		image bool
	}{
		{[]string{"--auto"}, false},
		{[]string{"--auto", "-I"}, true},
		{[]string{"--auto", "--text"}, false},
		{[]string{"--auto", "--raw"}, false},
		{[]string{"--auto", "-n"}, false},
		{[]string{"--auto", "--image=false"}, false},
	} {
		cmd := newReadCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetIn(strings.NewReader("123\n"))
		args := append(append([]string{}, tc.args...), "--json", "-o", filepath.Join(t.TempDir(), "out.png"))
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
			t.Fatal(err)
		}
		_, image := decoded["files"]
		if image != tc.image {
			t.Errorf("%v: %s", tc.args, out.String())
		}
	}
}

func TestReadCompressionCLIAndExplicitText(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_HARNESS", "claude")
	dir := t.TempDir()
	file := filepath.Join(dir, "sample.go")
	source := "package sample\n\nvar X = 1\n\nvar Y = 2\n"
	if err := os.WriteFile(file, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newReadCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--auto", "--text", "--compress=ws", "--line-numbers=10", file})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1 │ package sample") || !strings.Contains(out.String(), ". │ var X") {
		t.Fatalf("cadence failed: %s", out.String())
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != source {
		t.Fatal("compression changed source file")
	}
	cmd = newReadCmd()
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"-I", "--compress=ast", "--line-numbers=off", "--json", "-o", filepath.Join(dir, "image.png"), file})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var res readcard.RenderResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.OriginalTokenStats == nil || res.OriginalTokenStats.TextBytes <= res.TokenStats.TextBytes {
		t.Fatalf("missing compaction metrics: %+v", res)
	}
}

func TestReadHookNativeProtocolAndRepeat(t *testing.T) {
	enforceRead := true
	dir := t.TempDir()
	path := filepath.Join(dir, "file ' $x.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("line\n", 110)), 0600); err != nil {
		t.Fatal(err)
	}
	invoke := func(session string, input map[string]any) map[string]any {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": session, "tool_name": "Read", "tool_input": input, "cwd": dir})
		var out bytes.Buffer
		if err := runClaudeReadHook(bytes.NewReader(payload), &out, readHookOptions{StateDir: filepath.Join(dir, "state"), EnforceRead: &enforceRead}); err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
			t.Fatal(err)
		}
		return decoded
	}
	resp := invoke("large", map[string]any{"file_path": path, "offset": 10, "limit": 5})
	hook, ok := resp["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("missing hookSpecificOutput in denied response: %v", resp)
	}
	if hook["hookEventName"] != "PreToolUse" || hook["permissionDecision"] != "deny" {
		t.Fatalf("invalid hook contract %v", resp)
	}
	reason, ok := hook["permissionDecisionReason"].(string)
	if !ok {
		t.Fatalf("missing permissionDecisionReason in denied response: %v", hook)
	}
	if !strings.Contains(reason, "harnez read -n -L 10:14 -- '") || !strings.Contains(reason, "'\"'\"'") {
		t.Fatalf("unsafe/missing redirect: %s", reason)
	}
	if err := os.WriteFile(path, []byte("short\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if first := invoke("new-session", map[string]any{"file_path": path}); len(first) != 0 {
		t.Fatalf("pass-through must leave permission decision to harness: %v", first)
	}
	if second := invoke("new-session", map[string]any{"file_path": path}); second["hookSpecificOutput"] == nil {
		t.Fatal("second read not detected")
	}
	if separate := invoke("other-session", map[string]any{"file_path": path}); len(separate) != 0 {
		t.Fatal("session state leaked")
	}
}

func TestSubagentCLIStagesAndLaunches(t *testing.T) {
	dir := t.TempDir()
	cmd := newSubagentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--doc-mode=auto", "--provider=local", "--dir", dir, "--task", "Check output", "--", "cat"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Check output") || !strings.Contains(out.String(), "Concise development rules") || !strings.Contains(out.String(), "harnez read --auto") {
		t.Fatalf("launcher did not receive staged prompt: %s", out.String())
	}
}
