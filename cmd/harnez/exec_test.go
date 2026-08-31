package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
)

// testExecOptions returns execOptions isolated from the caller's real
// environment/DB: a throwaway state dir and DB path, explicit ticket so
// resolve.Ticket never has to walk this repo's own git history, and a
// tight-but-real Getenv so no ambient CLAUDE_CODE_SESSION_ID etc. leaks
// in from the actual test-runner environment.
func testExecOptions(t *testing.T) execOptions {
	t.Helper()
	dir := t.TempDir()
	return execOptions{
		Tool:      "test-tool",
		Ticket:    "harnez/118-test-ticket",
		Getenv:    func(string) string { return "" },
		StateDir:  filepath.Join(dir, "state"),
		DBPath:    filepath.Join(dir, "tool_catalog.sqlite"),
		TicketDir: dir,
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
		t.Errorf("distilled_bytes = %v, want nil/NULL (no distill byte-count signal exists yet, see issues/118 Notes)", *row.DistilledBytes)
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
	want := "harnez exec --tool Bash -- git status"
	if got.HookSpecificOutput.UpdatedInput["command"] != want {
		t.Errorf("updatedInput.command = %q, want %q", got.HookSpecificOutput.UpdatedInput["command"], want)
	}
	if got.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("hookEventName = %q, want PreToolUse", got.HookSpecificOutput.HookEventName)
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
