package claude

import "testing"

func TestWrapSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg, _ := applySkillsForTest(t)
	checkInstalledSkill(t, cfg, skillCheck{
		name: "wrap",
		file: "docs/commands/Wrap.md",
		names: []string{
			"disable-model-invocation: true",
			"`/evergreen`",
			"`/harnez-land`",
			"git push origin HEAD:refs/for/main/<topic>",
			"harnez pr wait <number> --contains <sha>",
		},
		// /wrap ends a session: helpers left running and touching unrelated branches are the side effects to guard.
		rules: []string{"stop background helpers", "never touch unrelated branches", "never force-push"},
	})
}
