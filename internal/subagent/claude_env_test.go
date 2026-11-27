package subagent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

// specWindowOffset is claude_compact_window_offset_tokens from the embedded spec.
func specWindowOffset(t *testing.T) int {
	t.Helper()
	spec, err := agentSpecOnce()
	if err != nil {
		t.Fatal(err)
	}
	return spec.ClaudeCompactWindowOffset
}

// The window is the threshold plus the spec offset, so Claude (which compacts
// about 33k below its window) compacts near the threshold, not 33k under it.
func TestClaudeWindowAddsSpecOffset(t *testing.T) {
	off := specWindowOffset(t)
	if off <= 0 || off >= 33000 {
		t.Fatalf("spec offset %d: want 0 < offset < 33000 so Claude compacts below the threshold", off)
	}
	if got, err := ClaudeCompactWindow(100000); err != nil || got != 100000+off {
		t.Fatalf("ClaudeCompactWindow(100000) = %d, %v; want %d", got, err, 100000+off)
	}
}

func TestClaudeChildGetsWindowOnStartAndResume(t *testing.T) {
	claudeEnvHome(t, "agent:\n  compact_threshold_tokens: 150000\n")
	fakeClaudeWindow(t)
	want := fmt.Sprintf("window=%d", 150000+specWindowOffset(t))
	d := ClaudeDriver{Dir: t.TempDir()}
	r, err := d.Run(context.Background(), RunOptions{Model: claudeEnvModel, Prompt: "p"})
	if err != nil || !strings.Contains(r.Response, want) {
		t.Fatalf("Run: response %q, err %v; want %s", responseOf(r), err, want)
	}
	r, err = d.Resume(context.Background(), "s1", "p", claudeEnvModel)
	if err != nil || !strings.Contains(r.Response, want) {
		t.Fatalf("Resume: response %q, err %v; want %s", responseOf(r), err, want)
	}
}

func TestClaudeStreamChildGetsWindowOnStartAndResume(t *testing.T) {
	claudeEnvHome(t, "agent:\n  compact_threshold_tokens: 150000\n")
	fakeClaudeWindow(t)
	want := fmt.Sprintf("window=%d", 150000+specWindowOffset(t))
	d := ClaudeStreamDriver{ClaudeDriver{Dir: t.TempDir()}}
	noEvents := func(Event) {}
	r, err := d.RunStream(context.Background(), RunOptions{Model: claudeEnvModel, Prompt: "p"}, noEvents)
	if err != nil || !strings.Contains(responseOf(r), want) {
		t.Fatalf("RunStream: response %q, err %v; want %s", responseOf(r), err, want)
	}
	r, err = d.ResumeStream(context.Background(), "s1", "p", claudeEnvModel, noEvents)
	if err != nil || !strings.Contains(responseOf(r), want) {
		t.Fatalf("ResumeStream: response %q, err %v; want %s", responseOf(r), err, want)
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
	if want := fmt.Sprintf("above the window %d harnez sets", 150000+specWindowOffset(t)); !strings.Contains(warn.String(), want) {
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
	window := 150000 + specWindowOffset(t)
	for _, value := range []string{"60000", "150000", strconv.Itoa(window)} {
		var warn bytes.Buffer
		claudeChildEnv(claudeEnvModel, []string{claudeWindowVar + "=" + value}, &warn)
		if warn.Len() != 0 {
			t.Fatalf("caller %s at or below window %d warned: %q", value, window, warn.String())
		}
	}
	var warn bytes.Buffer
	claudeChildEnv(claudeEnvModel, []string{claudeWindowVar + "=" + strconv.Itoa(window+1)}, &warn)
	if warn.Len() == 0 {
		t.Fatalf("caller %d one above window %d did not warn", window+1, window)
	}
}

func TestClaudeWindowUnderHundredKStillSet(t *testing.T) {
	claudeEnvHome(t, "agent:\n  compact_threshold_tokens: 80000\n")
	fakeClaudeWindow(t)
	r, err := ClaudeDriver{Dir: t.TempDir()}.Run(context.Background(), RunOptions{Model: claudeEnvModel, Prompt: "p"})
	if want := fmt.Sprintf("window=%d", 80000+specWindowOffset(t)); err != nil || !strings.Contains(responseOf(r), want) {
		t.Fatalf("response %q, err %v; want %s", responseOf(r), err, want)
	}
}

func TestClaudeChildEnvUnchangedWhenThresholdDisabled(t *testing.T) {
	claudeEnvHome(t, "agent:\n  compact_thresholds:\n    claude:sonnet:med: 0\n")
	var warn bytes.Buffer
	env := []string{"PATH=/bin"}
	if got := claudeChildEnv(claudeEnvModel, env, &warn); len(got) != 1 || got[0] != "PATH=/bin" {
		t.Fatalf("env %v; want unchanged when the threshold is disabled", got)
	}
	if warn.Len() != 0 {
		t.Fatalf("disabled threshold warned: %q; want silence for an intentional 0", warn.String())
	}
}

func TestClaudeChildEnvWarnsOnRealResolutionError(t *testing.T) {
	claudeEnvHome(t, "agent:\n  compact_threshold_tokens: [oops\n")
	var warn bytes.Buffer
	env := []string{"PATH=/bin"}
	if got := claudeChildEnv(claudeEnvModel, env, &warn); len(got) != 1 || got[0] != "PATH=/bin" {
		t.Fatalf("env %v; want unchanged on a resolution error", got)
	}
	if !strings.Contains(warn.String(), claudeWindowVar) || !strings.Contains(warn.String(), "parse agent compact config") {
		t.Fatalf("warning %q; want the resolution error named", warn.String())
	}
}

func TestClaudeMalformedCallerWindowKeptWithWarning(t *testing.T) {
	claudeEnvHome(t, "agent:\n  compact_threshold_tokens: 150000\n")
	var warn bytes.Buffer
	env := []string{claudeWindowVar + "=abc"}
	if got := claudeChildEnv(claudeEnvModel, env, &warn); len(got) != 1 || got[0] != claudeWindowVar+"=abc" {
		t.Fatalf("env %v; want malformed caller value kept as-is", got)
	}
	if !strings.Contains(warn.String(), "not a number") {
		t.Fatalf("warning %q; want a malformed-value warning", warn.String())
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
