package claude

import (
	"os"
	"path/filepath"
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
// ApplyAll omit the Tool Feedback Protocol skill entirely, and
// disabling it after a prior enabled apply actively removes what was
// already installed (not just skips future writes).
func TestApplyOmitsToolFeedbackProtocolWhenDisabled(t *testing.T) {
	targetDir := t.TempDir()
	claudeMDPath := filepath.Join(t.TempDir(), "CLAUDE.md")

	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	cfg.SkillsTarget = filepath.Join(t.TempDir(), "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(t.TempDir(), "codex-skills")
	cfg.CodexHooksTarget = filepath.Join(t.TempDir(), "codex-config.toml")
	cfg.AgyHooksTarget = filepath.Join(t.TempDir(), "gemini", "config", "hooks.json")
	skillsDir := filepath.Join(t.TempDir(), "claude-skills")
	cfg.ClaudeSkillsTarget = skillsDir
	cfg.PrimeAgentTarget = ""
	cfg.AgentsMD.Global.Target = claudeMDPath
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil
	cfg.DistillAutopipe.PiExtensionTarget = filepath.Join(t.TempDir(), "pi", "harnez-distill.ts")
	cfg.DistillAutopipe.OpenCodePluginTarget = filepath.Join(t.TempDir(), "opencode", "harnez-distill.ts")

	skillPath := filepath.Join(skillsDir, "tool-feedback-protocol", "SKILL.md")

	// Step 1: apply enabled — sanity check the skill lands
	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("first ApplyAll (enabled) failed: %v", err)
	}
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected skill file %s to exist after enabled apply: %v", skillPath, err)
	}

	// Step 2: disable and re-apply — the skill must be actively removed
	cfg.Feedback.DisableRateProtocol = true
	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("second ApplyAll (disabled) failed: %v", err)
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
