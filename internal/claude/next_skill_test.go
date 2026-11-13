package claude

import "testing"

func TestNextSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg, _ := applySkillsForTest(t)
	checkInstalledSkill(t, cfg, skillCheck{
		name:  "next",
		file:  "docs/commands/Next.md",
		names: []string{"harnez find -d . issues -a status:active"},
		// /next starts work: it must ask before executing an ambiguous target.
		rules: []string{"ask the user to choose"},
	})
}
