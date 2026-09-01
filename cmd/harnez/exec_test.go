package main

import (
	"bytes"
	"encoding/json"
	"errors"
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
	script := "harnez() { for a in \"$@\"; do printf '%s" + sep + "' \"$a\"; done; }\n" + rewritten
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

func TestRunExecWrapper_MissingTool(t *testing.T) {
	opts := testExecOptions(t)
	opts.Tool = ""
	_, err := runExecWrapper([]string{"true"}, opts, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected an error when --tool is empty")
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
	want := "harnez exec --tool Bash -- bash -c 'git status'"
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
// "harnez exec --tool Bash -- <command>" splice lets that outer shell
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
	// "$@" inside the shadowing function excludes the command name itself.
	want := []string{"exec", "--tool", "Bash", "--", "bash", "-c", original}
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
	if !strings.HasPrefix(rewritten, "harnez exec --tool Bash") {
		t.Errorf("updatedInput.command = %q, want it still wrapped by harnez exec", rewritten)
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
