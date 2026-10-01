package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestSanitizeSelftestID(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", "default"},
		{"abc-123_XYZ", "abc-123_XYZ"},
		{"agent:claude:session/123", "agent_claude_session_123"},
		{"a@b#c$d%e", "a_b_c_d_e"},
	}
	for _, tc := range cases {
		got := sanitizeSelftestID(tc.input)
		if got != tc.want {
			t.Errorf("sanitizeSelftestID(%q) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

func TestAgentSelftestCommandSurface(t *testing.T) {
	c := newAgentCmd()
	child, _, err := c.Find([]string{"selftest"})
	if err != nil {
		t.Fatalf("selftest subcommand not found: %v", err)
	}
	if child == nil || child.Name() != "selftest" {
		t.Fatal("selftest subcommand missing from agent command")
	}
}

func TestAgentSelftestHelpNonTTY(t *testing.T) {
	cmd := newAgentSelftestCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help failed: %v", err)
	}
	res := out.String()
	if !strings.Contains(res, "Run with --step hello to begin") {
		t.Errorf("expected non-TTY help to instruct to run --step hello, got:\n%s", res)
	}
	// Non-TTY should NOT reveal hidden step names in help
	if strings.Contains(res, "confirm-running") || strings.Contains(res, "verify") {
		t.Errorf("non-TTY help leaked step names:\n%s", res)
	}
}

func TestAgentSelftestHelpTTY(t *testing.T) {
	orig := isOutputTerminalFn
	defer func() { isOutputTerminalFn = orig }()
	isOutputTerminalFn = func(*cobra.Command) bool { return true }

	cmd := newAgentSelftestCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help failed: %v", err)
	}
	res := out.String()
	if !strings.Contains(res, "Steps:") || !strings.Contains(res, "confirm-running") || !strings.Contains(res, "verify") {
		t.Errorf("expected TTY help to list steps, got:\n%s", res)
	}
}

func TestAgentSelftestNoArgsNonTTY(t *testing.T) {
	cmd := newAgentSelftestCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("no-args execution failed: %v", err)
	}
	res := out.String()
	if !strings.Contains(res, "Run with --step hello to begin") {
		t.Errorf("expected guidance on no-args in non-TTY, got: %s", res)
	}
}

func TestAgentSelftestFullSequenceSuccess(t *testing.T) {
	tempDir := t.TempDir()
	sessionID := "test-session-123"

	// 1. Run --step hello
	{
		cmd := newAgentSelftestCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "hello"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("hello failed: %v", err)
		}
		if !strings.Contains(out.String(), "Agent background self-test initialized") {
			t.Errorf("unexpected hello output: %s", out.String())
		}
	}

	// 2. Launch background task in a goroutine
	bgDone := make(chan error, 1)
	go func() {
		cmd := newAgentSelftestCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--duration", "1s", "--step", "background"})
		bgDone <- cmd.Execute()
	}()

	// Wait for background to record started state
	statePath := filepath.Join(tempDir, "harnez-selftest-"+sessionID+".json")
	var started bool
	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		st, err := loadSelftestState(statePath)
		if err == nil && st.BackgroundStartedAt != nil {
			started = true
			break
		}
	}
	if !started {
		t.Fatal("background task did not record started state in time")
	}

	// 3. Run --step confirm while background is running
	{
		cmd := newAgentSelftestCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "confirm"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("confirm failed: %v", err)
		}
		if !strings.Contains(out.String(), "Confirmed background task is currently active") {
			t.Errorf("unexpected confirm output: %s", out.String())
		}
	}

	// 4. Run --step confirm-running with separate arg
	{
		cmd := newAgentSelftestCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "confirm-running", "200ms"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("confirm-running failed: %v", err)
		}
		if !strings.Contains(out.String(), "Recorded background running confirmation (duration: 200ms)") {
			t.Errorf("unexpected confirm-running output: %s", out.String())
		}
	}

	// Wait for background task goroutine to finish
	select {
	case err := <-bgDone:
		if err != nil {
			t.Fatalf("background task failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for background task")
	}

	// 5. Run --step verify
	{
		cmd := newAgentSelftestCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "verify"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("verify failed: %v", err)
		}
		if !strings.Contains(out.String(), "PASS:") {
			t.Errorf("unexpected verify output: %s", out.String())
		}
	}
}

func TestAgentSelftestErrorConditions(t *testing.T) {
	tempDir := t.TempDir()
	sessionID := "test-err-session"

	// Confirm before hello
	{
		cmd := newAgentSelftestCmd()
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "confirm"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error running confirm before hello")
		}
	}

	// Hello first
	{
		cmd := newAgentSelftestCmd()
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "hello"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("hello failed: %v", err)
		}
	}

	// Confirm before background started
	{
		cmd := newAgentSelftestCmd()
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "confirm"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "background task was not started") {
			t.Fatalf("expected error for confirm before background started, got: %v", err)
		}
	}

	// Confirm-running before confirm
	{
		cmd := newAgentSelftestCmd()
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "confirm-running"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "confirm' first") {
			t.Fatalf("expected error for confirm-running before confirm, got: %v", err)
		}
	}

	// Verify before completion
	{
		cmd := newAgentSelftestCmd()
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "verify"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "validation failed") {
			t.Fatalf("expected verify validation error, got: %v", err)
		}
	}
}

func TestAgentSelftestConfirmAfterBackgroundCompletedFails(t *testing.T) {
	tempDir := t.TempDir()
	sessionID := "test-sequential-err"

	// 1. Hello
	cmd := newAgentSelftestCmd()
	cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "hello"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hello failed: %v", err)
	}

	// 2. Run background synchronously to completion before confirm
	cmd = newAgentSelftestCmd()
	cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--duration", "50ms", "--step", "background"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("background failed: %v", err)
	}

	// 3. Try confirm after background already finished -> should fail
	cmd = newAgentSelftestCmd()
	cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "confirm"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "already finished") {
		t.Fatalf("expected error for confirm after background finished, got: %v", err)
	}
}

func TestAgentSelftestVerifyBadOrderings(t *testing.T) {
	tempDir := t.TempDir()
	sessionID := "test-bad-orderings"
	statePath := filepath.Join(tempDir, "harnez-selftest-"+sessionID+".json")

	t0 := time.Now()
	t1 := t0.Add(1 * time.Second)
	t2 := t0.Add(2 * time.Second)
	t3 := t0.Add(3 * time.Second)
	t4 := t0.Add(4 * time.Second)

	// State where confirm ran AFTER background finished
	st := &selftestState{
		SessionID:            sessionID,
		HelloAt:              &t0,
		BackgroundStartedAt:  &t1,
		BackgroundFinishedAt: &t2,
		ConfirmAt:            &t3,
		ConfirmRunningAt:     &t4,
	}
	if err := saveSelftestState(statePath, st); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	cmd := newAgentSelftestCmd()
	cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "verify"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "confirm ran after --step background had already completed") {
		t.Fatalf("expected bad ordering error, got: %v", err)
	}

	// State where confirm-running ran BEFORE confirm
	st.ConfirmAt = &t4
	st.ConfirmRunningAt = &t3
	st.BackgroundFinishedAt = &t4
	_ = saveSelftestState(statePath, st)

	cmd = newAgentSelftestCmd()
	cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "verify"})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "confirm-running ran before --step confirm") {
		t.Fatalf("expected confirm-running ordering error, got: %v", err)
	}
}

func TestAgentSelftestSessionIsolation(t *testing.T) {
	tempDir := t.TempDir()

	cmd1 := newAgentSelftestCmd()
	cmd1.SetArgs([]string{"--session", "session-A", "--state-dir", tempDir, "--step", "hello"})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("hello A failed: %v", err)
	}

	// session-B should not see session-A's state
	cmd2 := newAgentSelftestCmd()
	cmd2.SetArgs([]string{"--session", "session-B", "--state-dir", tempDir, "--step", "confirm"})
	if err := cmd2.Execute(); err == nil {
		t.Fatal("expected session-B confirm to fail without hello")
	}
}
