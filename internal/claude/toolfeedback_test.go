package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolFeedbackProtocolConfigEntry(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	var found *Command
	for i := range cfg.Skills {
		s := &cfg.Skills[i]
		if s.Name == "tool-feedback-protocol" {
			found = s
			break
		}
	}
	if found == nil {
		t.Fatalf("expected a skills entry named %q in embedded config.yaml", "tool-feedback-protocol")
	}
	if !strings.Contains(found.Content, "harnez rate <tool_name> <1-5>") {
		t.Errorf("expected tool-feedback-protocol skill to contain the harnez rate invocation, got:\n%s", found.Content)
	}
	if !strings.Contains(found.Content, "harnez rate --ok") {
		t.Errorf("expected tool-feedback-protocol skill to also document `harnez rate --ok`, got:\n%s", found.Content)
	}
}

func TestApplyInstallsToolFeedbackProtocol(t *testing.T) {
	targetDir := t.TempDir()

	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	claudeMDPath := filepath.Join(t.TempDir(), "CLAUDE.md")
	primeAgentDir := filepath.Join(t.TempDir(), "prime-agent")
	skillsDir := filepath.Join(t.TempDir(), "claude-skills")
	cfg.SkillsTarget = filepath.Join(t.TempDir(), "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(t.TempDir(), "codex-skills")
	cfg.CodexHooksTarget = filepath.Join(t.TempDir(), "codex-config.toml")
	cfg.AgyHooksTarget = filepath.Join(t.TempDir(), "gemini", "config", "hooks.json")
	cfg.ClaudeSkillsTarget = skillsDir
	cfg.PrimeAgentTarget = primeAgentDir
	cfg.AgentsMD.Global.Target = claudeMDPath
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil
	cfg.DistillAutopipe.PiExtensionTarget = filepath.Join(t.TempDir(), "pi", "harnez-distill.ts")
	cfg.DistillAutopipe.OpenCodePluginTarget = filepath.Join(t.TempDir(), "opencode", "harnez-distill.ts")

	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("first ApplyAll failed: %v", err)
	}

	skillPath := filepath.Join(skillsDir, "tool-feedback-protocol", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("expected skill file %s to be written: %v", skillPath, err)
	}
	content := string(data)
	if !strings.Contains(content, "harnez rate <tool_name> <1-5>") {
		t.Errorf("expected %s to contain harnez rate invocation, got:\n%s", skillPath, content)
	}

	// Verify global root instruction files were NOT created or modified
	if _, err := os.Stat(claudeMDPath); !os.IsNotExist(err) {
		t.Errorf("expected %s NOT to be created by ApplyAll", claudeMDPath)
	}
	primeAGENTSPath := filepath.Join(primeAgentDir, "AGENTS.md")
	if _, err := os.Stat(primeAGENTSPath); !os.IsNotExist(err) {
		t.Errorf("expected %s NOT to be created by ApplyAll", primeAGENTSPath)
	}

	// Idempotency: second apply
	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("second ApplyAll failed: %v", err)
	}

	// CleanAll must remove the skill
	if err := CleanAll(targetDir, cfg); err != nil {
		t.Fatalf("CleanAll failed: %v", err)
	}
	if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Errorf("expected skill %s to be removed by CleanAll", skillPath)
	}
}

func TestIssueTrackerDiscoveryConfigEntry(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	var found *MDSection
	for i := range cfg.AgentsMD.Local.Sections {
		s := &cfg.AgentsMD.Local.Sections[i]
		if strings.Contains(s.Content, "harnez find -d <repo> issues -a status:open") {
			found = s
			break
		}
	}
	if found == nil {
		t.Fatalf("expected an agents_md.local.sections entry containing the harnez find command list in embedded config.yaml")
	}
	if !strings.Contains(found.Content, "harnez index -d <repo>") {
		t.Errorf("expected Issue Tracker Discovery content to contain harnez index, got:\n%s", found.Content)
	}
}
