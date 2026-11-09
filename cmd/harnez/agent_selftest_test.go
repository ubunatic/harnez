package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/resolve"
)

func TestAgentSelftestGeneratedSessionAcrossShells(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX /bin/sh")
	}
	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "state dir's $; *")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(tempDir, "harnez"), ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	env := append(os.Environ(), "PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"), "HARNEZ_AGENT_ROLE=")
	for _, name := range resolve.SessionEnvVars {
		env = append(env, name+"=")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	command := func(line string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", line)
		cmd.Env = env
		return cmd
	}
	run := func(line string) string {
		t.Helper()
		output, err := command(line).CombinedOutput()
		if err != nil {
			t.Fatalf("command %q: %v\n%s", line, err, output)
		}
		return string(output)
	}
	oneCommand := func(output string) string {
		t.Helper()
		commands := extractSelftestCommands(output)
		if len(commands) != 1 {
			t.Fatalf("expected one printed command: %s", output)
		}
		return commands[0]
	}
	hello := run("harnez agent selftest --step hello --state-dir " + shellQuote(stateDir))
	files, err := filepath.Glob(filepath.Join(stateDir, "harnez-selftest-*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("hello state files = %v, error = %v", files, err)
	}
	state, err := loadSelftestState(files[0])
	if err != nil || state.SessionID == "" {
		t.Fatalf("hello state = %+v, error = %v", state, err)
	}
	commands := extractSelftestCommands(hello)
	if len(commands) != 2 {
		t.Fatalf("expected two hello commands: %s", hello)
	}
	for _, line := range commands {
		if !strings.Contains(line, "--session "+shellQuote(state.SessionID)) {
			t.Fatalf("command does not carry generated session %q: %s", state.SessionID, line)
		}
	}
	background := command(commands[0])
	var backgroundOutput bytes.Buffer
	background.Stdout = &backgroundOutput
	background.Stderr = &backgroundOutput
	if err := background.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- background.Wait() }()
	t.Cleanup(func() { cancel(); <-done })
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, err = loadSelftestState(files[0])
		if err == nil && state.BackgroundStartedAt != nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("background did not start: %v", ctx.Err())
		case <-ticker.C:
		}
	}
	confirm := run(commands[1])
	confirmRunning := run(oneCommand(confirm))
	verify := oneCommand(confirmRunning)
	select {
	case err := <-done:
		done <- err // Leave the result for cleanup.
		if err != nil {
			t.Fatalf("background: %v\n%s", err, backgroundOutput.String())
		}
	case <-ctx.Done():
		t.Fatalf("background did not finish: %v", ctx.Err())
	}
	if got := oneCommand(backgroundOutput.String()); got != verify {
		t.Fatalf("verify commands differ: %q and %q", got, verify)
	}
	run(verify)
	state, err = loadSelftestState(files[0])
	if err != nil || state.SessionID == "" || state.ConfirmAt == nil || state.ConfirmRunningAt == nil || state.BackgroundFinishedAt == nil {
		t.Fatalf("incomplete final state = %+v, error = %v", state, err)
	}
}

func TestSelftestShellQuote(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX /bin/sh")
	}
	for _, input := range []string{"", "plain", "with spaces", "apostrophe' and $HOME; *"} {
		cmd := exec.Command("/bin/sh", "-c", "printf '%s' "+shellQuote(input))
		got, err := cmd.Output()
		if err != nil {
			t.Fatalf("shell quote failed for %q: %v", input, err)
		}
		if string(got) != input {
			t.Errorf("shell quote roundtrip = %q, want %q", got, input)
		}
	}
}

func TestAgentSelftestPrintedCommandsAcrossShells(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX /bin/sh")
	}
	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "state dir's $; *")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(tempDir, "harnez")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Join("..", "..", "cmd", "harnez")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	// Derive the unusual session value from shell input, so the quoting test
	// exercises the real shell's parsing and argument passing.
	sessionID := "session with ' quote $HOME; *"
	base := "harnez agent selftest --session " + shellQuote(sessionID) + " --state-dir " + shellQuote(stateDir)
	call := func(command string) string {
		return "HTO=0 " + command
	}
	sh := func(command string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		process := exec.CommandContext(ctx, "/bin/sh", "-c", command)
		process.Env = append(os.Environ(), "PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
		return process.CombinedOutput()
	}

	hello, err := sh(call(base + " --step hello"))
	if err != nil {
		t.Fatalf("hello: %v\n%s", err, hello)
	}
	commands := extractSelftestCommands(string(hello))
	if len(commands) != 2 {
		t.Fatalf("hello did not print copyable commands: %s", hello)
	}
	for _, command := range commands {
		if !strings.Contains(command, "--session "+shellQuote(sessionID)) || !strings.Contains(command, "--state-dir "+shellQuote(stateDir)) {
			t.Errorf("printed command lacks safely quoted session/state dir: %s", command)
		}
	}
	backgroundOutput, err := sh(call(commands[0] + " --duration 1ms"))
	if err != nil {
		t.Fatalf("printed background command: %v\n%s", err, backgroundOutput)
	}
	backgroundNext := extractSelftestCommands(string(backgroundOutput))
	if len(backgroundNext) != 1 || !strings.Contains(backgroundNext[0], "--session "+shellQuote(sessionID)) || !strings.Contains(backgroundNext[0], "--state-dir "+shellQuote(stateDir)) {
		t.Fatalf("background did not print a session/state-dir-aware verify command: %s", backgroundOutput)
	}
	// The printed confirm command must resolve the state created in the previous
	// shell. This path deliberately reaches the expected completed-background
	// diagnostic, proving both unusual arguments survived shell parsing.
	confirmOutput, err := sh(call(commands[1]))
	if err == nil || !strings.Contains(string(confirmOutput), "already finished") {
		t.Fatalf("printed confirm command did not find shared state: err=%v output=%s", err, confirmOutput)
	}
	missingSessionCommand := "harnez agent selftest --step verify --session " + shellQuote(sessionID+"-missing") + " --state-dir " + shellQuote(stateDir)
	verifyOutput, err := sh(call(missingSessionCommand))
	if err == nil || !strings.Contains(string(verifyOutput), "run "+selftestCommand("hello", sessionID+"-missing", stateDir)+" first") {
		t.Fatalf("missing-state recovery omitted quoted session/state directory: err=%v output=%s", err, verifyOutput)
	}
}

func extractSelftestCommands(output string) []string {
	var commands []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "HTO=0 harnez agent selftest ") || strings.HasPrefix(line, "harnez agent selftest ") {
			commands = append(commands, line)
		}
	}
	return commands
}

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
	skipShort(t, "runs a 1s background selftest step")
	oldWait := selftestBackgroundWait
	started, finish := make(chan struct{}, 1), make(chan struct{})
	var finishOnce sync.Once
	release := func() { finishOnce.Do(func() { close(finish) }) }
	selftestBackgroundWait = func(time.Duration) { started <- struct{}{}; <-finish }
	t.Cleanup(func() { release(); selftestBackgroundWait = oldWait })
	t.Setenv("HTO", "0")
	tempDir := t.TempDir()
	sessionID := "test session ' $HOME; *"
	checkNext := func(output, step string) {
		t.Helper()
		commands := extractSelftestCommands(output)
		want := selftestCommand(step, sessionID, tempDir)
		if len(commands) != 1 || commands[0] != want {
			t.Errorf("next command = %q, want %q; output: %s", commands, want, output)
		}
	}

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
	type backgroundResult struct {
		output string
		err    error
	}
	bgDone := make(chan backgroundResult, 1)
	go func() {
		cmd := newAgentSelftestCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--duration", "1s", "--step", "background"})
		err := cmd.Execute()
		bgDone <- backgroundResult{out.String(), err}
	}()

	// Wait for background to record started state
	select {
	case <-started:
	case <-time.After(5 * time.Second * raceFactor):
		t.Fatal("background task did not reach its start event")
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
		checkNext(out.String(), "confirm-running")
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
		checkNext(out.String(), "verify")
	}
	release()

	// Wait for background task goroutine to finish
	select {
	case result := <-bgDone:
		if result.err != nil {
			t.Fatalf("background task failed: %v", result.err)
		}
		checkNext(result.output, "verify")
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

func TestAgentSelftestVerifyMissingHTOFails(t *testing.T) {
	oldWait := selftestBackgroundWait
	started, finish := make(chan struct{}, 1), make(chan struct{})
	var finishOnce sync.Once
	release := func() { finishOnce.Do(func() { close(finish) }) }
	selftestBackgroundWait = func(time.Duration) { started <- struct{}{}; <-finish }
	t.Cleanup(func() { release(); selftestBackgroundWait = oldWait })
	// Ensure HTO is unset
	t.Setenv("HTO", "")
	_ = os.Unsetenv("HTO")

	tempDir := t.TempDir()
	sessionID := "test-missing-hto"
	statePath := filepath.Join(tempDir, "harnez-selftest-"+sessionID+".json")

	// 1. Hello
	cmd := newAgentSelftestCmd()
	cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "hello"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hello failed: %v", err)
	}

	// 2. Background launched without HTO
	bgDone := make(chan error, 1)
	go func() {
		cmd := newAgentSelftestCmd()
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--duration", "200ms", "--step", "background"})
		bgDone <- cmd.Execute()
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second * raceFactor):
		t.Fatal("background task did not reach its start event")
	}

	// 3. Confirm
	cmd = newAgentSelftestCmd()
	cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "confirm"})
	_ = cmd.Execute()

	// 4. Confirm-running
	cmd = newAgentSelftestCmd()
	cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "confirm-running"})
	_ = cmd.Execute()
	release()

	<-bgDone

	// 5. Verify -> must fail because HTO was not set
	cmd = newAgentSelftestCmd()
	cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "verify"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "without setting HTO") {
		t.Fatalf("expected verify failure due to missing HTO, got: %v", err)
	}

	// 6. Test with explicit empty HTO string
	st, _ := loadSelftestState(statePath)
	st.BackgroundHTO = ""
	_ = saveSelftestState(statePath, st)

	cmd = newAgentSelftestCmd()
	cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "verify"})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "without setting HTO") {
		t.Fatalf("expected verify failure due to empty HTO string, got: %v", err)
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
		want := "run " + selftestCommand("confirm", sessionID, tempDir) + " first"
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error containing %q for confirm-running before confirm, got: %v", want, err)
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

func TestAgentSelftestConfirmRunningArgs(t *testing.T) {
	tempDir := t.TempDir()

	// Test case 1: no extra duration argument
	{
		sessionID := "test-no-arg"
		cmd := newAgentSelftestCmd()
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "hello"})
		_ = cmd.Execute()

		st, _ := loadSelftestState(selftestStateFilePath(tempDir, sessionID))
		now := time.Now()
		st.BackgroundStartedAt = &now
		_ = saveSelftestState(selftestStateFilePath(tempDir, sessionID), st)

		cmd = newAgentSelftestCmd()
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "confirm"})
		_ = cmd.Execute()

		cmd = newAgentSelftestCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "confirm-running"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("confirm-running with no arg failed: %v", err)
		}
		if !strings.Contains(out.String(), "Recorded background running confirmation.") {
			t.Errorf("unexpected output: %s", out.String())
		}
		st, _ = loadSelftestState(selftestStateFilePath(tempDir, sessionID))
		if st.ConfirmDuration != "" {
			t.Errorf("expected empty ConfirmDuration, got: %q", st.ConfirmDuration)
		}
	}

	// Test case 2: duration as separate argument
	{
		sessionID := "test-sep-arg"
		cmd := newAgentSelftestCmd()
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "hello"})
		_ = cmd.Execute()

		st, _ := loadSelftestState(selftestStateFilePath(tempDir, sessionID))
		now := time.Now()
		st.BackgroundStartedAt = &now
		_ = saveSelftestState(selftestStateFilePath(tempDir, sessionID), st)

		cmd = newAgentSelftestCmd()
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "confirm"})
		_ = cmd.Execute()

		cmd = newAgentSelftestCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--session", sessionID, "--state-dir", tempDir, "--step", "confirm-running", "150ms"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("confirm-running with separate arg failed: %v", err)
		}
		if !strings.Contains(out.String(), "Recorded background running confirmation (duration: 150ms)") {
			t.Errorf("unexpected output: %s", out.String())
		}
		st, _ = loadSelftestState(selftestStateFilePath(tempDir, sessionID))
		if st.ConfirmDuration != "150ms" {
			t.Errorf("expected ConfirmDuration '150ms', got: %q", st.ConfirmDuration)
		}
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
		BackgroundHTO:        "0",
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
