package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHarnezInitSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg := loadTestConfig(t)
	targetDir := t.TempDir()

	var harnezInit *Command
	for i := range cfg.Skills {
		if cfg.Skills[i].Name == "harnez-init" {
			harnezInit = &cfg.Skills[i]
			break
		}
	}
	if harnezInit == nil {
		t.Fatal("harnez-init skill not registered in embedded config")
	}
	if harnezInit.File != "docs/commands/HarnezInit.md" {
		t.Fatalf("harnez-init skill file = %q, want docs/commands/HarnezInit.md", harnezInit.File)
	}
	if override := cfg.Debloat.PresetSkillOverrides["harnez-init"]; override != "user-invocable-only" {
		t.Errorf("PresetSkillOverrides[harnez-init] = %q, want user-invocable-only", override)
	}

	cfg.SkillsTarget = filepath.Join(t.TempDir(), "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(t.TempDir(), "codex-skills")
	cfg.CodexHooksTarget = filepath.Join(t.TempDir(), "codex-config.toml")
	cfg.AgyHooksTarget = filepath.Join(t.TempDir(), "gemini", "config", "hooks.json")
	cfg.ClaudeSkillsTarget = filepath.Join(t.TempDir(), "claude-skills")
	cfg.PrimeAgentTarget = filepath.Join(t.TempDir(), "prime-agent")
	cfg.AgentsMD.Global.Target = filepath.Join(t.TempDir(), "AGENTS.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil
	cfg.DistillAutopipe.PiExtensionTarget = filepath.Join(t.TempDir(), "pi", "harnez-distill.ts")
	cfg.DistillAutopipe.OpenCodePluginTarget = filepath.Join(t.TempDir(), "opencode", "harnez-distill.ts")

	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	for _, skillsRoot := range skillTargets(cfg) {
		path := filepath.Join(skillsRoot, "harnez-init", "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected harnez-init skill at %s: %v", path, err)
		}
		content := string(data)
		if !strings.HasPrefix(content, "---\nname: \"harnez-init\"\ndescription:") {
			t.Errorf("expected skill frontmatter in %s, got:\n%s", path, content)
		}
		for _, want := range []string{
			"disable-model-invocation: true",
			"### 1. Guard: Verify Prior Initialization",
			".harnez/",
			"harnez:begin",
			"If NOT initialized",
			"Stop immediately",
			"Doc variant",
			"--variant lite",
			"--variant full",
			"Quota guardrails",
			"Documentation set",
			"Repository mode",
			"### 2. Re-run Init",
			"harnez init",
			"### 3. Assess the Diff",
			"git diff",
			"Broken links",
			"Self-referential pointers",
			"Contradicting rules",
			"Lost local edits",
			"### 4. Fix Local Fallout",
			"### 5. File Upstream Issues",
			"### 6. Commit and Report",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("expected %s to contain %q", path, want)
			}
		}
	}
}
