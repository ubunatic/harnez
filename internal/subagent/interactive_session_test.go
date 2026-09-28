package subagent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindCodexInteractiveSessionID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	started := time.Now()
	path := filepath.Join(home, ".codex", "sessions", "2026", "09", "28", "rollout-2026-09-28T12-00-00-test-thread.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := `{"type":"session_meta","payload":{"id":"test-thread","cwd":"/work","timestamp":"` + started.Add(time.Second).Format(time.RFC3339Nano) + `"}}` + "\n"
	if err := os.WriteFile(path, []byte(metadata), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := findCodexInteractiveSessionID(home, 0, "/work", started); got != "test-thread" {
		t.Fatalf("Codex ID = %q, want test-thread", got)
	}
	if got := findCodexInteractiveSessionID(home, 0, "/elsewhere", started); got != "" {
		t.Fatalf("mismatched cwd yielded Codex ID %q", got)
	}
}

func TestFindAgyInteractiveSessionIDUsesPIDLockAndDatabase(t *testing.T) {
	home := t.TempDir()
	procRoot := t.TempDir()
	const pid = 4242
	fdDir := filepath.Join(procRoot, "4242", "fd")
	if err := os.MkdirAll(fdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	id := "123e4567-e89b-12d3-a456-426614174000"
	lock := filepath.Join(home, ".gemini", "antigravity-cli", "presence", id+".lock")
	if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(lock, filepath.Join(fdDir, "4")); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(home, ".gemini", "antigravity-cli", "conversations", id+".db")
	if got := findAgyInteractiveSessionID(home, procRoot, pid); got != "" {
		t.Fatalf("recorded ID before conversation DB existed: %q", got)
	}
	if err := os.MkdirAll(filepath.Dir(db), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(db, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := findAgyInteractiveSessionID(home, procRoot, pid); got != id {
		t.Fatalf("agy ID = %q, want %s", got, id)
	}
}
