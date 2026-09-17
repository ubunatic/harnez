// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyAll_AgentProfile_DoesNotWriteToAgentRoots(t *testing.T) {
	dir := t.TempDir()
	codexDir := filepath.Join(dir, "codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatalf("MkdirAll codex dir: %v", err)
	}
	codexTarget := filepath.Join(codexDir, "AGENTS.md")
	userCodexRules := "# Custom Codex Rules\n"
	if err := os.WriteFile(codexTarget, []byte(userCodexRules), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &Config{
		AgentsMD: AgentsMD{
			Agents: map[string]AgentsMDTarget{
				"codex": {
					Target: codexTarget,
					Sections: []MDSection{
						{Name: "Background Job Waiting", Content: "codex-only async wait guidance"},
					},
				},
			},
		},
	}

	target := t.TempDir()
	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	data, err := os.ReadFile(codexTarget)
	if err != nil {
		t.Fatalf("expected codex target to still exist: %v", err)
	}
	if string(data) != userCodexRules {
		t.Errorf("expected codex target to remain untouched, got:\n%s", data)
	}
}

func TestCleanAll_DoesNotCleanAgentRoots(t *testing.T) {
	dir := t.TempDir()
	codexDir := filepath.Join(dir, "codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatalf("MkdirAll codex dir: %v", err)
	}
	codexTarget := filepath.Join(codexDir, "AGENTS.md")
	userCodexRules := "# Custom Codex Rules\n"
	if err := os.WriteFile(codexTarget, []byte(userCodexRules), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &Config{
		AgentsMD: AgentsMD{
			Agents: map[string]AgentsMDTarget{
				"codex": {
					Target: codexTarget,
					Sections: []MDSection{
						{Name: "Background Job Waiting", Content: "codex-only async wait guidance"},
					},
				},
			},
		},
	}

	target := t.TempDir()
	if err := CleanAll(target, cfg); err != nil {
		t.Fatalf("CleanAll failed: %v", err)
	}

	data, err := os.ReadFile(codexTarget)
	if err != nil {
		t.Fatalf("expected codex target to still exist after clean: %v", err)
	}
	if string(data) != userCodexRules {
		t.Errorf("expected codex target to remain untouched after clean, got:\n%s", data)
	}
}
