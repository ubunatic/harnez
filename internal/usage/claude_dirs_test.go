package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestClaudeConfigDirs_DefaultAndOverride(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	// Default from spec/usage.yaml with ~ expanded.
	dirs, err := ClaudeConfigDirs()
	if err != nil {
		t.Fatalf("default ClaudeConfigDirs: %v", err)
	}
	wantDefault := []string{filepath.Join(home, ".claude")}
	if !reflect.DeepEqual(dirs, wantDefault) {
		t.Fatalf("default dirs = %v, want %v", dirs, wantDefault)
	}

	// Override in local.yaml replaces spec list and expands ~.
	cfgDir := filepath.Join(xdg, "harnez")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "local.yaml")
	if err := os.WriteFile(cfgPath, []byte("usage:\n  claude_config_dirs:\n    - ~/.claude\n    - ~/extra-claude\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dirs, err = ClaudeConfigDirs()
	if err != nil {
		t.Fatalf("override ClaudeConfigDirs: %v", err)
	}
	wantOverride := []string{filepath.Join(home, ".claude"), filepath.Join(home, "extra-claude")}
	if !reflect.DeepEqual(dirs, wantOverride) {
		t.Fatalf("override dirs = %v, want %v", dirs, wantOverride)
	}

	// Empty list in local.yaml falls back to spec default.
	if err := os.WriteFile(cfgPath, []byte("usage:\n  claude_config_dirs: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dirs, err = ClaudeConfigDirs()
	if err != nil {
		t.Fatalf("empty override ClaudeConfigDirs: %v", err)
	}
	if !reflect.DeepEqual(dirs, wantDefault) {
		t.Fatalf("empty override dirs = %v, want %v", dirs, wantDefault)
	}
}

func TestClaudeTranscriptFilesMulti_DedupeAndMissing(t *testing.T) {
	root := t.TempDir()
	dir1 := filepath.Join(root, "d1")
	dir2 := filepath.Join(root, "d2")
	absent := filepath.Join(root, "absent")

	s1d1 := filepath.Join(dir1, "projects", "-p", "s1.jsonl")
	s1d2 := filepath.Join(dir2, "projects", "-p", "s1.jsonl")
	s2d2 := filepath.Join(dir2, "projects", "-p", "s2.jsonl")
	nested := filepath.Join(dir1, "projects", "-p", "s1", "subagents", "agent-a.jsonl")

	for _, p := range []string{s1d1, s1d2, s2d2, nested} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := ClaudeTranscriptFilesMulti([]string{dir1, absent, dir2})
	if err != nil {
		t.Fatalf("ClaudeTranscriptFilesMulti: %v", err)
	}
	want := []string{s1d1, s2d2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
}

func TestCollectProjectUsage_MultiDir(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	repoPath := filepath.Join(home, "myrepo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}

	cfgDir := filepath.Join(xdg, "harnez")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "local.yaml"),
		[]byte("usage:\n  claude_config_dirs:\n    - ~/.claude\n    - ~/extra-claude\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	writeSession := func(dir, name string, input int) {
		p := filepath.Join(dir, "projects", "-myrepo", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		line, _ := json.Marshal(map[string]any{
			"cwd": repoPath,
			"message": map[string]any{
				"model": "claude-sonnet-5",
				"usage": map[string]any{"input_tokens": input, "output_tokens": 0},
			},
		})
		if err := os.WriteFile(p, append(line, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	writeSession(filepath.Join(home, ".claude"), "s1.jsonl", 100)
	writeSession(filepath.Join(home, "extra-claude"), "s1.jsonl", 999) // deduped
	writeSession(filepath.Join(home, "extra-claude"), "s2.jsonl", 50)

	res, err := CollectProjectUsage(repoPath)
	if err != nil {
		t.Fatalf("CollectProjectUsage: %v", err)
	}
	cl := res.Agents["claude"]
	if cl == nil || cl.MatchedSessions != 2 || cl.Tokens.InputTokens != 150 {
		t.Fatalf("claude attribution = %+v, want 2 sessions and 150 input tokens", cl)
	}
}
