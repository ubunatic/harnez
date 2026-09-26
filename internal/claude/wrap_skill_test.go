package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrapSkillInstallToEveryConfiguredTarget(t *testing.T) {
	targetDir := t.TempDir()
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	var wrap *Command
	for i := range cfg.Skills {
		if cfg.Skills[i].Name == "wrap" {
			wrap = &cfg.Skills[i]
			break
		}
	}
	if wrap == nil {
		t.Fatal("wrap skill not registered in embedded config")
	}
	if wrap.File != "docs/commands/Wrap.md" {
		t.Fatalf("wrap skill file = %q, want docs/commands/Wrap.md", wrap.File)
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
		path := filepath.Join(skillsRoot, "wrap", "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected wrap skill at %s: %v", path, err)
		}
		content := string(data)
		if !strings.HasPrefix(content, "---\nname: \"wrap\"\ndescription:") {
			t.Errorf("expected skill frontmatter in %s, got:\n%s", path, content)
		}
		for _, want := range []string{
			"disable-model-invocation: true",
			"## Session Close Checklist",
			"Stop background helpers and subagents",
			"issue index",
			"partial or broken",
			"handoff issue",
			"`/evergreen`",
			"curates",
			"learnings",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("expected %s to contain %q", path, want)
			}
		}
	}
}
