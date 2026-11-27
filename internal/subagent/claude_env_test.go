package subagent

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const claudeWindowVar = "CLAUDE_CODE_AUTO_COMPACT_WINDOW"

// fakeCLI puts an executable named name, running the shell body, first on PATH.
func fakeCLI(t *testing.T, name, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// fakeClaudeWindow is a claude stand-in whose result text is the window value
// it received, so tests read the env the child really got.
func fakeClaudeWindow(t *testing.T) {
	t.Helper()
	fakeCLI(t, "claude", `printf '{"type":"result","subtype":"success","is_error":false,"session_id":"s1","result":"window=%s","usage":{"input_tokens":0}}\n' "$CLAUDE_CODE_AUTO_COMPACT_WINDOW"`)
}

// claudeEnvHome points HOME at a temp dir whose harnez config is cfg, and
// clears the caller's window variable.
func claudeEnvHome(t *testing.T, cfg string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".harnez"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".harnez", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(claudeWindowVar, "")
	if err := os.Unsetenv(claudeWindowVar); err != nil {
		t.Fatal(err)
	}
}

var claudeEnvModel = Model{Provider: "claude", Name: "sonnet", Tier: "med"}

func TestClaudeChildGetsWindowOnStartAndResume(t *testing.T) {
	claudeEnvHome(t, "agent:\n  compact_threshold_tokens: 150000\n")
	fakeClaudeWindow(t)
	d := ClaudeDriver{Dir: t.TempDir()}
	r, err := d.Run(context.Background(), RunOptions{Model: claudeEnvModel, Prompt: "p"})
	if err != nil || !strings.Contains(r.Response, "window=150000") {
		t.Fatalf("Run: response %q, err %v; want window=150000", responseOf(r), err)
	}
	r, err = d.Resume(context.Background(), "s1", "p", claudeEnvModel)
	if err != nil || !strings.Contains(r.Response, "window=150000") {
		t.Fatalf("Resume: response %q, err %v; want window=150000", responseOf(r), err)
	}
}

func TestClaudeStreamChildGetsWindowOnStartAndResume(t *testing.T) {
	claudeEnvHome(t, "agent:\n  compact_threshold_tokens: 150000\n")
	fakeClaudeWindow(t)
	d := ClaudeStreamDriver{ClaudeDriver{Dir: t.TempDir()}}
	noEvents := func(Event) {}
	r, err := d.RunStream(context.Background(), RunOptions{Model: claudeEnvModel, Prompt: "p"}, noEvents)
	if err != nil || !strings.Contains(responseOf(r), "window=150000") {
		t.Fatalf("RunStream: response %q, err %v; want window=150000", responseOf(r), err)
	}
	r, err = d.ResumeStream(context.Background(), "s1", "p", claudeEnvModel, noEvents)
	if err != nil || !strings.Contains(responseOf(r), "window=150000") {
		t.Fatalf("ResumeStream: response %q, err %v; want window=150000", responseOf(r), err)
	}
}

func TestClaudeCallerWindowAboveThresholdIsKeptWithWarning(t *testing.T) {
	claudeEnvHome(t, "agent:\n  compact_threshold_tokens: 150000\n")
	fakeClaudeWindow(t)
	t.Setenv(claudeWindowVar, "200000")
	r, err := ClaudeDriver{Dir: t.TempDir()}.Run(context.Background(), RunOptions{Model: claudeEnvModel, Prompt: "p"})
	if err != nil || !strings.Contains(responseOf(r), "window=200000") {
		t.Fatalf("response %q, err %v; want caller window=200000 kept", responseOf(r), err)
	}
	var warn bytes.Buffer
	if env := claudeChildEnv(claudeEnvModel, []string{claudeWindowVar + "=200000"}, &warn); len(env) != 1 {
		t.Fatalf("env %v; want caller env unchanged", env)
	}
	if !strings.Contains(warn.String(), "above the compaction threshold 150000") {
		t.Fatalf("warning %q, want above-threshold warning", warn.String())
	}
}

func TestClaudeCallerWindowBelowThresholdIsKeptSilently(t *testing.T) {
	claudeEnvHome(t, "agent:\n  compact_threshold_tokens: 150000\n")
	fakeClaudeWindow(t)
	t.Setenv(claudeWindowVar, "60000")
	r, err := ClaudeDriver{Dir: t.TempDir()}.Run(context.Background(), RunOptions{Model: claudeEnvModel, Prompt: "p"})
	if err != nil || !strings.Contains(responseOf(r), "window=60000") {
		t.Fatalf("response %q, err %v; want caller window=60000 kept", responseOf(r), err)
	}
	for _, value := range []string{"60000", "150000"} {
		var warn bytes.Buffer
		claudeChildEnv(claudeEnvModel, []string{claudeWindowVar + "=" + value}, &warn)
		if warn.Len() != 0 {
			t.Fatalf("caller %s at or below 150000 warned: %q", value, warn.String())
		}
	}
}

func TestClaudeWindowUnderHundredKStillSet(t *testing.T) {
	claudeEnvHome(t, "agent:\n  compact_threshold_tokens: 80000\n")
	fakeClaudeWindow(t)
	r, err := ClaudeDriver{Dir: t.TempDir()}.Run(context.Background(), RunOptions{Model: claudeEnvModel, Prompt: "p"})
	if err != nil || !strings.Contains(responseOf(r), "window=80000") {
		t.Fatalf("response %q, err %v; want window=80000", responseOf(r), err)
	}
}

func TestClaudeChildEnvUnchangedWhenThresholdDisabled(t *testing.T) {
	claudeEnvHome(t, "agent:\n  compact_thresholds:\n    claude:sonnet:med: 0\n")
	var warn bytes.Buffer
	env := []string{"PATH=/bin"}
	if got := claudeChildEnv(claudeEnvModel, env, &warn); len(got) != 1 || got[0] != "PATH=/bin" {
		t.Fatalf("env %v; want unchanged when no threshold resolves", got)
	}
	if !strings.Contains(warn.String(), claudeWindowVar) {
		t.Fatalf("warning %q; want a note that the window is not set", warn.String())
	}
}

func TestPiChildEnvUnchanged(t *testing.T) {
	claudeEnvHome(t, "agent:\n  compact_threshold_tokens: 150000\n")
	fakeCLI(t, "pi", `printf 'pi-window=%s\n' "$CLAUDE_CODE_AUTO_COMPACT_WINDOW"`)
	out, err := PiDriver{Dir: t.TempDir()}.command(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "pi-window=" {
		t.Fatalf("pi saw %q, want no harnez window", got)
	}
}

func responseOf(r *TurnResult) string {
	if r == nil {
		return ""
	}
	return r.Response
}
