package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModeCmd_Execution(t *testing.T) {
	tmpDir := t.TempDir()
	agentsFile := filepath.Join(tmpDir, "AGENTS.md")

	initial := "# Local Instructions\n"
	if err := os.WriteFile(agentsFile, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newModeCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// 1. Run `mode std`
	cmd.SetArgs([]string{"--file", agentsFile, "std"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mode std failed: %v", err)
	}

	if !strings.Contains(stdout.String(), "Concise Standard (Level 2)") {
		t.Errorf("stdout directive missing Level 2:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "harnez: added \"Concise Mode\" section") {
		t.Errorf("stderr missing update note:\n%s", stderr.String())
	}

	content, _ := os.ReadFile(agentsFile)
	if !strings.Contains(string(content), "<!-- harnez:begin Concise Mode -->") ||
		!strings.Contains(string(content), "Operational Mode: Concise Standard (Level 2)") {
		t.Errorf("AGENTS.md not updated properly:\n%s", string(content))
	}

	// 2. Run with --quiet flag
	stdout.Reset()
	stderr.Reset()
	cmd.SetArgs([]string{"--file", agentsFile, "--quiet", "ultra"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mode ultra --quiet failed: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected no stdout with --quiet, got: %s", stdout.String())
	}
	content, _ = os.ReadFile(agentsFile)
	if !strings.Contains(string(content), "Operational Mode: Concise Ultra (Level 3)") {
		t.Errorf("AGENTS.md not updated to Ultra:\n%s", string(content))
	}

	// 3. Subcommand invocation `mode off`
	stdout.Reset()
	stderr.Reset()
	cmd = newModeCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--file", agentsFile, "off"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mode off failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "ConciseMode OFF") {
		t.Errorf("stdout directive missing OFF notice:\n%s", stdout.String())
	}
	content, _ = os.ReadFile(agentsFile)
	if strings.Contains(string(content), "<!-- harnez:begin Concise Mode -->") {
		t.Errorf("AGENTS.md still contains Concise Mode section after off:\n%s", string(content))
	}

	// 4. Unknown tier error
	stdout.Reset()
	stderr.Reset()
	cmd.SetArgs([]string{"--file", agentsFile, "unknown-tier"})
	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for unknown tier, got nil")
	}
}
