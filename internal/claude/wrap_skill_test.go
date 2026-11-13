package claude

import "testing"

func TestWrapSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg, _ := applySkillsForTest(t)
	checkInstalledSkill(t, cfg, skillCheck{
		name:  "wrap",
		file:  "docs/commands/Wrap.md",
		names: []string{"disable-model-invocation: true", "`/evergreen`"},
		// /wrap ends a session: helpers left running are the side effect to guard.
		rules: []string{"stop background helpers"},
	})
}
