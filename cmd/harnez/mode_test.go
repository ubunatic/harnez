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

	// 1. Run `mode std --file ...`
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

	// 2b. Run with `vision`
	stdout.Reset()
	stderr.Reset()
	cmd = newModeCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--file", agentsFile, "vision"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mode vision failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "Vision Mode") {
		t.Errorf("expected Vision Mode directive, got: %s", stdout.String())
	}
	content, _ = os.ReadFile(agentsFile)
	if !strings.Contains(string(content), "Operational Mode: Vision Mode") {
		t.Errorf("AGENTS.md not updated to Vision Mode:\n%s", string(content))
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

func TestModeCmd_FlagsAndDefaults(t *testing.T) {
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(origWd)
	}()

	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. Default `mode std` writes to AGENTS.local.md and adds to git exclude
	cmd := newModeCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"std"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mode std failed: %v", err)
	}

	localFile := filepath.Join(tmpDir, "AGENTS.local.md")
	content, err := os.ReadFile(localFile)
	if err != nil {
		t.Fatalf("Failed to read AGENTS.local.md: %v", err)
	}
	if !strings.Contains(string(content), "Concise Standard (Level 2)") {
		t.Errorf("AGENTS.local.md missing Standard content: %s", string(content))
	}

	excludeContent, err := os.ReadFile(filepath.Join(gitDir, "info", "exclude"))
	if err != nil {
		t.Fatalf("Failed to read .git/info/exclude: %v", err)
	}
	if !strings.Contains(string(excludeContent), "AGENTS.local.md") {
		t.Errorf("git exclude missing AGENTS.local.md: %s", string(excludeContent))
	}

	// 2. Ephemeral flag `--ephemeral`
	stdout.Reset()
	stderr.Reset()
	cmd = newModeCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"-e", "ultra"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mode -e ultra failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "Concise Ultra (Level 3)") {
		t.Errorf("Ephemeral missing ultra directive: %s", stdout.String())
	}
	// localFile should NOT have been updated to ultra
	content, _ = os.ReadFile(localFile)
	if strings.Contains(string(content), "Concise Ultra") {
		t.Errorf("Ephemeral should not have modified AGENTS.local.md")
	}

	// 3. Persist flag `--persist`
	stdout.Reset()
	stderr.Reset()
	cmd = newModeCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--persist", "lite"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mode --persist lite failed: %v", err)
	}
	mainFile := filepath.Join(tmpDir, "AGENTS.md")
	content, err = os.ReadFile(mainFile)
	if err != nil {
		t.Fatalf("Failed to read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(content), "Concise Lite (Level 1)") {
		t.Errorf("AGENTS.md missing Lite content: %s", string(content))
	}

	// 4. `mode off` deletes AGENTS.local.md when empty
	stdout.Reset()
	stderr.Reset()
	cmd = newModeCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"off"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mode off failed: %v", err)
	}
	if _, err := os.Stat(localFile); !os.IsNotExist(err) {
		t.Errorf("AGENTS.local.md should have been deleted on mode off")
	}
}

