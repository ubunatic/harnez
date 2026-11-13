package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// skillCheck is what a per-skill test asserts about an installed skill (issue 1224):
// registration, installation to every target, the names agents type, and short stems of
// rules with side effects. Headings and sentences are free to change.
type skillCheck struct {
	name     string
	file     string   // expected `file:` in config.yaml; empty skips the check
	override string   // expected debloat preset override; empty skips the check
	names    []string // commands, flags, paths and markers, matched verbatim
	rules    []string // lower-case stems, matched on lower-cased, whitespace-normalised text
}

// applySkillsForTest applies the embedded config to temporary targets only and returns the
// config and the Claude target directory, so a test can apply a second time.
func applySkillsForTest(t *testing.T) (*Config, string) {
	t.Helper()
	cfg := loadTestConfig(t)
	targetDir := t.TempDir()

	cfg.SkillsTarget = filepath.Join(t.TempDir(), "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(t.TempDir(), "codex-skills")
	cfg.CodexHooksTarget = filepath.Join(t.TempDir(), "codex-config.toml")
	cfg.AgyHooksTarget = filepath.Join(t.TempDir(), "gemini", "config", "hooks.json")
	cfg.ClaudeSkillsTarget = filepath.Join(t.TempDir(), "claude-skills")
	cfg.PrimeAgentTarget = filepath.Join(t.TempDir(), "prime-agent")
	cfg.AgentsMD.Global.Target = filepath.Join(t.TempDir(), "AGENTS.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil // issue 149: keep tests off the real ~/.codex path
	cfg.DistillAutopipe.PiExtensionTarget = filepath.Join(t.TempDir(), "pi", "harnez-distill.ts")
	cfg.DistillAutopipe.OpenCodePluginTarget = filepath.Join(t.TempDir(), "opencode", "harnez-distill.ts")

	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}
	return cfg, targetDir
}

func normalizeSkillText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// checkInstalledSkill checks c against the config and against the SKILL.md in every skill
// target. It returns the installed contents in target order.
func checkInstalledSkill(t *testing.T, cfg *Config, c skillCheck) []string {
	t.Helper()
	var skill *Command
	for i := range cfg.Skills {
		if cfg.Skills[i].Name == c.name {
			skill = &cfg.Skills[i]
			break
		}
	}
	if skill == nil {
		t.Fatalf("%s skill not registered in embedded config", c.name)
	}
	if c.file != "" && skill.File != c.file {
		t.Errorf("%s skill file = %q, want %s", c.name, skill.File, c.file)
	}
	if c.override != "" {
		if got := cfg.Debloat.PresetSkillOverrides[c.name]; got != c.override {
			t.Errorf("PresetSkillOverrides[%s] = %q, want %s", c.name, got, c.override)
		}
	}

	targets := skillTargets(cfg)
	if len(targets) == 0 {
		t.Fatal("no skill targets configured")
	}
	var contents []string
	for _, skillsRoot := range targets {
		path := filepath.Join(skillsRoot, c.name, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected %s skill at %s: %v", c.name, path, err)
		}
		content := string(data)
		contents = append(contents, content)
		if !strings.HasPrefix(content, "---\nname: \""+c.name+"\"\ndescription:") {
			t.Errorf("expected skill frontmatter in %s, got:\n%s", path, content)
		}
		for _, name := range c.names {
			if !strings.Contains(content, name) {
				t.Errorf("%s no longer names %q", path, name)
			}
		}
		normalized := normalizeSkillText(content)
		for _, rule := range c.rules {
			if !strings.Contains(normalized, rule) {
				t.Errorf("%s lost the rule with stem %q", path, rule)
			}
		}
	}
	return contents
}

// assertSameSkillEverywhere fails when the installed copies differ between targets.
func assertSameSkillEverywhere(t *testing.T, name string, contents []string) {
	t.Helper()
	for i := 1; i < len(contents); i++ {
		if contents[i] != contents[0] {
			t.Errorf("%s skill content differs between targets 0 and %d", name, i)
		}
	}
}
