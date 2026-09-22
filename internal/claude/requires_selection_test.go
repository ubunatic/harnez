package claude

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// findSkill returns a copy of the named skill from an embedded config,
// letting a test attach `Requires` without touching config.yaml.
func findSkill(t *testing.T, cfg *Config, name string) (Command, int) {
	t.Helper()
	for i, skill := range cfg.Skills {
		if skill.Name == name {
			return skill, i
		}
	}
	t.Fatalf("skill %q not found in embedded config", name)
	return Command{}, -1
}

// TestApplyRemovesResourceBearingSkillWithUnmetRequires generalizes issue
// 142's rate_feedback removal to any skill with an unmet `requires:`
// (issue 491 M5): the skill's SKILL.md and its resource files must be
// actively removed, not merely skipped, once its required component drops
// out of the selection.
func TestApplyRemovesResourceBearingSkillWithUnmetRequires(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded: %v", err)
	}
	skillsDir := t.TempDir()
	cfg.ClaudeSkillsTarget = skillsDir
	cfg.SkillsTarget = filepath.Join(t.TempDir(), "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(t.TempDir(), "codex-skills")
	cfg.PrimeAgentTarget = ""
	cfg.CodexHooksTarget = ""
	cfg.AgyHooksTarget = ""

	docup, idx := findSkill(t, cfg, "docup")
	if len(docup.Resources) == 0 {
		t.Fatalf("docup skill has no resources; test needs a resource-bearing skill")
	}
	docup.Requires = []string{"telemetry"}
	cfg.Skills[idx] = docup

	skillMD := filepath.Join(skillsDir, "docup", "SKILL.md")
	resourcePaths := make([]string, len(docup.Resources))
	for i, r := range docup.Resources {
		resourcePaths[i] = filepath.Join(skillsDir, "docup", r.Target)
	}

	target := t.TempDir()
	full := Set{Docs: true, Skills: true, Telemetry: true, Agents: true, Usage: true}
	if err := ApplyAllVariant(target, cfg, full, nil, false, false, "", false); err != nil {
		t.Fatalf("apply with telemetry selected: %v", err)
	}
	if _, err := os.Stat(skillMD); err != nil {
		t.Fatalf("expected %s to exist under telemetry selection: %v", skillMD, err)
	}
	for _, p := range resourcePaths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected resource %s to exist under telemetry selection: %v", p, err)
		}
	}

	withoutTelemetry := Set{Docs: true, Skills: true, Agents: true, Usage: true}
	if err := ApplyAllVariant(target, cfg, withoutTelemetry, nil, false, false, "", false); err != nil {
		t.Fatalf("apply without telemetry: %v", err)
	}
	if _, err := os.Stat(skillMD); !os.IsNotExist(err) {
		t.Errorf("expected %s to be removed once its requires: component drops out, stat err = %v", skillMD, err)
	}
	for _, p := range resourcePaths {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected resource %s to be removed once its requires: component drops out, stat err = %v", p, err)
		}
	}

	changed, err := DiffAll(target, cfg, withoutTelemetry)
	if err != nil {
		t.Fatalf("DiffAll: %v", err)
	}
	if changed {
		t.Errorf("expected DiffAll to report no drift once the unmet-requires skill is fully removed")
	}
}

// TestCodexAgyAppliedByTelemetrySelection verifies issue 491 M5: Codex/AGY
// hooks are installed when telemetry is selected and actively removed
// (ownership-aware) when it is not, and DiffAll checks them under the
// resolved selection instead of skipping them.
func TestCodexAgyAppliedByTelemetrySelection(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded: %v", err)
	}
	cfg.Skills = nil
	cfg.Commands = nil
	cfg.SkillsTarget = filepath.Join(t.TempDir(), "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(t.TempDir(), "codex-skills")
	cfg.ClaudeSkillsTarget = filepath.Join(t.TempDir(), "claude-skills")
	cfg.PrimeAgentTarget = ""
	cfg.CodexHooksTarget = filepath.Join(t.TempDir(), "codex-config.toml")
	cfg.AgyHooksTarget = ""

	target := t.TempDir()
	withTelemetry := Set{Docs: true, Skills: true, Telemetry: true, Agents: true, Usage: true}
	if err := ApplyAllVariant(target, cfg, withTelemetry, nil, false, false, "", false); err != nil {
		t.Fatalf("apply with telemetry selected: %v", err)
	}
	if _, err := os.Stat(cfg.CodexHooksTarget); err != nil {
		t.Fatalf("expected codex hooks file to exist under telemetry selection: %v", err)
	}
	if changed, err := DiffAll(target, cfg, withTelemetry); err != nil || changed {
		t.Fatalf("DiffAll with telemetry selected: changed=%v err=%v, want clean", changed, err)
	}

	withoutTelemetry := Set{Docs: true, Skills: true, Agents: true, Usage: true}
	if changed, err := DiffAll(target, cfg, withoutTelemetry); err != nil {
		t.Fatalf("DiffAll without telemetry: %v", err)
	} else if !changed {
		t.Errorf("expected DiffAll to report drift: codex hooks still installed without telemetry selected")
	}

	if err := ApplyAllVariant(target, cfg, withoutTelemetry, nil, false, false, "", false); err != nil {
		t.Fatalf("apply without telemetry: %v", err)
	}
	if _, err := os.Stat(cfg.CodexHooksTarget); !os.IsNotExist(err) {
		t.Errorf("expected codex hooks file to be removed without telemetry selected, stat err = %v", err)
	}
	if changed, err := DiffAll(target, cfg, withoutTelemetry); err != nil || changed {
		t.Fatalf("DiffAll without telemetry after removal: changed=%v err=%v, want clean", changed, err)
	}
}

func TestSkillDisabledHelper(t *testing.T) {
	full := Set{Telemetry: true}
	none := Set{}

	rateFeedbackSkill := Command{Name: "tool-feedback-protocol", RateFeedback: true}
	if skillDisabled(rateFeedbackSkill, full, false) {
		t.Error("rate_feedback skill must stay enabled when not disabled")
	}
	if !skillDisabled(rateFeedbackSkill, full, true) {
		t.Error("rate_feedback skill must be disabled when RateFeedbackDisabled is true")
	}

	requiresSkill := Command{Name: "telemetry-only-skill", Requires: []string{"telemetry"}}
	if skillDisabled(requiresSkill, full, false) {
		t.Error("skill must stay enabled when its requires: component is selected")
	}
	if !skillDisabled(requiresSkill, none, false) {
		t.Error("skill must be disabled when its requires: component isn't selected")
	}

	if !slices.Equal(requiresSkill.Requires, []string{"telemetry"}) {
		t.Fatalf("test setup: expected Requires unchanged, got %v", requiresSkill.Requires)
	}
}
