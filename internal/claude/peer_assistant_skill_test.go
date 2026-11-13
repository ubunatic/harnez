package claude

import "testing"

func TestPeerAssistantSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg, _ := applySkillsForTest(t)
	contents := checkInstalledSkill(t, cfg, skillCheck{
		name: "peer-assistant",
		file: "docs/commands/PeerAssistant.md",
		names: []string{
			"`harnez init`",
			"harnez find -d <repo> issues -a status:open",
			"git log <checkpoint>..HEAD -- issues/",
		},
		// Two sessions in one checkout: the one-writer rule and helper cleanup must survive a trim.
		rules: []string{"writer per shared workspace", "stop or delete it"},
	})
	assertSameSkillEverywhere(t, "peer-assistant", contents)
}
