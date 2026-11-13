package claude

import "testing"

func TestHarnezInitSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg, _ := applySkillsForTest(t)
	checkInstalledSkill(t, cfg, skillCheck{
		name:     "harnez-init",
		file:     "docs/commands/HarnezInit.md",
		override: "user-invocable-only",
		// The invocation switch, the two initialization markers and the commands.
		names: []string{"disable-model-invocation: true", ".harnez/", "harnez:begin", "harnez init", "git diff"},
		// First-time initialization is the user's decision.
		rules: []string{"if not initialized, stop"},
	})
}
