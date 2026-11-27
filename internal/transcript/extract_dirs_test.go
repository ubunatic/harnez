package transcript

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDirs_DedupeAndMissing(t *testing.T) {
	root := t.TempDir()
	dir1 := filepath.Join(root, "d1")
	dir2 := filepath.Join(root, "d2")
	absent := filepath.Join(root, "absent")

	s1d1 := filepath.Join(dir1, "projects", "-p", "abc-111.jsonl")
	s1d2 := filepath.Join(dir2, "projects", "-p", "abc-111.jsonl")
	s2d2 := filepath.Join(dir2, "projects", "-p", "def-222.jsonl")
	nested := filepath.Join(dir1, "projects", "-p", "abc-111", "subagents", "def-sub.jsonl")

	for _, p := range []string{s1d1, s1d2, s2d2, nested} {
		writeFixture(t, p, "")
	}

	got, err := ResolveDirs([]string{dir1, absent, dir2}, "abc-111")
	if err != nil || got != s1d1 {
		t.Fatalf("ResolveDirs(abc-111) = %q, %v; want %q", got, err, s1d1)
	}
	got, err = ResolveDirs([]string{dir1, absent, dir2}, "def")
	if err != nil || got != s2d2 {
		t.Fatalf("ResolveDirs(def) = %q, %v; want %q", got, err, s2d2)
	}
	if _, err := ResolveDirs([]string{dir1, absent, dir2}, "def-sub"); err == nil {
		t.Fatalf("expected nested subagent transcript to be excluded")
	}
}

func TestResolveCommands_MultiDir(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	cfgDir := filepath.Join(xdg, "harnez")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "local.yaml"),
		[]byte("usage:\n  claude_config_dirs:\n    - ~/.claude\n    - ~/extra-claude\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s1 := filepath.Join(home, ".claude", "projects", "-p", "sess-111.jsonl")
	s1dup := filepath.Join(home, "extra-claude", "projects", "-p", "sess-111.jsonl")
	s2 := filepath.Join(home, "extra-claude", "projects", "-p", "sess-222.jsonl")
	for _, p := range []string{s1, s1dup, s2} {
		writeFixture(t, p, "")
	}

	got, err := ResolveCommands("sess-111", "claude")
	if err != nil || got != s1 {
		t.Fatalf("ResolveCommands(sess-111) = %q, %v; want %q", got, err, s1)
	}
	got, err = ResolveCommands("sess-222", "claude")
	if err != nil || got != s2 {
		t.Fatalf("ResolveCommands(sess-222) = %q, %v; want %q", got, err, s2)
	}
}
