package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCmd_RefusesNonCodingDirectoryWithoutForce(t *testing.T) {
	dir := t.TempDir()

	cmd := newInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"-d", dir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected init to fail on empty directory without --force, got nil error")
	}
	if !strings.Contains(err.Error(), "not a Git repository and contains no recognized project or source files") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestInitCmd_AllowsNonCodingDirectoryWithForce(t *testing.T) {
	dir := t.TempDir()

	cmd := newInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"-d", dir, "--force"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected init --force to succeed, got: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Errorf("expected AGENTS.md to be created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Errorf("expected CLAUDE.md to be created: %v", err)
	}
}

func TestInitCmd_Quota1Flag(t *testing.T) {
	cmd := newInitCmd()
	f := cmd.Flags().Lookup("quota-1")
	if f == nil {
		t.Fatal("expected --quota-1 flag to be registered on init command")
	}
	if f.DefValue != "false" {
		t.Errorf("flag default = %q, want false", f.DefValue)
	}
}
