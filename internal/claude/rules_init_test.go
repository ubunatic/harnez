// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/claude"
)

func TestRunInit_GeneratesHarnezRules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatal(err)
	}
	agents, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(agents), cfg.AgentsMD.Rules.Header+"\n\n") {
		t.Fatalf("AGENTS.md missing rules header:\n%s", agents)
	}
	for _, name := range []string{"Index.md", "Tools.md", "Issues.md", "Subagents.md", "Output.md"} {
		if _, err := os.Stat(filepath.Join(dir, ".harnez", "rules", name)); err != nil {
			t.Errorf("missing generated rule %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".harnez", "rules", "Quota.md")); !os.IsNotExist(err) {
		t.Errorf("Quota.md exists without Quota-1 opt-in: %v", err)
	}
	exclude, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exclude), ".harnez/rules/Local.md\n") {
		t.Fatalf("Local.md missing from git exclude:\n%s", exclude)
	}
	first := string(agents)
	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != first {
		t.Fatal("second init changed AGENTS.md")
	}
}

func TestRunInit_GeneratesRulesOutsideGit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/nongit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".harnez", "rules", "Index.md")); err != nil {
		t.Fatalf("missing rules outside git: %v", err)
	}
}
