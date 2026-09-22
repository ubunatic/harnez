// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"os"
	"path/filepath"
	"testing"
)

// TestApplyAll_DoesNotManageGlobalRootInstructionDocs verifies acceptance criteria
// of Issue 415: harnez apply does not install, overwrite, or manage global root instruction
// files (e.g. ~/.claude/CLAUDE.md, ~/.prime/agent/AGENTS.md, ~/.codex/AGENTS.md, ~/AGENTS.md).
func TestApplyAll_DoesNotManageGlobalRootInstructionDocs(t *testing.T) {
	targetDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	claudeMDPath := filepath.Join(homeDir, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(claudeMDPath), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	userClaudeContent := "# User Custom Global Rules\n\n- Do not overwrite me.\n"
	if err := os.WriteFile(claudeMDPath, []byte(userClaudeContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	primeDir := filepath.Join(homeDir, ".prime", "agent")
	primeAGENTSPath := filepath.Join(primeDir, "AGENTS.md")
	if err := os.MkdirAll(primeDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	userPrimeContent := "# User Prime Rules\n"
	if err := os.WriteFile(primeAGENTSPath, []byte(userPrimeContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	codexDir := filepath.Join(homeDir, ".codex")
	codexAGENTSPath := filepath.Join(codexDir, "AGENTS.md")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	userCodexContent := "# User Codex Rules\n"
	if err := os.WriteFile(codexAGENTSPath, []byte(userCodexContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	cfg.PrimeAgentTarget = primeDir
	cfg.SkillsTarget = filepath.Join(homeDir, ".gemini", "skills")
	cfg.CodexSkillsTarget = filepath.Join(homeDir, ".codex", "skills")
	cfg.ClaudeSkillsTarget = filepath.Join(homeDir, ".claude", "skills")

	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	// Verify all existing user global root files are 100% untouched
	for path, wantContent := range map[string]string{
		claudeMDPath:    userClaudeContent,
		primeAGENTSPath: userPrimeContent,
		codexAGENTSPath: userCodexContent,
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", path, err)
		}
		if string(got) != wantContent {
			t.Errorf("expected %s to remain untouched:\nwant:\n%s\ngot:\n%s", path, wantContent, string(got))
		}
	}

	// Verify ~/AGENTS.md symlink was NOT created
	homeAGENTSPath := filepath.Join(homeDir, "AGENTS.md")
	if _, err := os.Lstat(homeAGENTSPath); err == nil {
		t.Errorf("expected ~/AGENTS.md to not be created, but it exists")
	}

	// Verify DiffAll does not report drift on applied configuration
	changed, err := DiffAll(targetDir, cfg, FullComponentSelection{})
	if err != nil {
		t.Fatalf("DiffAll failed: %v", err)
	}
	if changed {
		t.Errorf("expected DiffAll not to report drift")
	}

	// Verify CleanAll does not delete or alter global root docs
	if err := CleanAll(targetDir, cfg); err != nil {
		t.Fatalf("CleanAll failed: %v", err)
	}
	for path, wantContent := range map[string]string{
		claudeMDPath:    userClaudeContent,
		primeAGENTSPath: userPrimeContent,
		codexAGENTSPath: userCodexContent,
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile %s after CleanAll: %v", path, err)
		}
		if string(got) != wantContent {
			t.Errorf("expected %s to remain untouched after CleanAll:\nwant:\n%s\ngot:\n%s", path, wantContent, string(got))
		}
	}
}

// TestInit_RefusesGlobalAgentRoots verifies that harnez init refuses to operate
// on global agent roots like ~/.claude, ~/.prime, ~/.codex, ~/.gemini, and $HOME.
func TestInit_RefusesGlobalAgentRoots(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	globalDirs := []string{
		homeDir,
		filepath.Join(homeDir, ".claude"),
		filepath.Join(homeDir, ".prime"),
		filepath.Join(homeDir, ".prime", "agent"),
		filepath.Join(homeDir, ".codex"),
		filepath.Join(homeDir, ".gemini"),
	}

	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	for _, gDir := range globalDirs {
		if err := os.MkdirAll(gDir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		// Must fail validation with or without force
		if err := ValidateInitTarget(gDir, false); err == nil {
			t.Errorf("expected ValidateInitTarget(%s, force=false) to fail", gDir)
		}
		if err := ValidateInitTarget(gDir, true); err == nil {
			t.Errorf("expected ValidateInitTarget(%s, force=true) to fail", gDir)
		}
		if err := RunInit(gDir, cfg, nil, "", true, false, false, false); err == nil {
			t.Errorf("expected RunInit(%s) to fail", gDir)
		}
		// Confirm AGENTS.md / CLAUDE.md was NOT created
		if _, err := os.Stat(filepath.Join(gDir, "AGENTS.md")); err == nil {
			t.Errorf("expected AGENTS.md not to be created in %s", gDir)
		}
	}
}
