package telemetry

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// Issue 735: when the XDG database already exists, the legacy rows must be
// merged before the legacy file is renamed, and overlapping rows stay single.
func TestDefaultDBPathMergesLegacyIntoExistingDB(t *testing.T) {
	home, data := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", data)
	legacy := filepath.Join(home, ".harnez", "tool_catalog.sqlite")
	current := filepath.Join(data, "harnez", "telemetry.sqlite")
	shared := sampleCall("s", "overlap", 4, 0)
	writeCalls(t, legacy, sampleCall("s", "old-only", 4, 0), shared)
	writeCalls(t, current, shared, sampleCall("s", "new-only", 4, 0))

	got, err := DefaultDBPath()
	if err != nil || got != current {
		t.Fatalf("DefaultDBPath = %q, %v", got, err)
	}
	db, err := Open(current)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if n, err := db.ToolCallCount(context.Background()); err != nil || n != 3 {
		t.Fatalf("tool calls = %d, %v; want 3 (old-only, overlap, new-only)", n, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy path still exists: %v", err)
	}
}

// Merging the same file twice adds nothing the second time; a file without
// a tool_calls table adds nothing.
func TestMergeLegacyToolCallsIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "legacy.sqlite")
	writeCalls(t, legacy, sampleCall("s", "a", 4, 0), sampleCall("s", "b", 4, 0))
	db, err := Open(filepath.Join(dir, "live.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if missing, err := db.LegacyToolCallsMissing(ctx, legacy); err != nil || missing != 2 {
		t.Fatalf("missing = %d, %v; want 2", missing, err)
	}
	// A file without a tool_calls table and an overlapping second file
	// merge in the same transaction.
	empty := filepath.Join(dir, "empty.sqlite")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	overlap := filepath.Join(dir, "overlap.sqlite")
	writeCalls(t, overlap, sampleCall("s", "c", 4, 0))
	for i, want := range [][3]int64{{2, 0, 1}, {0, 0, 0}} {
		res, err := db.MergeLegacyToolCalls(ctx, []string{legacy, empty, overlap})
		if err != nil {
			t.Fatal(err)
		}
		for j := range want {
			if res[j].Added != want[j] {
				t.Fatalf("merge %d: %s added %d, want %d", i, res[j].Source, res[j].Added, want[j])
			}
		}
	}
	if n, err := db.ToolCallCount(ctx); err != nil || n != 3 {
		t.Fatalf("tool calls = %d, %v; want 3", n, err)
	}
	backup := filepath.Join(dir, "backup.sqlite")
	if err := db.BackupInto(ctx, backup); err != nil {
		t.Fatal(err)
	}
	copyDB, err := Open(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	if n, err := copyDB.ToolCallCount(ctx); err != nil || n != 3 {
		t.Fatalf("backup rows = %d, %v; want 3", n, err)
	}
}

func writeCalls(t *testing.T, path string, calls ...ToolCall) {
	t.Helper()
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, call := range calls {
		if err := db.Insert(call); err != nil {
			t.Fatal(err)
		}
	}
}

// A row the live table rejects rolls back the whole merge, including the
// rows of earlier files, and leaves the database usable for a retry.
func TestMergeLegacyToolCallsRollsBackAllFiles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	good := filepath.Join(dir, "good.sqlite")
	writeCalls(t, good, sampleCall("s", "a", 4, 0))
	bad := filepath.Join(dir, "bad.sqlite")
	raw, err := sql.Open("sqlite", bad)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE tool_calls (id INTEGER PRIMARY KEY, created_at TEXT, session_id TEXT, agent_id TEXT, tool_name TEXT, call_type TEXT);
		INSERT INTO tool_calls (created_at, session_id, tool_name, call_type) VALUES ('2026-09-01T00:00:00Z', 's', 'Bash', 'hook')`); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	db, err := Open(filepath.Join(dir, "live.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.MergeLegacyToolCalls(ctx, []string{good, bad}); err == nil {
		t.Fatal("merge with a NULL agent_id row succeeded")
	}
	if n, err := db.ToolCallCount(ctx); err != nil || n != 0 {
		t.Fatalf("tool calls after failed merge = %d, %v; want 0", n, err)
	}
	res, err := db.MergeLegacyToolCalls(ctx, []string{good})
	if err != nil || res[0].Added != 1 {
		t.Fatalf("retry = %v, %v; want 1 added", res, err)
	}
}
