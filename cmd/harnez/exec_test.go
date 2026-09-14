package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
)

func strconvQuote(s string) string { return strconv.Quote(s) }

// shlexSplitForTest tokenizes rewritten exactly as a real outer 'bash -c
// <rewritten>' would (the same re-execution TestRunExecHook_* is guarding
// against), by shadowing the 'harnez' command with a shell function that
// just reports its argv instead of running the real binary — no real
// subprocess side effects, just bash's own word-splitting/quoting rules.
func shlexSplitForTest(rewritten string) ([]string, error) {
	const sep = "\x1f"
	script := "harnez() { for a in \"$@\"; do printf '%s" + sep + "' \"$a\"; done; }\n" +
		"⚙() { for a in \"$@\"; do printf '%s" + sep + "' \"$a\"; done; }\n" + rewritten
	out, err := exec.Command("bash", "-c", script).Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSuffix(string(out), sep), sep), nil
}

// testExecOptions returns execOptions isolated from the caller's real
// environment/DB: a throwaway state dir and DB path, explicit ticket, and
// a tight-but-real Getenv so no ambient CLAUDE_CODE_SESSION_ID etc. leaks
// in from the actual test-runner environment.
func testExecOptions(t *testing.T) execOptions {
	t.Helper()
	dir := t.TempDir()
	return execOptions{
		Tool:     "test-tool",
		Ticket:   "harnez/118-test-ticket",
		Getenv:   func(string) string { return "" },
		StateDir: filepath.Join(dir, "state"),
		DBPath:   filepath.Join(dir, "tool_catalog.sqlite"),
	}
}

func TestRunExecWrapper_ExitCodeTable(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"success", []string{"true"}, 0},
		{"non-zero", []string{"sh", "-c", "exit 7"}, 7},
		{"signal-terminated", []string{"sh", "-c", "kill -TERM $$"}, 128 + 15},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code, err := runExecWrapper(tc.args, testExecOptions(t), strings.NewReader(""), &out, &errOut)
			if err != nil {
				t.Fatalf("runExecWrapper() error = %v", err)
			}
			if code != tc.want {
				t.Errorf("exit code = %d, want %d", code, tc.want)
			}
		})
	}
}

func TestRunExecWrapper_SpawnErrorForMissingCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	_, err := runExecWrapper([]string{"harnez-118-definitely-not-a-real-command"}, testExecOptions(t),
		strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatal("expected an error for a command that cannot be spawned")
	}
}

func TestRunExecWrapper_ProxiesStdoutStderr(t *testing.T) {
	var out, errOut bytes.Buffer
	code, err := runExecWrapper(
		[]string{"sh", "-c", "echo out-line; echo err-line 1>&2"},
		testExecOptions(t), strings.NewReader(""), &out, &errOut,
	)
	if err != nil {
		t.Fatalf("runExecWrapper() error = %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(out.String()) != "out-line" {
		t.Errorf("stdout = %q, want %q", out.String(), "out-line")
	}
	if strings.TrimSpace(errOut.String()) != "err-line" {
		t.Errorf("stderr = %q, want %q", errOut.String(), "err-line")
	}
}

func TestRunExecWrapper_RecordsDurationAndRawBytes(t *testing.T) {
	opts := testExecOptions(t)
	opts.InsertTimeout = 2 * time.Second // generous: real sqlite insert, not a slow-writer test

	var out, errOut bytes.Buffer
	const sleepSecs = 0.2
	code, err := runExecWrapper(
		[]string{"sh", "-c", "sleep " + "0.2" + "; printf '%s' 'twelve bytes!'"},
		opts, strings.NewReader(""), &out, &errOut,
	)
	if err != nil {
		t.Fatalf("runExecWrapper() error = %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := out.String(); got != "twelve bytes!" {
		t.Fatalf("stdout = %q", got)
	}

	db, err := telemetry.Open(opts.DBPath)
	if err != nil {
		t.Fatalf("Open telemetry db: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(telemetry.Filter{CallType: "shell"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 shell row, got %d", len(rows))
	}
	row := rows[0]

	if row.DurationMs < int64(sleepSecs*1000)-50 {
		t.Errorf("duration_ms = %d, want roughly >= %d (sleep 0.2s)", row.DurationMs, int64(sleepSecs*1000))
	}
	if row.RawBytes != int64(len("twelve bytes!")) {
		t.Errorf("raw_bytes = %d, want %d", row.RawBytes, len("twelve bytes!"))
	}
	if row.DistilledBytes != nil {
		t.Errorf("distilled_bytes = %v, want nil/NULL when distillation not requested", *row.DistilledBytes)
	}
	if row.Score == nil || *row.Score != 5 {
		t.Errorf("score = %v, want 5 (clean success)", row.Score)
	}
	if row.ExitCode == nil || *row.ExitCode != 0 {
		t.Errorf("exit_code = %v, want 0", row.ExitCode)
	}
	if row.CallType != "shell" {
		t.Errorf("call_type = %q, want shell", row.CallType)
	}
	if row.ToolName != "test-tool" {
		t.Errorf("tool_name = %q, want test-tool", row.ToolName)
	}
}

func TestRunExecWrapper_RecordsDistilledBytes(t *testing.T) {
	opts := testExecOptions(t)
	opts.Distill = "gotest"
	opts.InsertTimeout = 2 * time.Second

	var out, errOut bytes.Buffer
	testOutput := "=== RUN   TestFoo\n--- PASS: TestFoo (0.01s)\n=== RUN   TestBar\n--- PASS: TestBar (0.01s)\nPASS\nok  example.com/foo 0.02s\n"
	code, err := runExecWrapper(
		[]string{"sh", "-c", "printf '%b' " + strconvQuote(testOutput)},
		opts, strings.NewReader(""), &out, &errOut,
	)
	if err != nil {
		t.Fatalf("runExecWrapper() error = %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	db, err := telemetry.Open(opts.DBPath)
	if err != nil {
		t.Fatalf("Open telemetry db: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(telemetry.Filter{CallType: "shell"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 shell row, got %d", len(rows))
	}
	row := rows[0]

	if row.RawBytes != int64(len(testOutput)) {
		t.Errorf("raw_bytes = %d, want %d", row.RawBytes, len(testOutput))
	}
	if row.DistilledBytes == nil {
		t.Fatalf("distilled_bytes is nil, want non-nil")
	}
	if *row.DistilledBytes >= row.RawBytes {
		t.Errorf("distilled_bytes = %d, want < raw_bytes (%d)", *row.DistilledBytes, row.RawBytes)
	}
	if row.Score == nil || *row.Score != 5 {
		t.Errorf("score = %v, want 5", row.Score)
	}
}

func TestRunExecWrapper_RecordsSyntheticQualityScores(t *testing.T) {
	cases := []struct {
		name      string
		script    string
		wantScore int
	}{
		{"panic", "echo 'panic: runtime error: index out of range'; exit 2", 1},
		{"syntax error", "echo './main.go:10:2: syntax error: unexpected semicolon'; exit 2", 2},
		{"test failure", "echo '--- FAIL: TestFoo (0.00s)'; exit 1", 3},
		{"clean success", "echo 'all good'; exit 0", 5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := testExecOptions(t)
			opts.InsertTimeout = 2 * time.Second

			var out, errOut bytes.Buffer
			_, err := runExecWrapper([]string{"sh", "-c", tc.script}, opts, strings.NewReader(""), &out, &errOut)
			if err != nil {
				t.Fatalf("runExecWrapper: %v", err)
			}

			db, err := telemetry.Open(opts.DBPath)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			defer db.Close()

			rows, err := db.Query(telemetry.Filter{CallType: "shell"})
			if err != nil || len(rows) != 1 {
				t.Fatalf("Query rows = %v, err = %v", rows, err)
			}
			if rows[0].Score == nil || *rows[0].Score != tc.wantScore {
				t.Errorf("row score = %v, want %d", rows[0].Score, tc.wantScore)
			}
		})
	}
}

func TestRunExecWrapper_TelemetryWriteNeverBlocksCommand(t *testing.T) {
	opts := testExecOptions(t)
	opts.InsertTimeout = 30 * time.Millisecond
	slowStarted := make(chan struct{}, 1)
	opts.Insert = func(dbPath string, call telemetry.ToolCall) error {
		slowStarted <- struct{}{}
		time.Sleep(2 * time.Second) // deliberately far past InsertTimeout and past the test's own deadline
		return errors.New("should never be observed by the wrapper")
	}

	var out, errOut bytes.Buffer
	start := time.Now()
	code, err := runExecWrapper([]string{"true"}, opts, strings.NewReader(""), &out, &errOut)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("runExecWrapper() error = %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// Bound generously above InsertTimeout to absorb scheduler jitter, but
	// nowhere near the injected writer's 2s sleep: this is the assertion
	// that a hung/slow telemetry write cannot delay the wrapped command's
	// own exit beyond the configured bound.
	if elapsed > 500*time.Millisecond {
		t.Errorf("runExecWrapper() took %v, want well under the 2s slow-writer delay (bounded by InsertTimeout=%v)", elapsed, opts.InsertTimeout)
	}

	select {
	case <-slowStarted:
		// good: the slow writer was actually invoked, so this test isn't
		// vacuously passing because Insert was never called.
	case <-time.After(time.Second):
		t.Fatal("injected slow Insert was never invoked")
	}
}

// TestRunExecWrapper_ExpectFailureRecordsExpectedCallTypeAndTrueExitCode
// covers issue 226 direction 2 end-to-end: a command whose text carries a
// leading HARNEZ_EXPECT_FAILURE=1 assignment (the shape the PreToolUse
// hook's rewritten `bash -c '<original command>'` produces) must still
// return/record the command's real exit code — never faked or swallowed —
// while the telemetry row's call_type is classified as
// telemetry.ExpectedFailureCallType instead of "shell".
func TestRunExecWrapper_ExpectFailureRecordsExpectedCallTypeAndTrueExitCode(t *testing.T) {
	opts := testExecOptions(t)
	opts.InsertTimeout = 2 * time.Second

	var out, errOut bytes.Buffer
	code, err := runExecWrapper(
		[]string{"bash", "-c", "HARNEZ_EXPECT_FAILURE=1 sh -c 'exit 7'"},
		opts, strings.NewReader(""), &out, &errOut,
	)
	if err != nil {
		t.Fatalf("runExecWrapper() error = %v", err)
	}
	if code != 7 {
		t.Fatalf("exit code = %d, want 7 (true exit code, never faked)", code)
	}

	db, err := telemetry.Open(opts.DBPath)
	if err != nil {
		t.Fatalf("Open telemetry db: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(telemetry.Filter{CallType: telemetry.ExpectedFailureCallType})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 shell-expected row, got %d", len(rows))
	}
	if rows[0].ExitCode == nil || *rows[0].ExitCode != 7 {
		t.Errorf("exit_code = %v, want 7", rows[0].ExitCode)
	}

	// The plain "shell" call_type must not also carry this row.
	plain, err := db.Query(telemetry.Filter{CallType: "shell"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(plain) != 0 {
		t.Errorf("expected 0 plain shell rows, got %d", len(plain))
	}
}

// TestRunExecWrapper_UnmarkedFailureStillRecordsShell is the regression
// guard alongside the test above: an ordinary failing command with no
// HARNEZ_EXPECT_FAILURE marker keeps being recorded as call_type=shell,
// exactly as before issue 226.
func TestRunExecWrapper_UnmarkedFailureStillRecordsShell(t *testing.T) {
	opts := testExecOptions(t)
	opts.InsertTimeout = 2 * time.Second

	var out, errOut bytes.Buffer
	code, err := runExecWrapper(
		[]string{"bash", "-c", "sh -c 'exit 7'"},
		opts, strings.NewReader(""), &out, &errOut,
	)
	if err != nil {
		t.Fatalf("runExecWrapper() error = %v", err)
	}
	if code != 7 {
		t.Fatalf("exit code = %d, want 7", code)
	}

	db, err := telemetry.Open(opts.DBPath)
	if err != nil {
		t.Fatalf("Open telemetry db: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(telemetry.Filter{CallType: "shell"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 shell row, got %d", len(rows))
	}
	if rows[0].ExitCode == nil || *rows[0].ExitCode != 7 {
		t.Errorf("exit_code = %v, want 7", rows[0].ExitCode)
	}
}

// TestRunExecWrapper_ExpectFailureViaProcessEnv covers the direct/manual
// invocation path: opts.Getenv(HARNEZ_EXPECT_FAILURE) set truthy, as would
// happen for a scripted `HARNEZ_EXPECT_FAILURE=1 harnez exec --tool ... --`
// call rather than one routed through the PreToolUse hook's bash -c wrap.
func TestRunExecWrapper_ExpectFailureViaProcessEnv(t *testing.T) {
	opts := testExecOptions(t)
	opts.InsertTimeout = 2 * time.Second
	opts.Getenv = func(k string) string {
		if k == expectFailureEnv {
			return "1"
		}
		return ""
	}

	var out, errOut bytes.Buffer
	code, err := runExecWrapper([]string{"sh", "-c", "exit 3"}, opts, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatalf("runExecWrapper() error = %v", err)
	}
	if code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}

	db, err := telemetry.Open(opts.DBPath)
	if err != nil {
		t.Fatalf("Open telemetry db: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(telemetry.Filter{CallType: telemetry.ExpectedFailureCallType})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 shell-expected row, got %d", len(rows))
	}
}

func TestDetectExpectFailure(t *testing.T) {
	cases := []struct {
		name string
		opts execOptions
		args []string
		want bool
	}{
		{"no marker", execOptions{Getenv: func(string) string { return "" }}, []string{"bash", "-c", "curl example.com"}, false},
		{"leading env assignment", execOptions{Getenv: func(string) string { return "" }}, []string{"bash", "-c", "HARNEZ_EXPECT_FAILURE=1 curl example.com"}, true},
		{"leading env assignment, true value", execOptions{Getenv: func(string) string { return "" }}, []string{"bash", "-c", "HARNEZ_EXPECT_FAILURE=true curl example.com"}, true},
		{"other var before it", execOptions{Getenv: func(string) string { return "" }}, []string{"bash", "-c", "FOO=bar HARNEZ_EXPECT_FAILURE=1 curl example.com"}, true},
		{"not at start of command", execOptions{Getenv: func(string) string { return "" }}, []string{"bash", "-c", "curl example.com HARNEZ_EXPECT_FAILURE=1"}, false},
		{"process env set", execOptions{Getenv: func(k string) string {
			if k == expectFailureEnv {
				return "1"
			}
			return ""
		}}, []string{"git", "status"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detectExpectFailure(tc.opts, tc.args)
			if got != tc.want {
				t.Errorf("detectExpectFailure(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestRunExecWrapper_InfersTool(t *testing.T) {
	opts := testExecOptions(t)
	opts.Tool = ""
	code, err := runExecWrapper([]string{"true"}, opts, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil || code != 0 {
		t.Fatalf("expected inferred tool execution to succeed, got code %d, err %v", code, err)
	}
}

func TestRunExecWrapper_NoCommand(t *testing.T) {
	_, err := runExecWrapper(nil, testExecOptions(t), strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected an error when no command is given")
	}
}

// --- harnez exec hook ---

func TestRunExecHook_RewritesBashCommand(t *testing.T) {
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git status"}}`)
	var out bytes.Buffer
	if err := runExecHook(in, &out); err != nil {
		t.Fatalf("runExecHook() error = %v", err)
	}

	var got hookOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out.String())
	}
	want := "⚙ git status"
	if got.HookSpecificOutput.UpdatedInput["command"] != want {
		t.Errorf("updatedInput.command = %q, want %q", got.HookSpecificOutput.UpdatedInput["command"], want)
	}
	if got.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("hookEventName = %q, want PreToolUse", got.HookSpecificOutput.HookEventName)
	}
}

// TestRunExecHook_PreservesShellMetacharacters is the regression check for
// the bug found reviewing issue 119: Claude Code re-executes the rewritten
// command via its own outer 'bash -c', so a naive
// "⚙ -- <command>" splice lets that outer shell
// re-interpret pipes/&&/; in <command> instead of harnez exec ever seeing
// them as part of one wrapped command. Wrapping in a quoted 'bash -c'
// argument must keep the whole original command intact.
func TestRunExecHook_PreservesShellMetacharacters(t *testing.T) {
	original := `make build && make test | grep -v ok; echo "done"`
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":` + strconvQuote(original) + `}}`)
	var out bytes.Buffer
	if err := runExecHook(in, &out); err != nil {
		t.Fatalf("runExecHook() error = %v", err)
	}
	var got hookOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out.String())
	}
	rewritten := got.HookSpecificOutput.UpdatedInput["command"]

	// Simulate Claude Code's own outer 'bash -c <rewritten>' re-execution:
	// the original command's metacharacters must survive intact as a
	// single argument to the inner 'bash -c', not be re-split by the
	// outer shell.
	outerArgs, err := shlexSplitForTest(rewritten)
	if err != nil {
		t.Fatalf("outer shell failed to parse rewritten command %q: %v", rewritten, err)
	}
	want := []string{"bash", "-c", original}
	if len(outerArgs) != len(want) {
		t.Fatalf("outer-shell tokenization of %q = %v, want %v", rewritten, outerArgs, want)
	}
	for i := range want {
		if outerArgs[i] != want[i] {
			t.Errorf("outer-shell token[%d] = %q, want %q (full: %v)", i, outerArgs[i], want[i], outerArgs)
		}
	}
}

// TestRunExecHook_ComposesDistillAutopipe is the regression check for the
// dual-hook race found reviewing issue 119: with HARNEZ_DISTILL_AUTOPIPE
// set, this single hook must apply distill's own noisy-command rewrite
// itself rather than relying on a second, separately-installed
// PreToolUse/Bash hook (Claude Code does not compose two hooks'
// updatedInput rewrites on the same matcher).
func TestRunExecHook_ComposesDistillAutopipe(t *testing.T) {
	t.Setenv("HARNEZ_DISTILL_AUTOPIPE", "true")
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"go test ./..."}}`)
	var out bytes.Buffer
	if err := runExecHook(in, &out); err != nil {
		t.Fatalf("runExecHook() error = %v", err)
	}
	var got hookOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out.String())
	}
	rewritten := got.HookSpecificOutput.UpdatedInput["command"]
	if !strings.Contains(rewritten, "--distill") && !strings.Contains(rewritten, "harnez distill") {
		t.Errorf("updatedInput.command = %q, want it to route through distill (autopipe enabled)", rewritten)
	}
	if !strings.HasPrefix(rewritten, "⚙") {
		t.Errorf("updatedInput.command = %q, want it still wrapped by ⚙", rewritten)
	}
}

func TestRunExecHook_IgnoresNonBashTool(t *testing.T) {
	in := strings.NewReader(`{"tool_name":"Read","tool_input":{"command":"git status"}}`)
	var out bytes.Buffer
	if err := runExecHook(in, &out); err != nil {
		t.Fatalf("runExecHook() error = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for non-Bash tool, got %q", out.String())
	}
}

func TestRunExecHook_IgnoresAlreadyWrappedCommand(t *testing.T) {
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"harnez exec --tool Bash -- git status"}}`)
	var out bytes.Buffer
	if err := runExecHook(in, &out); err != nil {
		t.Fatalf("runExecHook() error = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for an already-wrapped command, got %q", out.String())
	}
}

func TestRunExecHook_IgnoresEmptyCommand(t *testing.T) {
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":""}}`)
	var out bytes.Buffer
	if err := runExecHook(in, &out); err != nil {
		t.Fatalf("runExecHook() error = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for an empty command, got %q", out.String())
	}
}

func TestRunExecHook_IgnoresGearCommand(t *testing.T) {
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"⚙ echo 'hello'"}}`)
	var out bytes.Buffer
	if err := runExecHook(in, &out); err != nil {
		t.Fatalf("runExecHook() error = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for a gear command, got %q", out.String())
	}
}

func TestIsGearInvocation(t *testing.T) {
	cases := []struct {
		arg0 string
		want bool
	}{
		{"⚙", true},
		{"⚙️", true},
		{"\xe2\x9a\x99", true},
		{"\xe2\x9a\x99\xef\xb8\x8f", true},
		{"/usr/local/bin/⚙", true},
		{"/home/user/.claude/bin/⚙", true},
		{"/home/user/go/bin/⚙️", true},
		{"./⚙", true},
		{"harnez", false},
		{"exec", false},
		{"/usr/local/bin/harnez", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.arg0, func(t *testing.T) {
			if got := isGearInvocation(tc.arg0); got != tc.want {
				t.Errorf("isGearInvocation(%q) = %v, want %v", tc.arg0, got, tc.want)
			}
		})
	}
}

func TestExecCmd_GearAliasAndFlags(t *testing.T) {
	cmd := newExecCmd()
	hasGearAlias := false
	for _, alias := range cmd.Aliases {
		if alias == "⚙" || alias == "⚙️" {
			hasGearAlias = true
			break
		}
	}
	if !hasGearAlias {
		t.Errorf("expected newExecCmd to have ⚙ alias, got %v", cmd.Aliases)
	}

	// Verify --expect-failure flag exists
	f := cmd.Flags().Lookup("expect-failure")
	if f == nil {
		t.Fatal("expected --expect-failure flag on exec command")
	}
	if f.Value.Type() != "bool" {
		t.Errorf("expected --expect-failure flag to be bool, got %s", f.Value.Type())
	}

	// Verify --tool flag exists and is optional
	tf := cmd.Flags().Lookup("tool")
	if tf == nil {
		t.Fatal("expected --tool flag on exec command")
	}
}

func TestGearMulticallExecution(t *testing.T) {
	// Build a real test binary of harnez to test os.Args[0] multicall symlink dispatch
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "harnez")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/harnez")
	buildCmd.Dir = filepath.Join("..", "..")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(out))
	}

	// Create ⚙ symlink pointing to harnez binary
	gearPath := filepath.Join(binDir, "⚙")
	if err := os.Symlink(binPath, gearPath); err != nil {
		t.Fatalf("create symlink failed: %v", err)
	}

	// This test runs the real compiled binary, which now (issue 326) writes
	// a cli_invocations row on every invocation via main()'s
	// executeAndRecord, on top of the pre-existing tool_calls/sessionstate
	// writes `exec` itself performs. Both telemetry.DefaultDBPath and
	// resolve.DefaultStateDir resolve purely from os.UserHomeDir() -- there
	// never was a HARNEZ_DB_PATH/HARNEZ_STATE_DIR override in production
	// code, so the env vars of those names set below were silently inert
	// and every subprocess invocation was actually writing into the
	// *developer's real* ~/.harnez/tool_catalog.sqlite (indistinguishable
	// from genuine usage: cwd is this package directory, so project_name
	// recorded as plain "harnez"). t.Setenv("HOME", ...) redirects
	// os.UserHomeDir() for the whole test -- the only override point that
	// actually exists -- matching the isolation pattern
	// internal/claude/bash_shim_test.go already uses.
	fakeHome := filepath.Join(binDir, "home")
	if err := os.MkdirAll(fakeHome, 0o755); err != nil {
		t.Fatalf("create fake home: %v", err)
	}
	t.Setenv("HOME", fakeHome)

	// 1. Test basic command execution: ⚙ echo hello
	cmd := exec.Command(gearPath, "echo", "hello from gear")
	cmd.Env = append(os.Environ(), "HARNEZ_DISABLE_RATE_FEEDBACK=1", "ANTIGRAVITY_CONVERSATION_ID=", "CLAUDE_CONVERSATION_ID=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("⚙ echo failed: %v\n%s", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "hello from gear" {
		t.Errorf("stdout = %q, want %q", strings.TrimSpace(string(out)), "hello from gear")
	}

	// 2. Test exit code forwarding: ⚙ sh -c 'exit 42'
	cmd = exec.Command(gearPath, "sh", "-c", "exit 42")
	cmd.Env = append(os.Environ(), "HARNEZ_DISABLE_RATE_FEEDBACK=1", "ANTIGRAVITY_CONVERSATION_ID=", "CLAUDE_CONVERSATION_ID=")
	err = cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code 42, got nil")
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ExitCode() != 42 {
			t.Errorf("exit code = %d, want 42", exitErr.ExitCode())
		}
	} else {
		t.Fatalf("unexpected error type: %v", err)
	}

	// 3. Test telemetry recording with --tool and --expect-failure flags
	cmd = exec.Command(gearPath, "--tool", "custom-tool", "--expect-failure", "--", "sh", "-c", "exit 3")
	cmd.Env = os.Environ()
	err = cmd.Run()
	if err == nil {
		t.Fatal("expected exit code 3")
	}
	if errors.As(err, &exitErr) && exitErr.ExitCode() != 3 {
		t.Errorf("exit code = %d, want 3", exitErr.ExitCode())
	}

	// Verify telemetry row, under the isolated fakeHome this test actually
	// controls (there is no env-var override to point at a different path).
	dbPath := filepath.Join(fakeHome, ".harnez", "tool_catalog.sqlite")
	db, dbErr := telemetry.Open(dbPath)
	if dbErr != nil {
		t.Fatalf("open isolated telemetry db %s: %v", dbPath, dbErr)
	}
	defer db.Close()
	rows, qErr := db.Query(telemetry.Filter{CallType: telemetry.ExpectedFailureCallType})
	if qErr != nil {
		t.Fatalf("query telemetry rows: %v", qErr)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least one expected-failure telemetry row, got none")
	}
	if rows[0].ToolName != "custom-tool" {
		t.Errorf("tool_name = %q, want custom-tool", rows[0].ToolName)
	}
	if rows[0].ExitCode == nil || *rows[0].ExitCode != 3 {
		t.Errorf("exit_code = %v, want 3", rows[0].ExitCode)
	}
}

func TestInferToolFromArgs(t *testing.T) {
	cases := []struct {
		args        []string
		defaultTool string
		want        string
	}{
		{nil, "Bash", "Bash"},
		{[]string{"git", "status"}, "Bash", "git"},
		{[]string{"/usr/bin/npm", "test"}, "Bash", "npm"},
		{[]string{"bash", "-c", "git status && echo done"}, "Bash", "git"},
		{[]string{"sh", "-c", "FOO=bar /usr/bin/go test ./..."}, "Bash", "go"},
		{[]string{"bash", "-c", "sudo apt update"}, "Bash", "apt"},
		{[]string{"bash", "-c", "sudo -u root npm test"}, "Bash", "npm"},
		{[]string{"bash", "-c", "echo hello"}, "Bash", "echo"},
		{[]string{"git", "diff"}, "CustomTool", "CustomTool"},
		// issue 344: redirect target, keyword, operator, and bare
		// filename tokens must never be mistaken for a tool name.
		{[]string{"bash", "-c", "npm test 2>/dev/null || true"}, "Bash", "npm"},
		{[]string{"bash", "-c", "2>/dev/null || true"}, "Bash", "Bash"},
		{[]string{"bash", "-c", "for f in *.lock; do echo $f; done"}, "Bash", "Bash"},
		{[]string{"bash", "-c", "make check && make install"}, "Bash", "make"},
		{[]string{"README.md"}, "Bash", "Bash"},
	}
	for _, tc := range cases {
		got := inferToolFromArgs(tc.args, tc.defaultTool)
		if got != tc.want {
			t.Errorf("inferToolFromArgs(%v, %q) = %q, want %q", tc.args, tc.defaultTool, got, tc.want)
		}
	}
}

func TestIsSimpleShellCommand(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"git status", true},
		{"git diff --stat", true},
		{"npm test", true},
		{"sqlite3 ~/.harnez/tool_catalog.sqlite \"SELECT * FROM tool_calls\"", true},
		{"sqlite3 db \"SELECT 1;\"", false},
		{"make check", true},
		{"go test ./...", true},
		{"cd /tmp", false},
		{"read -r line", false},
		{"! git diff --quiet", false},
		{"[[ -f foo ]]", false},
		{"--version", false},
		{"-h", false},
		{"git status && echo done", false},
		{"git status || echo fail", false},
		{"git status; echo done", false},
		{"git log | grep fix", false},
		{"echo hello > /tmp/out", false},
		{"echo hello >> /tmp/out", false},
		{"cat < /tmp/in", false},
		{"echo $(pwd)", false},
		{"echo `pwd`", false},
		{"VAR=1 git status", false},
		{"export FOO=bar", false},
		{"for f in *.go; do echo $f; done", false},
		{"if test -f foo; then echo yes; fi", false},
		{"", false},
		{"   ", false},
	}
	for _, tc := range cases {
		got := isSimpleShellCommand(tc.cmd)
		if got != tc.want {
			t.Errorf("isSimpleShellCommand(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

func TestFormatGearRewrite(t *testing.T) {
	cases := []struct {
		cmd         string
		distillFlag string
		want        string
	}{
		{"git status", "", "⚙ git status"},
		{"git status && echo done", "", "⚙ bash -c 'git status && echo done'"},
		{"go test ./...", "--distill", "⚙ --distill -- bash -c 'go test ./...'"},
		{"VAR=1 git diff", "", "⚙ bash -c 'VAR=1 git diff'"},
		{"cd /tmp", "", "⚙ bash -c 'cd /tmp'"},
	}
	for _, tc := range cases {
		got := formatGearRewrite(tc.cmd, tc.distillFlag)
		if got != tc.want {
			t.Errorf("formatGearRewrite(%q, %q) = %q, want %q", tc.cmd, tc.distillFlag, got, tc.want)
		}
	}
}

func TestAlreadyRoutedThroughExec_WithEnvPrefix(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"⚙ git status", true},
		{"HARNEZ_EXPECT_FAILURE=1 ⚙ git status", true},
		{"FOO=bar BAR=baz ⚙ echo test", true},
		{"harnez exec -- git status", true},
		{"HARNEZ_EXPECT_FAILURE=1 harnez exec -- git status", true},
		{"git status", false},
		{"VAR=1 git status", false},
	}
	for _, tc := range cases {
		got := alreadyRoutedThroughExec(tc.cmd)
		if got != tc.want {
			t.Errorf("alreadyRoutedThroughExec(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}
