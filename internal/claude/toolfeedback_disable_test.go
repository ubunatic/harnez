package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRateFeedbackDisabled_ConfigAndEnv exercises issue 142's two opt-out
// sources: the config.yaml flag and the HARNEZ_DISABLE_RATE_FEEDBACK env
// var, plus the various "off" spellings the env var must still treat as
// disabled=false.
func TestRateFeedbackDisabled_ConfigAndEnv(t *testing.T) {
	getenv := func(v string) func(string) string {
		return func(string) string { return v }
	}

	cases := []struct {
		name     string
		cfgFlag  bool
		envValue string
		want     bool
	}{
		{"default enabled", false, "", false},
		{"config disables", true, "", true},
		{"env truthy disables", false, "1", true},
		{"env 'true' disables", false, "true", true},
		{"env empty does not disable", false, "", false},
		{"env '0' does not disable", false, "0", false},
		{"env 'false' does not disable", false, "false", false},
		{"env 'off' does not disable", false, "off", false},
		{"config wins even if env off", true, "0", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := &Config{Feedback: FeedbackConfig{DisableRateProtocol: c.cfgFlag}}
			got := RateFeedbackDisabled(cfg, getenv(c.envValue))
			if got != c.want {
				t.Errorf("RateFeedbackDisabled(cfgFlag=%v, env=%q) = %v, want %v", c.cfgFlag, c.envValue, got, c.want)
			}
		})
	}
}

// TestApplyOmitsToolFeedbackProtocolWhenDisabled verifies issue 142's core
// acceptance criterion: setting cfg.Feedback.DisableRateProtocol makes
// ApplyAll omit the Tool Feedback Protocol section/skill entirely, and
// disabling it after a prior enabled apply actively removes what was
// already installed (not just skips future writes) — otherwise a user
// toggling the flag off would still carry the stale instruction forever.
func TestApplyOmitsToolFeedbackProtocolWhenDisabled(t *testing.T) {
	targetDir := t.TempDir()
	claudeMDPath := filepath.Join(t.TempDir(), "CLAUDE.md")

	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	cfg.SkillsTarget = filepath.Join(t.TempDir(), "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(t.TempDir(), "codex-skills")
	skillsDir := filepath.Join(t.TempDir(), "claude-skills")
	cfg.ClaudeSkillsTarget = skillsDir
	cfg.PrimeAgentTarget = ""
	cfg.AgentsMD.Global.Target = claudeMDPath
	cfg.AgentsMD.Global.Symlink = ""
	cfg.DistillAutopipe.PiExtensionTarget = filepath.Join(t.TempDir(), "pi", "harnez-distill.ts")
	cfg.DistillAutopipe.OpenCodePluginTarget = filepath.Join(t.TempDir(), "opencode", "harnez-distill.ts")

	skillPath := filepath.Join(skillsDir, "tool-feedback-protocol", "SKILL.md")

	// Step 1: apply enabled — sanity check the section/skill land, matching
	// TestApplyInstallsToolFeedbackProtocol's existing coverage.
	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("first ApplyAll (enabled) failed: %v", err)
	}
	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("expected %s to exist after enabled apply: %v", claudeMDPath, err)
	}
	if !strings.Contains(string(data), "### Tool Feedback Protocol") {
		t.Fatalf("expected Tool Feedback Protocol section present after enabled apply, got:\n%s", data)
	}
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected skill file %s to exist after enabled apply: %v", skillPath, err)
	}

	// Step 2: disable and re-apply — the section/skill must be actively
	// removed, and other sections (e.g. Issue Tracker Discovery) untouched.
	cfg.Feedback.DisableRateProtocol = true
	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("second ApplyAll (disabled) failed: %v", err)
	}
	data, err = os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("expected %s to still exist after disabling (other sections remain): %v", claudeMDPath, err)
	}
	content := string(data)
	if strings.Contains(content, "Tool Feedback Protocol") {
		t.Errorf("expected Tool Feedback Protocol section removed after disabling, got:\n%s", content)
	}
	if !strings.Contains(content, "### Issue Tracker Discovery") {
		t.Errorf("expected unrelated Issue Tracker Discovery section to remain untouched, got:\n%s", content)
	}
	if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Errorf("expected skill file %s to be removed after disabling, stat err = %v", skillPath, err)
	}

	// DiffAll while disabled must report no drift (fully cleaned already).
	changed, err := DiffAll(targetDir, cfg)
	if err != nil {
		t.Fatalf("DiffAll failed: %v", err)
	}
	if changed {
		t.Errorf("expected DiffAll to report no drift once disabled state is fully applied")
	}
}
