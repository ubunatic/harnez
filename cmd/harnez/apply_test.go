// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
	"ubunatic.com/harnez/internal/telemetry"
)

func captureApplyStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = write
	fn()
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	out, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if err := read.Close(); err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestEnsureTelemetrySchemaMigratesBeforeApply(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".harnez", "tool_catalog.sqlite")
	db, err := telemetry.Open(path)
	if err != nil {
		t.Fatalf("create telemetry DB: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close telemetry DB: %v", err)
	}

	output := captureApplyStdout(t, func() {
		if err := ensureTelemetrySchema(); err != nil {
			t.Fatalf("ensureTelemetrySchema: %v", err)
		}
	})
	if strings.Contains(output, "telemetry schema:") {
		t.Fatalf("no-op schema initialization printed telemetry report: %q", output)
	}

	check, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("reopen migrated telemetry DB: %v", err)
	}
	defer check.Close()
	var version int
	if err := check.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != 9 {
		t.Fatalf("schema version = %d, want 9", version)
	}
	for _, column := range []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_tokens", "total_tokens"} {
		var count int
		if err := check.QueryRow("SELECT count(*) FROM pragma_table_info('tool_calls') WHERE name = ?", column).Scan(&count); err != nil {
			t.Fatalf("check %s: %v", column, err)
		}
		if count != 1 {
			t.Errorf("missing migrated column %s", column)
		}
	}
	for _, table := range []string{"compaction_events", "session_boundaries", "token_snapshots"} {
		var count int
		if err := check.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count); err != nil {
			t.Fatalf("check %s table: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("missing migrated table %s", table)
		}
	}
}

func TestApplyCmdMigratesLegacyCompactionSchema(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".harnez", "tool_catalog.sqlite")
	db, err := telemetry.Open(path)
	if err != nil {
		t.Fatalf("create telemetry DB: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close telemetry DB: %v", err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("reopen telemetry DB: %v", err)
	}
	_, err = raw.Exec(`
		ALTER TABLE compaction_events RENAME TO compaction_events_current;
		CREATE TABLE compaction_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TEXT NOT NULL,
			session_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			turn_id TEXT NOT NULL DEFAULT '',
			trigger TEXT NOT NULL DEFAULT '',
			reason TEXT NOT NULL DEFAULT '',
			input_tokens INTEGER,
			cached_input_tokens INTEGER,
			output_tokens INTEGER,
			reasoning_tokens INTEGER,
			 total_tokens INTEGER
		);
		INSERT INTO compaction_events (created_at, session_id, event_type)
			SELECT created_at, session_id, event_type FROM compaction_events_current;
		DROP TABLE compaction_events_current;
		PRAGMA user_version = 5;
	`)
	if err != nil {
		raw.Close()
		t.Fatalf("create legacy telemetry schema: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close legacy telemetry DB: %v", err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"apply", "-t", filepath.Join(home, ".claude")})
	output := captureApplyStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("apply failed: %v", err)
		}
	})
	if !bytes.Contains([]byte(output), []byte("telemetry schema: v9 (migrations:")) {
		t.Fatalf("migration output missing telemetry report: %q", output)
	}

	check, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("reopen migrated telemetry DB: %v", err)
	}
	defer check.Close()
	var version int
	if err := check.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != 9 {
		t.Fatalf("schema version = %d, want 9", version)
	}
	var modelPresent int
	if err := check.QueryRow("SELECT count(*) FROM pragma_table_info('compaction_events') WHERE name = 'model'").Scan(&modelPresent); err != nil {
		t.Fatalf("check compaction_events.model: %v", err)
	}
	if modelPresent != 1 {
		t.Fatal("compaction_events.model was not added by apply")
	}
}

func TestApplyCmd_ZeroDefaultDocs(t *testing.T) {
	dir := t.TempDir()
	cmd := newRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"apply", "-t", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	goDoc := filepath.Join(dir, "docs", "Go.md")
	if _, err := os.Stat(goDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Go.md not to be installed by default on bare apply")
	}
}

func TestApplyCmd_OptInDocs(t *testing.T) {
	dir := t.TempDir()
	cmd := newRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"apply", "-t", dir, "-d", "golang"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("apply -d golang failed: %v", err)
	}

	goDoc := filepath.Join(dir, "docs", "Go.md")
	if _, err := os.Stat(goDoc); err != nil {
		t.Fatalf("expected Go.md to be installed when -d golang passed: %v", err)
	}

	bashDoc := filepath.Join(dir, "docs", "Bash.md")
	if _, err := os.Stat(bashDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Bash.md not to be installed")
	}

	// Now run apply with --clean-docs and no doc flags -> Go.md should be cleaned
	cmd2 := newRootCmd()
	var stdout2, stderr2 bytes.Buffer
	cmd2.SetOut(&stdout2)
	cmd2.SetErr(&stderr2)
	cmd2.SetArgs([]string{"apply", "-t", dir, "--clean-docs"})

	if err := cmd2.Execute(); err != nil {
		t.Fatalf("apply --clean-docs failed: %v", err)
	}

	if _, err := os.Stat(goDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Go.md to be cleaned up after apply --clean-docs")
	}
}

func TestApplyCmd_RemoveDocsFlag(t *testing.T) {
	dir := t.TempDir()
	// First install a doc
	cmd := newRootCmd()
	cmd.SetArgs([]string{"apply", "-t", dir, "-d", "golang"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("apply -d golang failed: %v", err)
	}

	goDoc := filepath.Join(dir, "docs", "Go.md")
	if _, err := os.Stat(goDoc); err != nil {
		t.Fatalf("expected Go.md to exist: %v", err)
	}

	// Now remove with --remove-docs
	cmd2 := newRootCmd()
	cmd2.SetArgs([]string{"apply", "-t", dir, "--remove-docs"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("apply --remove-docs failed: %v", err)
	}

	if _, err := os.Stat(goDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Go.md to be removed by --remove-docs")
	}
}
