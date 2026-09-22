package claude_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/agy"
	"ubunatic.com/harnez/internal/claude"
)

func TestBashShimProvisioningAndLifecycle(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	targetDir := filepath.Join(tmpHome, ".claude")
	shimPath := filepath.Join(tmpHome, ".harnez", "shims", "bash")

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

	// 1. ApplyAll should create ~/.harnez/shims/bash with 0755 mode and exact content
	if err := claude.ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	fi, err := os.Stat(shimPath)
	if err != nil {
		t.Fatalf("expected bash shim at %s: %v", shimPath, err)
	}
	if fi.Mode().Perm() != 0755 {
		t.Errorf("expected mode 0755, got %o", fi.Mode().Perm())
	}
	content, err := os.ReadFile(shimPath)
	if err != nil {
		t.Fatalf("read shim: %v", err)
	}
	if string(content) != claude.BashShimContent {
		t.Errorf("shim content mismatch: got:\n%s\nwant:\n%s", string(content), claude.BashShimContent)
	}

	// 2. DiffAll should report no changes when aligned
	changed, err := claude.DiffAll(targetDir, cfg, claude.FullComponentSelection{})
	if err != nil {
		t.Fatalf("DiffAll failed: %v", err)
	}
	if changed {
		t.Errorf("expected DiffAll to report false when shim is present and aligned")
	}

	// 3. Remove shim -> DiffAll should report drift
	if err := os.Remove(shimPath); err != nil {
		t.Fatalf("remove shim: %v", err)
	}
	changed, err = claude.DiffAll(targetDir, cfg, claude.FullComponentSelection{})
	if err != nil {
		t.Fatalf("DiffAll failed: %v", err)
	}
	if !changed {
		t.Errorf("expected DiffAll to report true when shim is missing")
	}

	// 4. Modify content -> DiffAll should report drift
	if err := os.WriteFile(shimPath, []byte("#!/bin/sh\necho wrong\n"), 0755); err != nil {
		t.Fatalf("write modified shim: %v", err)
	}
	changed, err = claude.DiffAll(targetDir, cfg, claude.FullComponentSelection{})
	if err != nil {
		t.Fatalf("DiffAll failed: %v", err)
	}
	if !changed {
		t.Errorf("expected DiffAll to report true when shim content is modified")
	}

	// 5. Modify permissions -> DiffAll should report drift
	if err := os.Chmod(shimPath, 0644); err != nil {
		t.Fatalf("write non-executable shim: %v", err)
	}
	changed, err = claude.DiffAll(targetDir, cfg, claude.FullComponentSelection{})
	if err != nil {
		t.Fatalf("DiffAll failed: %v", err)
	}
	if !changed {
		t.Errorf("expected DiffAll to report true when shim mode is not 0755")
	}

	// 6. Re-apply to repair drift
	if err := claude.ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll repair failed: %v", err)
	}
	fi, err = os.Stat(shimPath)
	if err != nil {
		t.Fatalf("expected shim restored: %v", err)
	}
	if fi.Mode().Perm() != 0755 {
		t.Errorf("expected repaired mode 0755, got %o", fi.Mode().Perm())
	}
	content, _ = os.ReadFile(shimPath)
	if string(content) != claude.BashShimContent {
		t.Errorf("expected repaired content, got:\n%s", string(content))
	}

	// 7. CleanAll should remove ~/.harnez/shims/bash
	if err := claude.CleanAll(targetDir, cfg); err != nil {
		t.Fatalf("CleanAll failed: %v", err)
	}
	if _, err := os.Stat(shimPath); !os.IsNotExist(err) {
		t.Errorf("expected bash shim to be removed by CleanAll, but err = %v", err)
	}
}

func TestApplyInstallsAgyObservationHooks(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	targetDir := filepath.Join(tmpHome, ".claude")
	agyHooksPath := filepath.Join(tmpHome, ".gemini", "config", "hooks.json")

	// Seed .gemini environment with existing custom hook
	if err := os.MkdirAll(filepath.Dir(agyHooksPath), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	initialJSON := `{"custom": {"enabled": true}}`
	if err := os.WriteFile(agyHooksPath, []byte(initialJSON), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	cfg.AgyHooksTarget = agyHooksPath
	cfg.SkillsTarget = filepath.Join(tmpHome, ".gemini", "skills")
	cfg.CodexSkillsTarget = filepath.Join(tmpHome, ".codex", "skills")
	cfg.CodexHooksTarget = filepath.Join(tmpHome, ".codex", "config.toml")
	cfg.ClaudeSkillsTarget = filepath.Join(tmpHome, ".claude", "skills")
	cfg.PrimeAgentTarget = filepath.Join(tmpHome, ".prime", "agent")
	cfg.AgentsMD.Global.Target = filepath.Join(tmpHome, ".claude", "CLAUDE.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil

	// DiffAll should detect that hooks.json is missing the harnez observer hook
	changed, err := claude.DiffAll(targetDir, cfg, claude.FullComponentSelection{})
	if err != nil {
		t.Fatalf("DiffAll: %v", err)
	}
	if !changed {
		t.Errorf("expected DiffAll to detect missing harnez hook in hooks.json")
	}

	// ApplyAll should write harnez observer hook to hooks.json while preserving "custom"
	if err := claude.ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll: %v", err)
	}

	installed, drifted := agy.Status(agyHooksPath)
	if !installed || drifted {
		t.Errorf("expected harnez hook installed and not drifted in hooks.json after ApplyAll, got installed=%v drifted=%v", installed, drifted)
	}
	data, err := os.ReadFile(agyHooksPath)
	if err != nil {
		t.Fatalf("ReadFile hooks.json: %v", err)
	}
	if !strings.Contains(string(data), "custom") {
		t.Errorf("expected custom hook preserved, got:\n%s", string(data))
	}
	if !strings.Contains(string(data), "harnez hook agy") {
		t.Errorf("expected harnez hook agy in hooks.json, got:\n%s", string(data))
	}
}

func TestBashShimExecutionAndRecursionGuard(t *testing.T) {
	tmpDir := t.TempDir()
	shimPath := filepath.Join(tmpDir, "bash")

	if err := os.WriteFile(shimPath, []byte(claude.BashShimContent), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// 1. With HARNEZ_INTERCEPTED=1, the shim directly execs /bin/bash "$@"
	cmd := exec.Command(shimPath, "-c", "echo inside-subshell")
	cmd.Env = append(os.Environ(), "HARNEZ_INTERCEPTED=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exec shim with HARNEZ_INTERCEPTED=1 failed: %v (out=%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "inside-subshell" {
		t.Errorf("expected 'inside-subshell', got %q", string(out))
	}
}
