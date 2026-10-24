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
	fixture := "" +
		`{"type":"user","timestamp":"2026-10-20T18:00:00.000Z","uuid":"plain","message":{"content":"/lean-sprint 097"}}` + "\n" +
		`{"type":"user","timestamp":"2026-10-20T18:00:00.500Z","uuid":"structured","message":{"content":"<command-name>/lean-sprint</command-name><command-args>097</command-args>"}}` + "\n" +
		`{"type":"user","timestamp":"2026-10-20T18:00:02.000Z","uuid":"later","message":{"content":"/issue 878"}}` + "\n"
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
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
		if args[len(args)-1] == "--json" {
			var result struct {
				Records []struct {
					Text string `json:"text"`
				} `json:"records"`
			}
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			slashCount := 0
			for _, record := range result.Records {
				if strings.HasPrefix(record.Text, "/") {
					slashCount++
				}
			}
			if slashCount != 2 {
				t.Fatalf("got %d slash messages, want 2: %+v", slashCount, result.Records)
			}
		} else if count := strings.Count(output.String(), "/lean-sprint 097") + strings.Count(output.String(), "/issue 878"); count != 2 {
			t.Fatalf("got %d slash messages, want 2: %s", count, output.String())
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
