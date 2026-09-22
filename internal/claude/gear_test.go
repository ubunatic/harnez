package claude_test

import (
	"os"
	"path/filepath"
	"testing"

	"ubunatic.com/harnez/internal/claude"
)

func TestGearSymlinkProvisioning(t *testing.T) {
	targetDir := t.TempDir()
	gearLink := filepath.Join(targetDir, "bin", "⚙")

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	cfg.SkillsTarget = filepath.Join(t.TempDir(), "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(t.TempDir(), "codex-skills")
	cfg.CodexHooksTarget = filepath.Join(t.TempDir(), "codex-config.toml")
	cfg.AgyHooksTarget = filepath.Join(t.TempDir(), "gemini", "config", "hooks.json")
	cfg.ClaudeSkillsTarget = filepath.Join(t.TempDir(), "claude-skills")
	cfg.PrimeAgentTarget = filepath.Join(t.TempDir(), "prime-agent")
	cfg.AgentsMD.Global.Target = filepath.Join(t.TempDir(), "CLAUDE.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil
	cfg.DistillAutopipe.PiExtensionTarget = filepath.Join(t.TempDir(), "pi", "harnez-distill.ts")
	cfg.DistillAutopipe.OpenCodePluginTarget = filepath.Join(t.TempDir(), "opencode", "harnez-distill.ts")

	// 1. ApplyAll should create targetDir/bin/⚙ symlink
	if err := claude.ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}
	fi, err := os.Lstat(gearLink)
	if err != nil {
		t.Fatalf("expected gear symlink at %s: %v", gearLink, err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to be a symlink", gearLink)
	}

	// 2. DiffAll should report no changes when aligned
	changed, err := claude.DiffAll(targetDir, cfg, claude.FullComponentSelection{})
	if err != nil {
		t.Fatalf("DiffAll failed: %v", err)
	}
	if changed {
		t.Errorf("expected DiffAll to report false when gear symlink is present")
	}

	// 3. Remove symlink -> DiffAll should report drift
	if err := os.Remove(gearLink); err != nil {
		t.Fatalf("remove gearLink: %v", err)
	}
	changed, err = claude.DiffAll(targetDir, cfg, claude.FullComponentSelection{})
	if err != nil {
		t.Fatalf("DiffAll failed: %v", err)
	}
	if !changed {
		t.Errorf("expected DiffAll to report true when gear symlink is missing")
	}

	// 4. Re-apply to repair
	if err := claude.ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll repair failed: %v", err)
	}
	if _, err := os.Lstat(gearLink); err != nil {
		t.Fatalf("expected gear symlink to be restored: %v", err)
	}

	// 5. CleanAll should remove gear symlink
	if err := claude.CleanAll(targetDir, cfg); err != nil {
		t.Fatalf("CleanAll failed: %v", err)
	}
	if _, err := os.Lstat(gearLink); !os.IsNotExist(err) {
		t.Errorf("expected gear symlink to be removed by CleanAll, but err = %v", err)
	}
}
