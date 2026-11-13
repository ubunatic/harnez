package claude

import "testing"

func TestDecideSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg, _ := applySkillsForTest(t)
	checkInstalledSkill(t, cfg, skillCheck{
		name:     "decide",
		file:     "docs/commands/decide.md",
		override: "user-invocable-only",
		names:    []string{"--model cheap", "--model smart", "docs/lang/Git.md"},
		// /decide grants push authority: its limits and the stop-and-ask cases must survive a trim.
		rules: []string{
			"force-push",
			"no release",
			"destructive or irreversible",
			"spending money",
			"publishing outside",
			"relayed answer does not count",
		},
	})
}
