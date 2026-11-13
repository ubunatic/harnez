package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueCommandAndSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg, targetDir := applySkillsForTest(t)
	contents := checkInstalledSkill(t, cfg, skillCheck{
		name: "issue",
		names: []string{
			`harnez find -d <repo> issues "<search terms>"`,
			`harnez issues new -d <repo> "<title>"`,
			"`/goal`",
			"git -C <repo> push origin HEAD:refs/for/main/<topic>",
			"git -C <repo> merge-base --is-ancestor <sha> origin/main",
			"harnez pr wait <number> --contains <sha>",
			"`--force`",
		},
		// The skill publishes commits: the limits on that must survive a trim.
		rules: []string{
			"do not fall back to a direct push",
			"never delete or overwrite",
			"stop and report",
		},
	})
	for i, skillsRoot := range skillTargets(cfg) {
		assertIssueRouteOrder(t, filepath.Join(skillsRoot, "issue", "SKILL.md"), contents[i])
		for _, name := range []string{"issue", "review", "roadmap"} {
			path := filepath.Join(skillsRoot, name, "SKILL.md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("expected %s skill at %s: %v", name, path, err)
			}
			for _, line := range strings.Split(string(data), "\n") {
				if strings.Contains(line, "`harnez") || strings.HasPrefix(line, "harnez ") {
					if strings.Contains(line, " -I") || strings.Contains(line, " --image") {
						t.Errorf("%s recommends routine image rendering: %s", path, line)
					}
				}
			}
		}
	}

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

// The three publication routes are tried in this order; the order is structure, not wording.
func assertIssueRouteOrder(t *testing.T, path, content string) {
	t.Helper()
	routes := []string{
		"1. **Direct push to `main`**",
		"2. **AGit PR to `main`**",
		"3. **Stop**",
	}
	last := -1
	for _, route := range routes {
		idx := strings.Index(content, route)
		if idx < 0 {
			t.Fatalf("expected %s to contain route %q, got:\n%s", path, route, content)
		}
		if idx <= last {
			t.Errorf("route %q must come after the previous route in %s", route, path)
		}
		last = idx
	}
}
