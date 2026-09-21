package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func TestAgentDefaultModelIsMarkedOnce(t *testing.T) {
	var out strings.Builder
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"models"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "(default)") != 1 {
		t.Fatalf("models output=%q", out.String())
	}
}

func TestAgentDefaultLiteralIsNotShadowed(t *testing.T) {
	root := filepath.Join("..", "..")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(data), "codex:luna:low") {
			t.Errorf("default literal shadowed in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAgentStartDefaultModelLine(t *testing.T) {
	d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "thread"}}, {ev: msg("CONFIRM: ready")}, {ev: msg("done")}}}
	out := runScripted(t, d, "start", "task")
	if !strings.Contains(out, "model: codex:luna:low (default)") {
		t.Fatalf("output=%q", out)
	}
	d = &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "thread"}}, {ev: msg("CONFIRM: ready")}, {ev: msg("done")}}}
	out = runScripted(t, d, "start", "--model", "codex:luna", "task")
	if strings.Contains(out, "(default)") {
		t.Fatalf("explicit output=%q", out)
	}
}
