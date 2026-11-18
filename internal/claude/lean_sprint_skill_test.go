package claude

import "testing"

func TestLeanSprintSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg, _ := applySkillsForTest(t)
	checkInstalledSkill(t, cfg, skillCheck{
		name: "lean-sprint",
		file: "docs/commands/lean-sprint.md",
		names: []string{
			"`/lean-sprint <ticket> <searcher>/<developer>/<reviewer>`",
			"harnez agent start --dry-run",
		},
		// Slash picks (issue 1059): a rejected or extra pick must not be swapped for another model.
		rules: []string{"never substitute", "host still reviews every diff"},
	})
}
