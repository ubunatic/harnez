package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManCommand(t *testing.T) {
	root := newRootCmd()
	dir := t.TempDir()
	root.SetArgs([]string{"man", "--dir", dir})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("execute man --dir: %v", err)
	}
	path := filepath.Join(dir, "harnez-agent-start.1")
	page, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated page: %v", err)
	}
	if !strings.Contains(string(page), "--interactive") {
		t.Errorf("generated page lacks --interactive: %s", path)
	}
}

func TestManCommandPrintsRoff(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"man"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("execute man: %v", err)
	}
	if !strings.Contains(out.String(), ".TH \"HARNEZ\" \"1\"") {
		t.Errorf("expected roff man header, got %q", out.String()[:min(out.Len(), 200)])
	}
}
