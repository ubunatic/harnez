package claude

import "testing"

func TestHarnezLandSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg, _ := applySkillsForTest(t)
	checkInstalledSkill(t, cfg, skillCheck{
		name: "harnez-land",
		file: "docs/commands/HarnezLand.md",
		names: []string{
			"disable-model-invocation: true",
			"refs/for/main/<topic>",
			"harnez pr wait <number> --contains <sha>",
			"git merge --ff-only origin/main",
			"git merge-tree --write-tree origin/main <branch>",
		},
		// /harnez-land pushes and switches branches: rewriting main or remote history is the side effect to guard.
		rules: []string{"force-push", "never commit on `main`", "never reset or rebase `main`"},
	})
}
