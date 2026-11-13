package claude

import (
	"strings"
	"testing"
)

func TestReviewSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg, targetDir := applySkillsForTest(t)
	checkInstalledSkill(t, cfg, skillCheck{
		name: "review",
		names: []string{
			"`git diff`",
			"`git status`",
			`harnez issues new -d <repo> "<title>"`,
		},
	})

	out, err := CaptureTestStdout(func() error {
		return ApplyAll(targetDir, cfg, nil, false, false)
	})
	if err != nil {
		t.Fatalf("second ApplyAll failed: %v", err)
	}
	if !strings.Contains(out, "No changes.") {
		t.Errorf("expected second ApplyAll to be idempotent, got:\n%s", out)
	}
}
