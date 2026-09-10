package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHarnezAdvisorSkillInstallsIdenticallyToEveryAgentTarget(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	root := t.TempDir()
	cfg.SkillsTarget = filepath.Join(root, "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(root, "codex-skills")
	cfg.ClaudeSkillsTarget = filepath.Join(root, "claude-skills")
	cfg.PrimeAgentTarget = filepath.Join(root, "prime-agent")
	cfg.CodexHooksTarget = filepath.Join(root, "codex-config.toml")
	cfg.AgyHooksTarget = filepath.Join(root, "gemini", "config", "hooks.json")
	cfg.AgentsMD.Global.Target = filepath.Join(root, "AGENTS.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil

	if err := ApplyAll(root, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	expectedTargets := []string{
		filepath.Join(root, "gemini-skills"),
		filepath.Join(root, "codex-skills"),
		filepath.Join(root, "claude-skills"),
		filepath.Join(root, "prime-agent", "skills"),
	}
	targets := skillTargets(cfg)
	if len(targets) != len(expectedTargets) {
		t.Fatalf("expected four agent skill targets, got %d: %v", len(targets), targets)
	}
	want := ""
	for _, target := range expectedTargets {
		path := filepath.Join(target, "harnez-advisor", "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(data)
		if !strings.HasPrefix(content, "---\nname: \"harnez-advisor\"\ndescription:") {
			t.Errorf("expected skill frontmatter in %s, got:\n%s", path, content)
		}
		if want == "" {
			want = content
		} else if content != want {
			t.Errorf("skill content in %s differs from the other targets", path)
		}
	}
	if !strings.Contains(want, "There are no portable skill flags.") {
		t.Errorf("skill does not state the prose-only contract:\n%s", want)
	}
}
