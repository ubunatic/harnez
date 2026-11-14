package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func TestAgentCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	dir := filepath.Join(home, "sessions", "2026", "11", "14")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	id := "01234567-1234-1234-1234-123456789abc"
	data, err := os.ReadFile("../../internal/transcript/testdata/codex-commands.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-2026-11-14T00-00-00-"+id+".jsonl")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	store := t.TempDir()
	saveSessions(t, store, &subagent.Session{ID: "managed-123", Name: "worker", Provider: "codex", ProviderSessionID: id})
	for _, args := range [][]string{{"commands", "--name", "worker"}, {"commands", "managed-1"}, {"commands", id}, {"commands", path}} {
		got, err := runWithStore(t, nil, store, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if got != "pwd\ngit status\nprintf hi\n'bash' '-lc' 'echo hi'\n" {
			t.Fatalf("%v: %q", args, got)
		}
	}
	for _, args := range [][]string{{"commands"}, {"commands", "missing"}, {"commands", "--name", "missing"}, {"commands", path, "--name", "worker"}} {
		_, err := runWithStore(t, nil, store, args...)
		if err == nil {
			t.Fatalf("%v: expected error", args)
		}
	}
	saveSessions(t, store, &subagent.Session{ID: "managed-456", Name: "other", Provider: "codex", ProviderSessionID: id})
	_, err = runWithStore(t, nil, store, "commands", "managed-")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous: %v", err)
	}
}

func TestAgentCommandsClaudeAndDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", home)
	project := filepath.Join(home, "projects", "project")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	id := "claude-session"
	data := []byte("{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"tool_use\",\"name\":\"Bash\",\"input\":{\"command\":\"one\\r\\ntwo\\nthree\"}}]}}\n")
	if err := os.WriteFile(filepath.Join(project, id+".jsonl"), data, 0600); err != nil {
		t.Fatal(err)
	}
	store := t.TempDir()
	work := t.TempDir()
	saveSessions(t, store, &subagent.Session{ID: "claude-managed", Name: "collision-1226", Provider: "claude", ProviderSessionID: id, WorkingDir: work})
	collision := "collision-1226"
	f, err := os.CreateTemp(".", collision+"-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer func() { _ = os.Remove(f.Name()) }()
	name := filepath.Base(f.Name())
	saveSessions(t, store, &subagent.Session{ID: "claude-collision", Name: name, Provider: "claude", ProviderSessionID: id, WorkingDir: work})
	for _, args := range [][]string{{"commands", "--name", name}, {"commands", "--name", name, "--dir", work}, {"commands", "claude-managed", "--dir", work}} {
		got, err := runWithStore(t, nil, store, args...)
		if err != nil || got != `one\r\ntwo\nthree`+"\n" {
			t.Fatalf("%v: got %q, err %v", args, got, err)
		}
	}
	for _, args := range [][]string{{"commands", "--name", name, "--dir", t.TempDir()}, {"commands", "claude-managed", "--dir", t.TempDir()}, {"commands", id, "--dir", work}} {
		if _, err := runWithStore(t, nil, store, args...); err == nil {
			t.Fatalf("%v: expected directory error", args)
		}
	}
}
