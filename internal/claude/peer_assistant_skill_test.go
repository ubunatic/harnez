package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPeerAssistantSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg := loadTestConfig(t)
	targetDir := t.TempDir()

	var skill *Command
	for i := range cfg.Skills {
		if cfg.Skills[i].Name == "peer-assistant" {
			skill = &cfg.Skills[i]
			break
		}
	}
	if skill == nil {
		t.Fatal("peer-assistant skill not registered in embedded config")
	}
	if skill.File != "docs/commands/PeerAssistant.md" {
		t.Fatalf("peer-assistant skill file = %q, want docs/commands/PeerAssistant.md", skill.File)
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

	want := ""
	for _, skillsRoot := range skillTargets(cfg) {
		path := filepath.Join(skillsRoot, "peer-assistant", "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected peer-assistant skill at %s: %v", path, err)
		}
		content := string(data)
		if !strings.HasPrefix(content, "---\nname: \"peer-assistant\"\ndescription:") {
			t.Errorf("expected skill frontmatter in %s, got:\n%s", path, content)
		}
		if want == "" {
			want = content
		} else if content != want {
			t.Errorf("skill content in %s differs from the other targets", path)
		}
		for _, marker := range []string{
			"## Assisting Loop",
			"## Watching",
			"## Peer-Reported Bugs",
			"run `harnez init` in your repo",
			"## Peer Communication",
			"harnez find -d <repo> issues -a status:open",
			"git log <checkpoint>..HEAD -- issues/",
			"Otherwise check at each turn",
			"only while a scheduled check is running",
			"available native messaging/session tools",
			"writer per shared workspace",
			"durable documentation",
			"stop or delete it",
		} {
			if !strings.Contains(content, marker) {
				t.Errorf("expected %s to contain %q", path, marker)
			}
		}
	}
}
