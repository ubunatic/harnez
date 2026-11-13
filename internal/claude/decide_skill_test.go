package claude

import "testing"

func TestDecideSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg, _ := applySkillsForTest(t)
	checkInstalledSkill(t, cfg, skillCheck{
		name:     "decide",
		file:     "docs/commands/decide.md",
		override: "user-invocable-only",
		names:    []string{"--model cheap", "--model smart", "docs/lang/Git.md"},
		// /decide grants push authority: the two limits on it must survive a trim.
		rules: []string{"force-push", "no release"},
	})
}
