package claude_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/markdown"
)

func TestHarnezEnvScriptAndShellIntegration(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	targetDir := filepath.Join(tmpHome, ".claude")
	envPath := filepath.Join(tmpHome, ".harnez", "env.sh")
	bashrcPath := filepath.Join(tmpHome, ".bashrc")
	zshrcPath := filepath.Join(tmpHome, ".zshrc")

	// Pre-create ~/.bashrc and ~/.zshrc with user content
	if err := os.WriteFile(bashrcPath, []byte("# user bashrc\nalias ll='ls -la'\n"), 0644); err != nil {
		t.Fatalf("write bashrc: %v", err)
	}
	if err := os.WriteFile(zshrcPath, []byte("# user zshrc\n"), 0644); err != nil {
		t.Fatalf("write zshrc: %v", err)
	}

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	cfg.SkillsTarget = filepath.Join(tmpHome, ".gemini", "skills")
	cfg.CodexSkillsTarget = filepath.Join(tmpHome, ".codex", "skills")
	cfg.CodexHooksTarget = filepath.Join(tmpHome, ".codex", "config.toml")
	cfg.AgyHooksTarget = filepath.Join(tmpHome, ".gemini", "config", "hooks.json")
	cfg.ClaudeSkillsTarget = filepath.Join(tmpHome, ".claude", "skills")
	cfg.PrimeAgentTarget = filepath.Join(tmpHome, ".prime", "agent")
	cfg.AgentsMD.Global.Target = filepath.Join(tmpHome, ".claude", "CLAUDE.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil
	cfg.DistillAutopipe.PiExtensionTarget = filepath.Join(tmpHome, ".pi", "harnez-distill.ts")
	cfg.DistillAutopipe.OpenCodePluginTarget = filepath.Join(tmpHome, ".config", "opencode", "harnez-distill.ts")

	// 1. Plain ApplyAll (without installShell) should create ~/.harnez/env.sh but not modify .bashrc / .zshrc
	if err := claude.ApplyAll(targetDir, cfg, nil, false, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	envData, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env.sh: %v", err)
	}
	if string(envData) != claude.HarnezEnvContent {
		t.Errorf("env.sh content mismatch: got:\n%s\nwant:\n%s", string(envData), claude.HarnezEnvContent)
	}
	if !strings.Contains(string(envData), "agy()") {
		t.Errorf("expected agy() function in env.sh")
	}

	if markdown.ContainsSectionMK(bashrcPath, "env") {
		t.Errorf("expected ~/.bashrc NOT modified by default plain apply")
	}
	if markdown.ContainsSectionMK(zshrcPath, "env") {
		t.Errorf("expected ~/.zshrc NOT modified by default plain apply")
	}

	// 2. ApplyAll with installShell=true should inject the managed block into both rc files
	if err := claude.ApplyAll(targetDir, cfg, nil, false, false, true); err != nil {
		t.Fatalf("ApplyAll with shell failed: %v", err)
	}

	if !markdown.ContainsSectionMK(bashrcPath, "env") {
		t.Errorf("expected ~/.bashrc to contain managed env section after apply --shell")
	}
	if !markdown.ContainsSectionMK(zshrcPath, "env") {
		t.Errorf("expected ~/.zshrc to contain managed env section after apply --shell")
	}

	bashrcData, _ := os.ReadFile(bashrcPath)
	if !strings.Contains(string(bashrcData), "alias ll='ls -la'") {
		t.Errorf("user bashrc content was destroyed")
	}

	// 3. DiffAll should report no changes when aligned
	changed, err := claude.DiffAll(targetDir, cfg, nil)
	if err != nil {
		t.Fatalf("DiffAll failed: %v", err)
	}
	if changed {
		t.Errorf("expected DiffAll=false when fully aligned, got true")
	}

	// 4. Drift detection: corrupt env.sh
	if err := os.WriteFile(envPath, []byte("# corrupted\n"), 0644); err != nil {
		t.Fatalf("write corrupted env.sh: %v", err)
	}
	changed, err = claude.DiffAll(targetDir, cfg, nil)
	if err != nil {
		t.Fatalf("DiffAll on drift: %v", err)
	}
	if !changed {
		t.Errorf("expected DiffAll=true after corrupting env.sh")
	}

	// 5. Repair via ApplyAll
	if err := claude.ApplyAll(targetDir, cfg, nil, false, false, true); err != nil {
		t.Fatalf("ApplyAll repair failed: %v", err)
	}
	envData, _ = os.ReadFile(envPath)
	if string(envData) != claude.HarnezEnvContent {
		t.Errorf("env.sh was not repaired")
	}

	// 6. CleanAll should remove env.sh and clean .bashrc / .zshrc
	if err := claude.CleanAll(targetDir, cfg); err != nil {
		t.Fatalf("CleanAll failed: %v", err)
	}

	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Errorf("expected env.sh removed after CleanAll")
	}
	if markdown.ContainsSectionMK(bashrcPath, "env") {
		t.Errorf("expected ~/.bashrc cleaned after CleanAll")
	}
	if markdown.ContainsSectionMK(zshrcPath, "env") {
		t.Errorf("expected ~/.zshrc cleaned after CleanAll")
	}
	cleanedBashrc, _ := os.ReadFile(bashrcPath)
	if !strings.Contains(string(cleanedBashrc), "alias ll='ls -la'") {
		t.Errorf("user bashrc content was lost after CleanAll")
	}
}
