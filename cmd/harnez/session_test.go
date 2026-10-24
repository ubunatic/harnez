package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionExtractCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".claude", "projects", "slug", "test-session.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"type":"user","message":{"content":"/lean-sprint 097"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"extract", path}, {"extract", "test-session", "--json"}} {
		cmd := newSessionCmd()
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), "/lean-sprint 097") {
			t.Fatalf("lost slash command: %s", output.String())
		}
		if args[len(args)-1] == "--json" && !json.Valid(output.Bytes()) {
			t.Fatalf("invalid JSON: %s", output.String())
		}
	}
	for _, args := range [][]string{{"extract"}, {"extract", "a", "b"}, {"extract", "missing"}} {
		cmd := newSessionCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("expected argument/session error for %v", args)
		}
	}
}
