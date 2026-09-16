package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueCommandAndSkillInstallToEveryConfiguredTarget(t *testing.T) {
	targetDir := t.TempDir()
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

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

	for _, skillsRoot := range skillTargets(cfg) {
		path := filepath.Join(skillsRoot, "issue", "SKILL.md")
		content := readIssueInstall(t, path)
		if !strings.HasPrefix(content, "---\nname: \"issue\"\ndescription:") {
			t.Errorf("expected skill frontmatter in %s, got:\n%s", path, content)
		}
		assertIssueTLDR(t, path, content)
	}

	out, err := captureStdoutClaudeSkills(func() error {
		return ApplyAll(targetDir, cfg, nil, false, false)
	})
	if err != nil {
		t.Fatalf("second ApplyAll failed: %v", err)
	}
	if !strings.Contains(out, "No changes.") {
		t.Errorf("expected second ApplyAll to be idempotent, got:\n%s", out)
	}
}

func readIssueInstall(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected issue workflow at %s: %v", path, err)
	}
	return string(data)
}

func assertIssueTLDR(t *testing.T, path, content string) {
	t.Helper()
	for _, want := range []string{
		"## TL;DR",
		`harnez find -d <repo> issues "<search terms>"`,
		`harnez issues new -d <repo> "<title>"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %s to contain %q, got:\n%s", path, want, content)
		}
	}
}
