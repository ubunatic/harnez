package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/telemetry"
)

// TestRateExecStats_UnaffectedByRateFeedbackDisableEnv is issue 142's
// acceptance criterion 3: disabling the Tool Feedback Protocol
// *instruction injection* (HARNEZ_DISABLE_RATE_FEEDBACK, consulted only by
// claude.RateFeedbackDisabled/ApplyAll when rendering AGENTS.md/CLAUDE.md
// and skills) must not silently disable unrelated telemetry — `harnez
// rate` itself, `harnez exec`, and `harnez stats` all keep recording and
// reporting normally regardless of the env var, since none of runRate,
// runExecWrapper, or runStats ever consults it.
func TestRateExecStats_UnaffectedByRateFeedbackDisableEnv(t *testing.T) {
	t.Setenv("HARNEZ_DISABLE_RATE_FEEDBACK", "1")

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tool_catalog.sqlite")
	stateDir := filepath.Join(dir, "state")
	getenv := func(string) string { return "" }

	// harnez rate still writes a call_type=internal row.
	if err := runRate([]string{"Read", "5", "flawless"}, rateOptions{
		SessionFlag: "sess-toggle",
		DBPath:      dbPath,
		StateDir:    stateDir,
		Getenv:      getenv,
	}); err != nil {
		t.Fatalf("runRate: %v", err)
	}

	// harnez exec still writes a call_type=shell row.
	var out, errOut bytes.Buffer
	exitCode, err := runExecWrapper([]string{"true"}, execOptions{
		Tool:             "test-tool",
		Ticket:           "harnez/142-toggle",
		Getenv:           getenv,
		StateDir:         stateDir,
		ProcessRecordDir: filepath.Join(dir, "procs"),
		DBPath:           dbPath,
	}, os.Stdin, &out, &errOut)
	if err != nil {
		t.Fatalf("runExecWrapper: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("runExecWrapper exit code = %d, want 0", exitCode)
	}

	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	rows, err := db.Query(telemetry.Filter{})
	db.Close()
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	var sawInternal, sawShell bool
	for _, r := range rows {
		switch r.CallType {
		case "internal":
			sawInternal = true
		case "shell":
			sawShell = true
		}
	}
	if !sawInternal {
		t.Errorf("expected a call_type=internal row from harnez rate even with the instruction-injection env var set, rows=%+v", rows)
	}
	if !sawShell {
		t.Errorf("expected a call_type=shell row from harnez exec even with the instruction-injection env var set, rows=%+v", rows)
	}

	// harnez stats still reports both rows normally.
	var statsBuf bytes.Buffer
	if err := runStats(&statsBuf, statsOptions{DBPath: dbPath}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	statsOut := statsBuf.String()
	if !strings.Contains(statsOut, "Read") || !strings.Contains(statsOut, "test-tool") {
		t.Errorf("expected harnez stats to report both the rate and exec rows unaffected by the disable env var, got:\n%s", statsOut)
	}
}
