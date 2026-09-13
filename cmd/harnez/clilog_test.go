package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/telemetry"
)

// TestRedactArgs covers issue 326's privacy contract for the args column:
// flag names survive, values and prose-shaped positionals do not.
func TestRedactArgs(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "bare subcommand path is kept verbatim",
			argv: []string{"issues", "new"},
			want: "issues new",
		},
		{
			name: "boolean flags are kept",
			argv: []string{"find", "issues", "--json", "--all"},
			want: "find issues --json --all",
		},
		{
			name: "deny-listed flag value is redacted (separate token)",
			argv: []string{"usage", "--host", "workstation.local"},
			want: "usage --host " + redactedPlaceholder,
		},
		{
			name: "deny-listed flag value is redacted (equals form)",
			argv: []string{"usage", "--host=workstation.local"},
			want: "usage --host=" + redactedPlaceholder,
		},
		{
			name: "deny-listed shorthand value is redacted",
			argv: []string{"index", "-d", "harnez"},
			want: "index -d " + redactedPlaceholder,
		},
		{
			name: "safe value of a non-denied flag survives",
			argv: []string{"stats", "--project", "harnez"},
			want: "stats --project harnez",
		},
		{
			name: "prose positional is redacted",
			argv: []string{"rate", "Grep", "1", "missed target, wrong file"},
			want: "rate Grep 1 " + redactedPlaceholder,
		},
		{
			name: "absolute path positional is redacted",
			argv: []string{"assess", "/home/uwe/projects/harnez"},
			want: "assess " + redactedPlaceholder,
		},
		{
			name: "numeric bare limit survives",
			argv: []string{"log", "-5"},
			want: "log -5",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactArgs(tc.argv); got != tc.want {
				t.Errorf("redactArgs(%q) = %q, want %q", tc.argv, got, tc.want)
			}
		})
	}
}

// TestRedactArgs_NeverLeaksDenyListedValue is the ticket's explicit
// verification item, stated as a property rather than a golden string: no
// deny-listed flag's value may appear anywhere in the stored args.
func TestRedactArgs_NeverLeaksDenyListedValue(t *testing.T) {
	secret := "s3cret-host"
	for _, flag := range []string{"--host", "--token", "--out", "--session", "--ticket", "--config", "-t", "-d", "-c", "-o"} {
		for _, argv := range [][]string{{"x", flag, secret}, {"x", flag + "=" + secret}} {
			if got := redactArgs(argv); strings.Contains(got, secret) {
				t.Errorf("redactArgs(%q) leaked the value: %q", argv, got)
			}
		}
	}
}

// writeBlocker creates a regular file at path, so a DB path underneath it
// can never be created — a deterministic "unopenable telemetry DB".
func writeBlocker(path string) error {
	return os.WriteFile(path, []byte("x"), 0o644)
}

// testRoot builds a minimal cobra tree mirroring the real root's shape (a
// nested subcommand plus a deliberately-failing one) for the recording
// tests below.
func testRoot() *cobra.Command {
	root := &cobra.Command{Use: "harnez", SilenceErrors: true, SilenceUsage: true}
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	ok := &cobra.Command{Use: "status", RunE: func(*cobra.Command, []string) error { return nil }}
	boom := &cobra.Command{Use: "boom", RunE: func(*cobra.Command, []string) error { return fmt.Errorf("deliberate failure") }}
	issues := &cobra.Command{Use: "issues"}
	issues.AddCommand(&cobra.Command{Use: "new", RunE: func(*cobra.Command, []string) error { return nil }})
	root.AddCommand(ok, boom, issues)
	return root
}

// recordOpts points the write path at a throwaway DB and a synthetic
// environment so it never touches the user's real telemetry DB.
func recordOpts(t *testing.T) cliLogOptions {
	t.Helper()
	dir := t.TempDir()
	return cliLogOptions{
		DBPath:   filepath.Join(dir, "tool_catalog.sqlite"),
		StateDir: filepath.Join(dir, "sessions"),
		Getenv:   func(string) string { return "" },
	}
}

func readRows(t *testing.T, opts cliLogOptions) []telemetry.CLIInvocation {
	t.Helper()
	db, err := telemetry.Open(opts.DBPath)
	if err != nil {
		t.Fatalf("open telemetry db: %v", err)
	}
	defer db.Close()
	rows, err := db.QueryCLIInvocations(telemetry.Filter{}, 0)
	if err != nil {
		t.Fatalf("QueryCLIInvocations: %v", err)
	}
	return rows
}

// TestExecuteAndRecord_SuccessfulCommand asserts a successful run writes
// exactly one row with exit_code 0.
func TestExecuteAndRecord_SuccessfulCommand(t *testing.T) {
	opts := recordOpts(t)
	if err := executeAndRecord(testRoot(), []string{"status"}, opts); err != nil {
		t.Fatalf("executeAndRecord: %v", err)
	}

	rows := readRows(t, opts)
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 row, got %d", len(rows))
	}
	if rows[0].Command != "status" {
		t.Errorf("Command = %q, want %q", rows[0].Command, "status")
	}
	if rows[0].ExitCode == nil || *rows[0].ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", rows[0].ExitCode)
	}
	if rows[0].SessionID == "" {
		t.Error("SessionID is empty; the PPID session fallback should always resolve one")
	}
}

// TestExecuteAndRecord_FailingSubcommand is issue 326's single most
// important assertion: Cobra's PersistentPostRunE does NOT fire when RunE
// returns an error, so a hook-based implementation would silently lose
// exactly the invocations most worth recording. A failing subcommand must
// still produce a row, with a non-zero exit_code.
func TestExecuteAndRecord_FailingSubcommand(t *testing.T) {
	opts := recordOpts(t)
	if err := executeAndRecord(testRoot(), []string{"boom"}, opts); err == nil {
		t.Fatal("expected the failing subcommand to return an error")
	}

	rows := readRows(t, opts)
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 row for the failed run, got %d", len(rows))
	}
	if rows[0].Command != "boom" {
		t.Errorf("Command = %q, want %q", rows[0].Command, "boom")
	}
	if rows[0].ExitCode == nil || *rows[0].ExitCode == 0 {
		t.Errorf("ExitCode = %v, want non-zero", rows[0].ExitCode)
	}
}

// TestExecuteAndRecord_FullCommandPath asserts the command column records
// the full cobra path ("issues new"), not just the leaf name.
func TestExecuteAndRecord_FullCommandPath(t *testing.T) {
	opts := recordOpts(t)
	if err := executeAndRecord(testRoot(), []string{"issues", "new"}, opts); err != nil {
		t.Fatalf("executeAndRecord: %v", err)
	}

	rows := readRows(t, opts)
	if len(rows) != 1 || rows[0].Command != "issues new" {
		t.Fatalf("expected one row with command %q, got %+v", "issues new", rows)
	}
}

// TestExecuteAndRecord_OptOut asserts HARNEZ_DISABLE_CLI_LOG suppresses the
// write entirely — checked before any DB work, so no database file is even
// created.
func TestExecuteAndRecord_OptOut(t *testing.T) {
	opts := recordOpts(t)
	opts.Getenv = func(name string) string {
		if name == disableCLILogEnv {
			return "1"
		}
		return ""
	}
	if err := executeAndRecord(testRoot(), []string{"status"}, opts); err != nil {
		t.Fatalf("executeAndRecord: %v", err)
	}
	if _, err := os.Stat(opts.DBPath); !os.IsNotExist(err) {
		t.Fatalf("opt-out still created a telemetry db at %s (stat err: %v)", opts.DBPath, err)
	}
}

// TestExecuteAndRecord_UnwritableDBIsSilent asserts the best-effort
// contract: an unopenable telemetry DB must not fail the command or emit
// any output.
func TestExecuteAndRecord_UnwritableDBIsSilent(t *testing.T) {
	opts := recordOpts(t)
	// A path whose parent is an existing *file* can never be created as a
	// directory, so telemetry.Open fails deterministically.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := writeBlocker(blocker); err != nil {
		t.Fatalf("seed blocker file: %v", err)
	}
	opts.DBPath = filepath.Join(blocker, "sub", "tool_catalog.sqlite")

	root := testRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	if err := executeAndRecord(root, []string{"status"}, opts); err != nil {
		t.Fatalf("an unwritable telemetry db must not fail the command: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("best-effort recording printed output: %q", out.String())
	}
}
