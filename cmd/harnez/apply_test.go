// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
	"ubunatic.com/harnez/internal/telemetry"
)

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

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open stale telemetry DB: %v", err)
	}
	if _, err := raw.Exec("PRAGMA user_version = 3"); err != nil {
		raw.Close()
		t.Fatalf("stamp stale schema version: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close stale telemetry DB: %v", err)
	}

	if err := ensureTelemetrySchema(); err != nil {
		t.Fatalf("ensureTelemetrySchema: %v", err)
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
	if version != 4 {
		t.Fatalf("schema version = %d, want 4", version)
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
