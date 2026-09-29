package skillreg

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
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

func TestSyncBundledInstallsIdempotentlyAndRemoves(t *testing.T) {
	reg := newReg(t)
	fsys := fstest.MapFS{
		"SKILL.md":           {Data: []byte("---\nname: gamma\ndescription: g\n---\nbody\n")},
		"scripts/run.sh":     {Data: []byte("#!/bin/sh\n")},
		"agents/openai.yaml": {Data: []byte("interface:\n  display_name: G\n")},
	}
	b := []Bundled{{Name: "gamma", URL: "u", Commit: "c", FS: fsys}}
	changed, _, err := SyncBundled(reg.Targets, b)
	if err != nil || len(changed) != 2 { // claude + codex; gemini gets no explicit-only copy
		t.Fatalf("first sync: %v %v", changed, err)
	}
	claude := filepath.Join(reg.Targets[0].Dir, "gamma")
	if data, _ := os.ReadFile(filepath.Join(claude, "SKILL.md")); !strings.Contains(string(data), "disable-model-invocation: true") {
		t.Errorf("claude copy not explicit:\n%s", data)
	}
	if fi, err := os.Stat(filepath.Join(claude, "scripts/run.sh")); err != nil || fi.Mode()&0o100 == 0 {
		t.Errorf("script not executable: %v", err)
	}
	if !IsBundled(claude) {
		t.Error("no bundled marker")
	}
	if changed, _, err := SyncBundled(reg.Targets, b); err != nil || len(changed) != 0 {
		t.Fatalf("second sync not idempotent: %v %v", changed, err)
	}
	// A registry install of the same name is refused, and a foreign dir is left alone.
	if err := checkTarget(claude); err == nil {
		t.Error("registry could overwrite a bundled copy")
	}
	foreign := filepath.Join(reg.Targets[1].Dir, "delta")
	writeFile(t, filepath.Join(foreign, "SKILL.md"), "mine")
	_, notes, err := SyncBundled(reg.Targets, []Bundled{{Name: "delta", FS: fsys}})
	if err != nil || len(notes) != 1 {
		t.Fatalf("foreign dir: %v %v", notes, err)
	}
	if data, _ := os.ReadFile(filepath.Join(foreign, "SKILL.md")); string(data) != "mine" {
		t.Error("foreign dir overwritten")
	}
	if _, err := os.Stat(claude); err == nil {
		t.Error("unlisted bundled skill not removed")
	}
}

func TestVendorCopiesSkillsAndLicenseAndReportsGone(t *testing.T) {
	repo := fixtureRepo(t)
	dir := filepath.Join(t.TempDir(), "vendor")
	src := VendorSource{URL: repo, Dir: dir, Skills: []string{"alpha"}}
	if _, err := Vendor(src); err == nil || !strings.Contains(err.Error(), "LICENSE") {
		t.Fatalf("vendored without license: %v", err)
	}
	writeFile(t, filepath.Join(repo, "LICENSE"), "MIT\n")
	commitAll(t, repo, "license")
	rep, err := Vendor(src)
	if err != nil || rep.Skills[0].Status != "new" || !slices.Contains(rep.NotBundled, "beta") {
		t.Fatalf("first vendor: %+v %v", rep, err)
	}
	for _, f := range []string{"LICENSE", "alpha/SKILL.md", "alpha/scripts/run.sh"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	if rep, err := Vendor(src); err != nil || rep.Skills[0].Status != Unchanged {
		t.Fatalf("re-vendor: %+v %v", rep, err)
	}
	run(t, repo, "rm", "-rq", "plugins/p/skills/alpha")
	commitAll(t, repo, "drop alpha")
	src.Skills = []string{"alpha", "beta"}
	writeFile(t, filepath.Join(dir, "stale", "SKILL.md"), "x")
	writeFile(t, filepath.Join(dir, "notes", "README.md"), "keep")
	rep, err = Vendor(src)
	if err != nil || rep.Skills[0].Status != Gone || rep.Skills[1].Status != "new" || !slices.Equal(rep.Removed, []string{"stale"}) {
		t.Fatalf("gone: %+v %v", rep, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "alpha", "SKILL.md")); err != nil {
		t.Error("gone skill's vendored copy was deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, "notes", "README.md")); err != nil {
		t.Error("vendor removed a dir that holds no skill")
	}
}

func TestVendorRejectsUnsafeNames(t *testing.T) {
	dir := t.TempDir()
	if _, err := Vendor(VendorSource{URL: "unused", Dir: dir, Skills: []string{"../x"}}); err == nil {
		t.Fatal("unsafe name accepted")
	}
}

func TestSyncBundledSkipsRegistryInstall(t *testing.T) {
	reg := newReg(t)
	if _, err := reg.Install(InstallOptions{URL: fixtureRepo(t), Name: "alpha"}); err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{"SKILL.md": {Data: []byte("---\nname: alpha\n---\n")}}
	writeFile(t, filepath.Join(reg.Targets[0].Dir, ".x.harnez-staging", "f"), "left over")
	changed, notes, err := SyncBundled(reg.Targets, []Bundled{{Name: "alpha", FS: fsys}})
	if err != nil || len(changed) != 0 || len(notes) != 2 || !strings.Contains(notes[0], "harnez skill remove alpha") {
		t.Fatalf("changed %v notes %v err %v", changed, notes, err)
	}
	if IsBundled(filepath.Join(reg.Targets[0].Dir, "alpha")) {
		t.Error("registry install overwritten")
	}
	if _, err := os.Stat(filepath.Join(reg.Targets[0].Dir, ".x.harnez-staging")); err == nil {
		t.Error("stale staging dir kept")
	}
}
