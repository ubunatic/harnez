package skillreg

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func commitAll(t *testing.T, repo, msg string) {
	t.Helper()
	run(t, repo, "add", "-A")
	run(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", msg)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPlanUpdateUnchangedAndNewSkills(t *testing.T) {
	reg := newReg(t)
	repo := fixtureRepo(t)
	if _, err := reg.Install(InstallOptions{URL: repo, Name: "alpha"}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "plugins/p/skills/gamma/SKILL.md"), "---\nname: gamma\ndescription: g\n---\n")
	commitAll(t, repo, "add gamma")
	plan, err := reg.PlanUpdate("alpha", false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != Unchanged || plan.Diff != "" {
		t.Errorf("status %q diff %q", plan.Status, plan.Diff)
	}
	if len(plan.NewSkills) != 1 || plan.NewSkills[0].Name != "gamma" {
		t.Errorf("new skills: %+v", plan.NewSkills)
	}
}

func TestUpdateFollowsMovedSkill(t *testing.T) {
	reg := newReg(t)
	repo := fixtureRepo(t)
	if _, err := reg.Install(InstallOptions{URL: repo, Name: "alpha"}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "mv", "plugins/p/skills/alpha", "skills/alpha")
	commitAll(t, repo, "move")
	plan, after, err := reg.Update("alpha", false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != Moved || after.Path != "skills/alpha" {
		t.Fatalf("plan %+v after %+v", plan, after)
	}
}

func TestUpdateKeepsGoneSkillAndHintsRename(t *testing.T) {
	reg := newReg(t)
	repo := fixtureRepo(t)
	first, err := reg.Install(InstallOptions{URL: repo, Name: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	run(t, repo, "rm", "-rq", "plugins/p/skills/alpha")
	writeFile(t, filepath.Join(repo, "skills/omega/SKILL.md"), "---\nname: omega\ndescription: o\n---\n")
	writeFile(t, filepath.Join(repo, "CHANGELOG.md"), "- alpha became omega\n- beta is unchanged\n")
	commitAll(t, repo, "rename")
	plan, after, err := reg.Update("alpha", false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != Gone || after.Commit != first.Commit {
		t.Fatalf("plan %+v after %+v", plan, after)
	}
	if !slices.Equal(plan.Candidates, []string{"omega"}) || len(plan.Hints) != 1 || !strings.HasPrefix(plan.Hints[0], "CHANGELOG.md:1:") {
		t.Errorf("candidates %v hints %v", plan.Candidates, plan.Hints)
	}
	if _, err := os.Stat(filepath.Join(reg.Targets[0].Dir, "alpha", "SKILL.md")); err != nil {
		t.Error("gone skill was uninstalled")
	}
}

func TestUpdatePinnedRefStaysUnlessLatest(t *testing.T) {
	reg := newReg(t)
	repo := fixtureRepo(t)
	run(t, repo, "tag", "v1")
	if _, err := reg.Install(InstallOptions{URL: repo, Ref: "v1", Name: "alpha"}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "plugins/p/skills/alpha/SKILL.md"), "---\nname: alpha\ndescription: v2\n---\n")
	commitAll(t, repo, "v2")
	if plan, err := reg.PlanUpdate("alpha", false); err != nil || plan.Status != Unchanged {
		t.Fatalf("pinned plan: %+v %v", plan, err)
	}
	plan, err := reg.PlanUpdate("alpha", true)
	if err != nil || plan.Status != Changed || !strings.Contains(plan.Diff, "+description: v2") {
		t.Fatalf("latest plan: %+v %v", plan, err)
	}
	if e, _, _ := reg.Get("alpha"); e.Ref != "v1" {
		t.Error("PlanUpdate wrote the registry")
	}
	if _, after, err := reg.Update("alpha", true); err != nil || after.Ref != "" || after.Description != "v2" {
		t.Fatalf("latest update: %+v %v", after, err)
	}
}

func TestUpdateWithPrunedCacheAndRename(t *testing.T) {
	reg := newReg(t)
	repo := fixtureRepo(t)
	first, err := reg.Install(InstallOptions{URL: repo, Name: "alpha", As: "np-alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(reg.SrcDir("np-alpha", first.Commit)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "mv", "plugins/p/skills/alpha", "skills/alpha")
	commitAll(t, repo, "move")
	plan, after, err := reg.Update("np-alpha", false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != Moved || after.Name != "np-alpha" || after.Upstream != "alpha" || after.Path != "skills/alpha" {
		t.Fatalf("plan %+v after %+v", plan, after)
	}
}
