package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/telemetry"
)

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
	skipShort(t, "builds and runs the harnez binary")
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

	// Isolate the real multicall subprocesses under a fake home. Their
	// telemetry is spooled while no server is running, then drained through
	// the server-owned writer below.
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

	config, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(fakeHome, ".local", "share", "harnez", "telemetry.sqlite")
	service := &telemetryCollectorService{config: config, path: dbPath, interval: time.Hour}
	if err := service.drainSpool(context.Background()); err != nil {
		t.Fatalf("drain telemetry spool: %v", err)
	}
	db, dbErr := telemetry.OpenReadOnly(dbPath)
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
func TestNewExecCmd_Quota1Flag(t *testing.T) {
	cmd := newExecCmd()
	f := cmd.Flags().Lookup("quota-1")
	if f == nil {
		t.Fatal("expected --quota-1 flag to be registered")
	}
	if f.DefValue != "false" {
		t.Errorf("flag default = %q, want false", f.DefValue)
	}
}
