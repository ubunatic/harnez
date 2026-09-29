package skillreg

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureRepo builds a git repo shaped like a Claude Code plugin marketplace
// with two skills, a hook dir, an MCP file, a doctor script, and .env.example.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	files := map[string]string{
		"plugins/p/.claude-plugin/plugin.json":      `{"name":"p"}`,
		"plugins/p/hooks/hooks.json":                `{}`,
		"plugins/p/.mcp.json":                       `{}`,
		"plugins/p/skills/alpha/SKILL.md":           "---\nname: alpha\ndescription: >\n  Build scroll pages.\nallowed-tools: Bash, Read\n---\n# alpha\n",
		"plugins/p/skills/alpha/scripts/doctor.mjs": "// check\n",
		"plugins/p/skills/alpha/scripts/run.sh":     "#!/bin/sh\n",
		"plugins/p/skills/beta/SKILL.md":            "---\nname: beta\ndescription: Grill the user about a plan.\n---\n",
		".env.example":                              "# comment\nAPI_KEY=x\n",
	}
	for p, c := range files {
		full := filepath.Join(repo, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(p, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(full, []byte(c), mode); err != nil {
			t.Fatal(err)
		}
	}
	run(t, repo, "init", "-q", "-b", "main")
	run(t, repo, "add", ".")
	run(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "init")
	return repo
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func newReg(t *testing.T) *Registry {
	root := t.TempDir()
	return &Registry{
		Root: filepath.Join(root, "reg"),
		Targets: []Target{
			{Agent: "claude", Dir: filepath.Join(root, "claude")},
			{Agent: "codex", Dir: filepath.Join(root, "codex")},
			{Agent: "gemini", Dir: filepath.Join(root, "gemini")},
		},
	}
}

func TestParseSource(t *testing.T) {
	for in, want := range map[string][2]string{
		"https://github.com/a/b":      {"https://github.com/a/b", ""},
		"https://github.com/a/b@v1.2": {"https://github.com/a/b", "v1.2"},
		"git@github.com:a/b":          {"git@github.com:a/b", ""},
		"git@github.com:a/b@abc123":   {"git@github.com:a/b", "abc123"},
		"/tmp/x@main":                 {"/tmp/x", "main"},
	} {
		url, ref := ParseSource(in)
		if url != want[0] || ref != want[1] {
			t.Errorf("ParseSource(%q) = %q, %q; want %q, %q", in, url, ref, want[0], want[1])
		}
	}
}

func TestInstallNeedsSelectorForMultiSkillRepo(t *testing.T) {
	reg := newReg(t)
	_, err := reg.Install(InstallOptions{URL: fixtureRepo(t)})
	if err == nil || !strings.Contains(err.Error(), "several skills") {
		t.Fatalf("want several-skills error, got %v", err)
	}
}

func TestInstallCopiesOnlySkillDirToAllTargets(t *testing.T) {
	reg := newReg(t)
	e, err := reg.Install(InstallOptions{URL: fixtureRepo(t), Name: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Path != "plugins/p/skills/alpha" || len(e.Commit) != 40 {
		t.Fatalf("unexpected entry %+v", e)
	}
	for _, target := range reg.Targets[:2] {
		dir := filepath.Join(target.Dir, "alpha")
		for _, f := range []string{"SKILL.md", "scripts/doctor.mjs", MarkerFile} {
			if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
				t.Errorf("%s missing: %v", f, err)
			}
		}
		info, err := os.Stat(filepath.Join(dir, "scripts/run.sh"))
		if err != nil || info.Mode().Perm()&0o100 == 0 {
			t.Errorf("run.sh lost its exec bit: %v %v", info, err)
		}
		if _, err := os.Stat(filepath.Join(target.Dir, "beta")); err == nil {
			t.Error("sibling skill beta was installed")
		}
		if _, err := os.Stat(filepath.Join(dir, "hooks")); err == nil {
			t.Error("plugin hooks were installed")
		}
	}
	got, ok, err := reg.Get("alpha")
	if err != nil || !ok || got.Commit != e.Commit {
		t.Fatalf("registry entry: %+v %v %v", got, ok, err)
	}
}

func TestInstallRefusesForeignSkill(t *testing.T) {
	reg := newReg(t)
	foreign := filepath.Join(reg.Targets[1].Dir, "alpha")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Install(InstallOptions{URL: fixtureRepo(t), Name: "alpha"}); err == nil {
		t.Fatal("overwrote a skill without marker")
	}
	if _, err := os.Stat(filepath.Join(reg.Targets[0].Dir, "alpha")); err == nil {
		t.Error("partial install happened before the refusal")
	}
}

func TestUpdatePicksUpNewCommitAndDropsOldCache(t *testing.T) {
	reg := newReg(t)
	repo := fixtureRepo(t)
	first, err := reg.Install(InstallOptions{URL: repo, Name: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(repo, "plugins/p/skills/alpha/SKILL.md")
	if err := os.WriteFile(skill, []byte("---\nname: alpha\ndescription: v2\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qam", "v2")
	plan, after, err := reg.Update("alpha", false)
	if err != nil {
		t.Fatal(err)
	}
	before := plan.Entry
	if plan.Status != Changed || !strings.Contains(plan.DiffStat, "SKILL.md") {
		t.Errorf("plan: %+v", plan)
	}
	if before.Commit != first.Commit || after.Commit == first.Commit || after.Description != "v2" {
		t.Fatalf("update: before %+v after %+v", before, after)
	}
	if _, err := os.Stat(reg.SrcDir("alpha", first.Commit)); err == nil {
		t.Error("old cache kept")
	}
	data, _ := os.ReadFile(filepath.Join(reg.Targets[0].Dir, "alpha", "SKILL.md"))
	if !strings.Contains(string(data), "v2") {
		t.Error("installed copy not updated")
	}
}

func TestPinnedRefInstall(t *testing.T) {
	reg := newReg(t)
	repo := fixtureRepo(t)
	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	pinned := strings.TrimSpace(string(out))
	run(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "later")
	e, err := reg.Install(InstallOptions{URL: repo, Ref: pinned, Path: "plugins/p/skills/beta"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Commit != pinned || e.Name != "beta" {
		t.Fatalf("pinned install: %+v", e)
	}
}

func TestRemoveKeepsForeignCopies(t *testing.T) {
	reg := newReg(t)
	if _, err := reg.Install(InstallOptions{URL: fixtureRepo(t), Name: "alpha"}); err != nil {
		t.Fatal(err)
	}
	// Simulate a user replacing one copy by hand: no marker, must survive.
	foreign := filepath.Join(reg.Targets[1].Dir, "alpha")
	if err := os.Remove(filepath.Join(foreign, MarkerFile)); err != nil {
		t.Fatal(err)
	}
	if err := reg.Remove("alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(reg.Targets[0].Dir, "alpha")); err == nil {
		t.Error("marked copy not removed")
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Error("unmarked copy was removed")
	}
	if entries, _ := reg.Load(); len(entries) != 0 {
		t.Errorf("registry not emptied: %+v", entries)
	}
}

func TestSearchFindsInstalledAndSiblings(t *testing.T) {
	reg := newReg(t)
	if _, err := reg.Install(InstallOptions{URL: fixtureRepo(t), Name: "alpha"}); err != nil {
		t.Fatal(err)
	}
	hits, err := reg.Search("GRILL plan")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Skill.Name != "beta" || hits[0].Installed {
		t.Fatalf("search sibling: %+v", hits)
	}
	hits, _ = reg.Search("scroll")
	if len(hits) != 1 || !hits[0].Installed {
		t.Fatalf("search installed: %+v", hits)
	}
}

func TestExploreReportsExtrasWithoutRunning(t *testing.T) {
	rep, err := Explore(fixtureRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Skills) != 2 {
		t.Fatalf("skills: %+v", rep.Skills)
	}
	alpha := rep.Skills[0]
	if alpha.Name != "alpha" || alpha.Doctor != "scripts/doctor.mjs" || alpha.AllowedTools != "Bash, Read" || len(alpha.Scripts) != 2 {
		t.Errorf("alpha report: %+v", alpha)
	}
	extras := strings.Join(rep.Plugin, "\n")
	for _, want := range []string{"hooks", "MCP servers"} {
		if !strings.Contains(extras, want) {
			t.Errorf("plugin extras miss %q: %v", want, rep.Plugin)
		}
	}
	if len(rep.EnvKeys) != 1 || rep.EnvKeys[0] != "API_KEY" {
		t.Errorf("env keys: %v", rep.EnvKeys)
	}
}

// singleSkillRepo builds a repo with one SKILL.md at the root.
func singleSkillRepo(t *testing.T, skillMD string) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "init", "-q", "-b", "main")
	run(t, repo, "add", ".")
	run(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "init")
	return repo
}

func TestInstallRejectsUnsafeSkillName(t *testing.T) {
	for _, name := range []string{"../../escape", "a/b", "..", "-x", "Up"} {
		reg := newReg(t)
		repo := singleSkillRepo(t, "---\nname: "+name+"\ndescription: x\n---\n")
		if _, err := reg.Install(InstallOptions{URL: repo}); err == nil || !strings.Contains(err.Error(), "safe directory name") {
			t.Errorf("name %q: want unsafe-name error, got %v", name, err)
		}
		if _, err := os.Stat(filepath.Dir(reg.Root)); err == nil {
			entries, _ := os.ReadDir(filepath.Dir(reg.Root))
			for _, e := range entries {
				if e.Name() != "reg" && e.Name() != "claude" && e.Name() != "codex" && e.Name() != "gemini" {
					t.Errorf("name %q: wrote %s outside the targets", name, e.Name())
				}
			}
		}
	}
}

func TestCloneRejectsOptionLikeSource(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "x")
	for _, c := range [][2]string{{"--upload-pack=touch /tmp/pwn", ""}, {"/tmp/repo", "-f"}} {
		if _, err := Clone(c[0], c[1], dst); err == nil || !strings.Contains(err.Error(), "must not start") {
			t.Errorf("Clone(%q, %q): want refusal, got %v", c[0], c[1], err)
		}
	}
}

func TestRootLevelSkillDoesNotCopyGitDir(t *testing.T) {
	reg := newReg(t)
	repo := singleSkillRepo(t, "---\nname: rooty\ndescription: x\n---\n")
	if _, err := reg.Install(InstallOptions{URL: repo}); err != nil {
		t.Fatal(err)
	}
	for _, target := range reg.Targets[:2] {
		if _, err := os.Stat(filepath.Join(target.Dir, "rooty", ".git")); err == nil {
			t.Errorf("%s: .git copied into the skill", target)
		}
	}
}

func TestInstallLeavesNoStagingDirs(t *testing.T) {
	reg := newReg(t)
	if _, err := reg.Install(InstallOptions{URL: fixtureRepo(t), Name: "alpha"}); err != nil {
		t.Fatal(err)
	}
	for _, target := range reg.Targets {
		entries, _ := os.ReadDir(target.Dir)
		for _, e := range entries {
			if strings.Contains(e.Name(), "harnez-staging") {
				t.Errorf("%s: staging dir %s left behind", target, e.Name())
			}
		}
	}
}

func TestExplicitInstallDisablesImplicitUse(t *testing.T) {
	reg := newReg(t)
	if _, err := reg.Install(InstallOptions{URL: fixtureRepo(t), Name: "alpha"}); err != nil {
		t.Fatal(err)
	}
	claudeMD, err := os.ReadFile(filepath.Join(reg.Targets[0].Dir, "alpha", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := ReadSkill(filepath.Join(reg.Targets[0].Dir, "alpha", "SKILL.md"))
	if err != nil || s.Name != "alpha" || s.Description != "Build scroll pages." {
		t.Fatalf("frontmatter broken after edit: %+v %v\n%s", s, err, claudeMD)
	}
	if strings.Count(string(claudeMD), "disable-model-invocation: true") != 1 {
		t.Errorf("claude copy lacks the switch:\n%s", claudeMD)
	}
	codex, err := os.ReadFile(filepath.Join(reg.Targets[1].Dir, "alpha", "agents", "openai.yaml"))
	if err != nil || !strings.Contains(string(codex), "allow_implicit_invocation: false") {
		t.Errorf("codex policy missing: %v\n%s", err, codex)
	}
	if _, err := os.Stat(filepath.Join(reg.Targets[2].Dir, "alpha")); err == nil {
		t.Error("gemini got a copy of an explicit-only skill")
	}
}

func TestAutoInstallThenExplicitDropsGeminiCopy(t *testing.T) {
	reg := newReg(t)
	repo := fixtureRepo(t)
	if _, err := reg.Install(InstallOptions{URL: repo, Name: "alpha", Auto: true}); err != nil {
		t.Fatal(err)
	}
	gem := filepath.Join(reg.Targets[2].Dir, "alpha")
	if _, err := os.Stat(gem); err != nil {
		t.Fatalf("auto install skipped gemini: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(reg.Targets[0].Dir, "alpha", "SKILL.md"))
	if strings.Contains(string(data), "disable-model-invocation") {
		t.Error("auto install disabled model invocation")
	}
	if _, _, err := reg.Update("alpha", false); err != nil {
		t.Fatal(err)
	}
	if e, _, _ := reg.Get("alpha"); !e.Auto {
		t.Error("update lost the auto mode")
	}
	if _, err := reg.Install(InstallOptions{URL: repo, Name: "alpha"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gem); err == nil {
		t.Error("switch to explicit left the gemini copy")
	}
	data, _ = os.ReadFile(filepath.Join(reg.Targets[0].Dir, "alpha", "SKILL.md"))
	if !strings.Contains(string(data), "disable-model-invocation: true") {
		t.Error("switch to explicit did not disable model invocation in the claude copy")
	}
}

func TestDisableModelInvocation(t *testing.T) {
	for in, want := range map[string]string{
		"---\nname: a\ndisable-model-invocation: false\n---\nbody\n": "---\nname: a\ndisable-model-invocation: true\n---\nbody\n",
		"# no frontmatter\n": "---\ndisable-model-invocation: true\n---\n# no frontmatter\n",
		"---\r\nname: a\r\ndescription: d\r\n---\r\nbody\r\n": "---\nname: a\ndescription: d\ndisable-model-invocation: true\n---\nbody\n",
		"---\n---\nbody\n": "---\ndisable-model-invocation: true\n---\nbody\n",
		"---\nname: a\ndescription: x ---- y\n----\n---\n": "---\nname: a\ndescription: x ---- y\n----\ndisable-model-invocation: true\n---\n",
	} {
		if got := string(disableModelInvocation([]byte(in))); got != want {
			t.Errorf("disableModelInvocation(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExplicitCodexKeepsExistingOpenAIYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "agents", "openai.yaml")
	if err := os.WriteFile(path, []byte("interface:\n  display_name: X\npolicy:\n  allow_implicit_invocation: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := makeExplicit("codex", dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "display_name: X") || !strings.Contains(string(data), "allow_implicit_invocation: false") {
		t.Errorf("openai.yaml after makeExplicit:\n%s", data)
	}
}

func TestInstallAsRenamesAndUpdateKeepsIt(t *testing.T) {
	reg := newReg(t)
	repo := fixtureRepo(t)
	// Make alpha's body mention its own name, so the rename warns.
	skill := filepath.Join(repo, "plugins/p/skills/alpha/SKILL.md")
	if err := os.WriteFile(skill, []byte("---\nname: alpha\ndescription: d\n---\nSee alpha docs.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qam", "mention")
	e, err := reg.Install(InstallOptions{URL: repo, Name: "alpha", As: "np-alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "np-alpha" || e.Upstream != "alpha" || len(e.Warnings) != 1 || !strings.Contains(e.Warnings[0], "SKILL.md") {
		t.Fatalf("entry: %+v", e)
	}
	for _, target := range reg.Targets[:2] {
		s, err := ReadSkill(filepath.Join(target.Dir, "np-alpha", "SKILL.md"))
		if err != nil || s.Name != "np-alpha" {
			t.Errorf("%s: renamed frontmatter: %+v %v", target.Agent, s, err)
		}
		if _, err := os.Stat(filepath.Join(target.Dir, "alpha")); err == nil {
			t.Errorf("%s: upstream-named copy installed", target.Agent)
		}
	}
	if _, after, err := reg.Update("np-alpha", false); err != nil || after.Name != "np-alpha" || after.Upstream != "alpha" {
		t.Fatalf("update lost the rename: %+v %v", after, err)
	}
	if hits, _ := reg.Search("d"); len(hits) == 0 || !hits[0].Installed {
		t.Errorf("search after rename: %+v", hits)
	}
	if err := reg.Remove("np-alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(reg.Targets[0].Dir, "np-alpha")); err == nil {
		t.Error("renamed copy not removed")
	}
}

func TestInstallRefusesReservedAndTakenNames(t *testing.T) {
	reg := newReg(t)
	reg.Reserved = []string{"alpha"}
	repo := fixtureRepo(t)
	if _, err := reg.Install(InstallOptions{URL: repo, Name: "alpha"}); err == nil || !strings.Contains(err.Error(), "--as") {
		t.Fatalf("reserved name: want --as hint, got %v", err)
	}
	if _, err := reg.Install(InstallOptions{URL: repo, Name: "alpha", As: "x-alpha"}); err != nil {
		t.Fatalf("--as around reserved name: %v", err)
	}
	other := singleSkillRepo(t, "---\nname: x-alpha\ndescription: other\n---\n")
	if _, err := reg.Install(InstallOptions{URL: other}); err == nil || !strings.Contains(err.Error(), "already installed from") || !strings.Contains(err.Error(), "--as") {
		t.Fatalf("taken name: want already-installed with --as hint, got %v", err)
	}
}

func TestSetFrontmatterReplacesInPlace(t *testing.T) {
	got := string(setFrontmatter([]byte("---\nname: a\ndescription: d\n---\n"), "name", "b"))
	if want := "---\nname: b\ndescription: d\n---\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMentionsUsesWordBoundaries(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: \"alpha\"\n---\nalphabet soup\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "ref.md"), []byte("run /alpha now\n"), 0o644)
	if got := mentions(dir, "alpha"); len(got) != 1 || got[0] != "ref.md" {
		t.Errorf("mentions: %v", got)
	}
}
