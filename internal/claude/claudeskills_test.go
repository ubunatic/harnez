package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClaudeSkillsTargetRoundTrip exercises issue 130 item 7: harnez-authored
// skills must reach Claude Code as real ~/.claude/skills/<name>/SKILL.md
// files (auto-triggered Skills), not only as manually-invoked slash commands
// under ~/.claude/commands/. This covers the full apply -> diff -> status ->
// clean lifecycle for the new claude_skills_target, isolated from every
// other target (Gemini/Codex/Prime) this apply run would also touch.
func TestClaudeSkillsTargetRoundTrip(t *testing.T) {
	targetDir := t.TempDir()

	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	claudeSkillsDir := filepath.Join(t.TempDir(), "claude-skills")
	cfg.SkillsTarget = filepath.Join(t.TempDir(), "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(t.TempDir(), "codex-skills")
	cfg.CodexHooksTarget = filepath.Join(t.TempDir(), "codex-config.toml")
	cfg.AgyHooksTarget = filepath.Join(t.TempDir(), "gemini", "config", "hooks.json")
	cfg.ClaudeSkillsTarget = claudeSkillsDir
	cfg.PrimeAgentTarget = filepath.Join(t.TempDir(), "prime-agent")
	cfg.AgentsMD.Global.Target = filepath.Join(t.TempDir(), "CLAUDE.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil // issue 149: keep tests off the real ~/.codex path
	cfg.DistillAutopipe.PiExtensionTarget = filepath.Join(t.TempDir(), "pi", "harnez-distill.ts")
	cfg.DistillAutopipe.OpenCodePluginTarget = filepath.Join(t.TempDir(), "opencode", "harnez-distill.ts")

	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	skillPath := filepath.Join(claudeSkillsDir, "evergreen", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("expected %s to be written: %v", skillPath, err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "---\nname: \"evergreen\"\ndescription:") {
		t.Errorf("expected %s to carry Agent Skills frontmatter (name/description), got:\n%s", skillPath, content)
	}

	publishPath := filepath.Join(claudeSkillsDir, "publish", "SKILL.md")
	pubData, err := os.ReadFile(publishPath)
	if err != nil {
		t.Fatalf("expected %s to be written: %v", publishPath, err)
	}
	pubContent := string(pubData)
	if !strings.HasPrefix(pubContent, "---\nname: \"publish\"\ndescription:") {
		t.Errorf("expected %s to carry Agent Skills frontmatter (name/description), got:\n%s", publishPath, pubContent)
	}
	// diff must report no drift immediately after apply.
	changed, err := DiffAll(targetDir, cfg)
	if err != nil {
		t.Fatalf("DiffAll failed: %v", err)
	}
	if changed {
		t.Errorf("expected DiffAll to report no drift right after ApplyAll")
	}

	// status must report the new target as ok.
	statusOut, err := captureStdoutClaudeSkills(func() error {
		return RunStatus("(embedded)", cfg, targetDir)
	})
	if err != nil {
		t.Fatalf("RunStatus failed: %v", err)
	}
	if !strings.Contains(statusOut, skillPath) || !strings.Contains(statusOut, "ok") {
		t.Errorf("expected RunStatus to report %s as ok, got:\n%s", skillPath, statusOut)
	}

	// introduce drift by hand-editing the file, then confirm diff detects it.
	if err := os.WriteFile(skillPath, []byte("drifted"), 0644); err != nil {
		t.Fatalf("failed to simulate drift: %v", err)
	}
	changed, err = DiffAll(targetDir, cfg)
	if err != nil {
		t.Fatalf("DiffAll after drift failed: %v", err)
	}
	if !changed {
		t.Errorf("expected DiffAll to report drift after hand-editing %s", skillPath)
	}

	// re-apply should repair the drift.
	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("repair ApplyAll failed: %v", err)
	}
	repaired, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("expected %s to still exist after repair: %v", skillPath, err)
	}
	if string(repaired) != content {
		t.Errorf("expected repaired content to match original apply output")
	}

	// clean must remove the Claude Code skill file (and its now-empty dir).
	if err := CleanAll(targetDir, cfg); err != nil {
		t.Fatalf("CleanAll failed: %v", err)
	}
	if _, err := os.Stat(skillPath); err == nil || !os.IsNotExist(err) {
		t.Errorf("expected %s to be removed by CleanAll, err=%v", skillPath, err)
	}
	if _, err := os.Stat(filepath.Join(claudeSkillsDir, "evergreen")); err == nil {
		t.Errorf("expected the now-empty evergreen skill dir to be removed by CleanAll")
	}
}

func captureStdoutClaudeSkills(f func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	errVal := f()

	w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 0, 65536)
	tmp := make([]byte, 4096)
	for {
		n, rerr := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if rerr != nil {
			break
		}
	}
	return string(buf), errVal
}
