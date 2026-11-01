package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecideSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg := loadTestConfig(t)
	targetDir := t.TempDir()

	var decide *Command
	for i := range cfg.Skills {
		if cfg.Skills[i].Name == "decide" {
			decide = &cfg.Skills[i]
			break
		}
	}
	if decide == nil {
		t.Fatal("decide skill not registered in embedded config")
	}
	if decide.File != "docs/commands/decide.md" {
		t.Fatalf("decide skill file = %q, want docs/commands/decide.md", decide.File)
	}
	if got := cfg.Debloat.PresetSkillOverrides["decide"]; got != "user-invocable-only" {
		t.Fatalf("decide preset override = %q, want user-invocable-only", got)
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
		path := filepath.Join(skillsRoot, "decide", "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected decide skill at %s: %v", path, err)
		}
		content := string(data)
		if !strings.HasPrefix(content, "---\nname: \"decide\"\ndescription:") {
			t.Errorf("expected skill frontmatter in %s, got:\n%s", path, content)
		}
		for _, want := range []string{
			"## Rules",
			"--model cheap",
			"--model smart",
			"docs/lang/Git.md",
			"Never force-push.",
			"grants no release",
			"Very critical only",
			"relayed answer does not count",
			"Circuit breaker",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("expected %s to contain %q", path, want)
			}
		}
	}
}
