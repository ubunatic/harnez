package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/telemetry"
)

// Issue 735: merge-legacy backs up the live DB, adds only missing rows and
// reports counts; a second run adds nothing and takes no backup.
func TestStatsMergeLegacyBacksUpAndReportsCounts(t *testing.T) {
	dir := t.TempDir()
	live := filepath.Join(dir, "telemetry.sqlite")
	legacy := filepath.Join(dir, "tool_catalog.sqlite.bak-1")
	at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	call := func(tool string) telemetry.ToolCall {
		return telemetry.ToolCall{CreatedAt: at, SessionID: "s", AgentID: "claude", ToolName: tool, CallType: "shell"}
	}
	for path, calls := range map[string][]telemetry.ToolCall{legacy: {call("old"), call("both")}, live: {call("both")}} {
		db, err := telemetry.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range calls {
			if err := db.Insert(c); err != nil {
				t.Fatal(err)
			}
		}
		db.Close()
	}
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	output, err := server.MergeLegacyToolCalls(context.Background(), live, []string{legacy}, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"tool calls before: 1", "backup: " + live + ".before-merge-20261006T120000Z", "added 1 from " + legacy, "tool calls after: 2"} {
		if !strings.Contains(output, want) {
			t.Errorf("output %q missing %q", output, want)
		}
	}
	output, err = server.MergeLegacyToolCalls(context.Background(), live, []string{legacy}, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "nothing to merge") || strings.Contains(output, "backup:") {
		t.Errorf("second run output = %q", output)
	}
}

func TestLegacyDatabaseFilesSkipsSQLiteSideFiles(t *testing.T) {
	got := legacyDatabaseFiles([]string{"a.bak-1", "a.bak-1-wal", "a.bak-1-shm", "a.bak-2-journal", "a.bak-2"})
	if strings.Join(got, ",") != "a.bak-1,a.bak-2" {
		t.Fatalf("files = %v", got)
	}
}
