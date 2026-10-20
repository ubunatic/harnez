package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitAgentFileMigratesLegacyAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "CLAUDE.md")
	path := filepath.Join(dir, "AGENTS.md")
	const content = "# My project\nUser-owned text.\n"
	if err := os.WriteFile(legacy, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	r := &initReport{}
	if safe, err := initAgentFile(r, dir, nil, false); err != nil || !safe || r.changes != 2 {
		t.Fatalf("migration: safe=%v changes=%d err=%v", safe, r.changes, err)
	}
	if target, err := os.Readlink(legacy); err != nil || target != "AGENTS.md" {
		t.Fatalf("symlink target=%q err=%v", target, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != content {
		t.Fatalf("migrated content=%q err=%v", data, err)
	}
	again := &initReport{}
	if safe, err := initAgentFile(again, dir, nil, false); err != nil || !safe || again.changes != 0 {
		t.Fatalf("repeat: safe=%v changes=%d err=%v", safe, again.changes, err)
	}
}
