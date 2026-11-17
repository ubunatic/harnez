package claude_test

import (
	"os"
	"os/exec"
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
	envFishPath := filepath.Join(tmpHome, ".harnez", "env.fish")
	bashrcPath := filepath.Join(tmpHome, ".bashrc")
	zshrcPath := filepath.Join(tmpHome, ".zshrc")
	fishrcPath := filepath.Join(tmpHome, ".config", "fish", "config.fish")

	// Pre-create ~/.bashrc, ~/.zshrc, and ~/.config/fish/config.fish with user content
	if err := os.WriteFile(bashrcPath, []byte("# user bashrc\nalias ll='ls -la'\n"), 0644); err != nil {
		t.Fatalf("write bashrc: %v", err)
	}
	if err := os.WriteFile(zshrcPath, []byte("# user zshrc\n"), 0644); err != nil {
		t.Fatalf("write zshrc: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(fishrcPath), 0755); err != nil {
		t.Fatalf("mkdir fish dir: %v", err)
	}
	if err := os.WriteFile(fishrcPath, []byte("# user fish config\nalias ll 'ls -la'\n"), 0644); err != nil {
		t.Fatalf("write config.fish: %v", err)
	}

	cfg := claude.LoadTestConfig(t)
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
	if !strings.Contains(string(envData), "alias hagy='harnez-agy'") {
		t.Errorf("expected hagy alias for harnez-agy in env.sh")
	}

	envFishData, err := os.ReadFile(envFishPath)
	if err != nil {
		t.Fatalf("read env.fish: %v", err)
	}
	if string(envFishData) != claude.HarnezEnvFishContent {
		t.Errorf("env.fish content mismatch: got:\n%s\nwant:\n%s", string(envFishData), claude.HarnezEnvFishContent)
	}
	if !strings.Contains(string(envFishData), "function agy") {
		t.Errorf("expected agy function in env.fish")
	}
	if !strings.Contains(string(envFishData), "alias hagy='harnez-agy'") {
		t.Errorf("expected hagy alias for harnez-agy in env.fish")
	}

	if markdown.ContainsSectionMK(bashrcPath, "env") {
		t.Errorf("expected ~/.bashrc NOT modified by default plain apply")
	}
	if markdown.ContainsSectionMK(zshrcPath, "env") {
		t.Errorf("expected ~/.zshrc NOT modified by default plain apply")
	}
	if markdown.ContainsSectionMK(fishrcPath, "env") {
		t.Errorf("expected ~/.config/fish/config.fish NOT modified by default plain apply")
	}

	// 2. ApplyAll with installShell=true should inject the managed block into all rc files
	if err := claude.ApplyAll(targetDir, cfg, nil, false, false, true); err != nil {
		t.Fatalf("ApplyAll with shell failed: %v", err)
	}

	if !markdown.ContainsSectionMK(bashrcPath, "env") {
		t.Errorf("expected ~/.bashrc to contain managed env section after apply --shell")
	}
	if !markdown.ContainsSectionMK(zshrcPath, "env") {
		t.Errorf("expected ~/.zshrc to contain managed env section after apply --shell")
	}
	if !markdown.ContainsSectionMK(fishrcPath, "env") {
		t.Errorf("expected ~/.config/fish/config.fish to contain managed env section after apply --shell")
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

	// 4. Drift detection: corrupt env.sh and env.fish
	if err := os.WriteFile(envPath, []byte("# corrupted\n"), 0644); err != nil {
		t.Fatalf("write corrupted env.sh: %v", err)
	}
	if err := os.WriteFile(envFishPath, []byte("# corrupted\n"), 0644); err != nil {
		t.Fatalf("write corrupted env.fish: %v", err)
	}
	changed, err = claude.DiffAll(targetDir, cfg, nil)
	if err != nil {
		t.Fatalf("DiffAll on drift: %v", err)
	}
	if !changed {
		t.Errorf("expected DiffAll=true after corrupting env.sh/env.fish")
	}

	// 5. Repair via ApplyAll
	if err := claude.ApplyAll(targetDir, cfg, nil, false, false, true); err != nil {
		t.Fatalf("ApplyAll repair failed: %v", err)
	}
	envData, _ = os.ReadFile(envPath)
	if string(envData) != claude.HarnezEnvContent {
		t.Errorf("env.sh was not repaired")
	}
	envFishData, _ = os.ReadFile(envFishPath)
	if string(envFishData) != claude.HarnezEnvFishContent {
		t.Errorf("env.fish was not repaired")
	}

	// 6. CleanAll should remove env.sh/env.fish and clean .bashrc / .zshrc / config.fish
	if err := claude.CleanAll(targetDir, cfg); err != nil {
		t.Fatalf("CleanAll failed: %v", err)
	}

	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Errorf("expected env.sh removed after CleanAll")
	}
	if _, err := os.Stat(envFishPath); !os.IsNotExist(err) {
		t.Errorf("expected env.fish removed after CleanAll")
	}
	if markdown.ContainsSectionMK(bashrcPath, "env") {
		t.Errorf("expected ~/.bashrc cleaned after CleanAll")
	}
	if markdown.ContainsSectionMK(zshrcPath, "env") {
		t.Errorf("expected ~/.zshrc cleaned after CleanAll")
	}
	if markdown.ContainsSectionMK(fishrcPath, "env") {
		t.Errorf("expected ~/.config/fish/config.fish cleaned after CleanAll")
	}
	cleanedBashrc, _ := os.ReadFile(bashrcPath)
	if !strings.Contains(string(cleanedBashrc), "alias ll='ls -la'") {
		t.Errorf("user bashrc content was lost after CleanAll")
	}
	cleanedFishrc, _ := os.ReadFile(fishrcPath)
	if !strings.Contains(string(cleanedFishrc), "alias ll 'ls -la'") {
		t.Errorf("user config.fish content was lost after CleanAll")
	}
}

func TestHarnezEnvHagyAlias(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	targetDir := filepath.Join(tmpHome, ".claude")
	cfg := claude.LoadTestConfig(t)
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

	if err := claude.ApplyAll(targetDir, cfg, nil, false, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	fakeBin := filepath.Join(tmpHome, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	mockLauncher := filepath.Join(fakeBin, "harnez-agy")
	if err := os.WriteFile(mockLauncher, []byte("#!/bin/sh\nprintf 'called:%s\\n' \"$*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	envPath := filepath.Join(tmpHome, ".harnez", "env.sh")
	cmd := exec.Command("/bin/bash", "-c", "shopt -s expand_aliases\nsource \"$1\"\nhagy --meter", "bash", envPath)
	cmd.Env = []string{"HOME=" + tmpHome, "PATH=" + fakeBin + ":/usr/bin:/bin"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash source env.sh && hagy: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "called:--meter" {
		t.Errorf("hagy output = %q, want %q", strings.TrimSpace(string(out)), "called:--meter")
	}

	if fishBin, lookErr := exec.LookPath("fish"); lookErr == nil {
		envFishPath := filepath.Join(tmpHome, ".harnez", "env.fish")
		fishCmd := exec.Command(fishBin, "-c", "source \"$argv[1]\"; hagy --meter", envFishPath)
		fishCmd.Env = []string{"HOME=" + tmpHome, "PATH=" + fakeBin + ":/usr/bin:/bin"}
		fishOut, fishErr := fishCmd.CombinedOutput()
		if fishErr != nil {
			t.Fatalf("fish source env.fish && hagy: %v\n%s", fishErr, fishOut)
		}
		if strings.TrimSpace(string(fishOut)) != "called:--meter" {
			t.Errorf("fish hagy output = %q, want %q", strings.TrimSpace(string(fishOut)), "called:--meter")
		}
	}
}
