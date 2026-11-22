package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestFastForwardPullKeepsStudiesIndexClean reproduces issue 1045: after a
// fast-forward pull in a clone with an untracked study, an untracked dotfile
// and a user-edited tracked file, the post-merge hook must leave docs/README.md
// clean, the user edit must stay unstaged, and an issue-only commit must pass
// the pre-commit lint.
func TestFastForwardPullKeepsStudiesIndexClean(t *testing.T) {
	skipShort(t, "builds the CLI and performs real Git pulls")

	binDir := t.TempDir()
	bin := buildUsageTestBinary(t)
	if err := os.Symlink(bin, filepath.Join(binDir, "harnez")); err != nil {
		t.Fatalf("symlink shared harnez binary: %v", err)
	}
	home := t.TempDir()
	env := e2eEnv(home, binDir)

	harnezIn := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("harnez %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	setIdentity := func(dir string) {
		t.Helper()
		gitRunE2E(t, dir, nil, "config", "user.name", "Harnez E2E")
		gitRunE2E(t, dir, nil, "config", "user.email", "harnez-e2e@example.invalid")
	}

	origin := t.TempDir()
	gitRunE2E(t, origin, nil, "init", "-q", "--bare", "-b", "main")

	work := t.TempDir()
	gitRunE2E(t, work, nil, "init", "-q", "-b", "main")
	setIdentity(work)
	writeMergeFixture(t, work, "go.mod", "module fixture\n\ngo 1.24\n")
	writeMergeFixture(t, work, "AGENTS.md", "# Fixture\n")
	writeMergeFixture(t, work, "issues/001-base.md", "# 001 — base\n\n**Status**: Open\n")
	writeMergeFixture(t, work, "issues/README.md", "<!-- managed by harnez index -->\n# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n")
	writeMergeFixture(t, work, "docs/README.md", "<!-- managed by harnez index: generated sections are overwritten; only the studies table is generated; other docs tables and prose are hand-written -->\n# Docs\n\nHandwritten preface.\n\n**`docs/studies/`** — case studies\n\n| File | Topic |\n|------|-------|\n")
	writeMergeFixture(t, work, "docs/studies/2026-01-01-a.md", "# Study: A\n\n**Scope**: base study\n")
	harnezIn(work, "init", "-d", ".")
	gitRunE2E(t, work, env, "add", "-A")
	harnezIn(work, "index", "-d", ".")
	gitRunE2E(t, work, env, "add", "-A")
	gitRunE2E(t, work, env, "commit", "-q", "-m", "docs: base")
	gitRunE2E(t, work, nil, "remote", "add", "origin", origin)
	gitRunE2E(t, work, nil, "push", "-q", "origin", "main")

	// The pulling clone is cloned before the teammate's push, so the pull below
	// is a real fast-forward. It has the hooks installed and local untracked notes.
	clone := filepath.Join(t.TempDir(), "clone")
	gitRunE2E(t, filepath.Dir(clone), nil, "clone", "-q", origin, clone)
	setIdentity(clone)
	harnezIn(clone, "init", "-d", ".")
	if _, err := os.Stat(filepath.Join(clone, ".git", "hooks", "post-merge")); err != nil {
		t.Fatalf("init did not install the post-merge hook; the pull would not exercise it: %v", err)
	}

	// A teammate pushes a new study with its generated index.
	writeMergeFixture(t, work, "docs/studies/2026-01-02-new.md", "# Study: New\n\n**Scope**: pulled study\n")
	gitRunE2E(t, work, env, "add", "-A")
	harnezIn(work, "index", "-d", ".")
	gitRunE2E(t, work, env, "add", "-A")
	gitRunE2E(t, work, env, "commit", "-q", "-m", "docs(studies): add new study")
	gitRunE2E(t, work, nil, "push", "-q", "origin", "main")

	writeMergeFixture(t, clone, "docs/studies/.story-1121-facts.md", "# Study: Facts\n\n**Scope**: private notes\n")
	writeMergeFixture(t, clone, "docs/studies/2026-01-03-draft.md", "# Study: Draft\n\n**Scope**: unpublished draft\n")
	appendE2E(t, filepath.Join(clone, "AGENTS.md"), "local note\n")

	gitRunE2E(t, clone, nil, "fetch", "-q", "origin")
	if behind := gitRunE2E(t, clone, nil, "rev-list", "--count", "HEAD..origin/main"); behind != "1" {
		t.Fatalf("clone is %s commits behind origin/main, want a single fast-forward commit", behind)
	}
	gitRunE2E(t, clone, env, "pull", "-q", "--ff-only")

	if status := gitRunE2E(t, clone, nil, "status", "--porcelain", "--", "docs/README.md"); status != "" {
		readme, _ := os.ReadFile(filepath.Join(clone, "docs", "README.md"))
		t.Fatalf("post-merge left docs/README.md drifted: %s\n%s", status, readme)
	}
	readme, err := os.ReadFile(filepath.Join(clone, "docs", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".story-1121-facts.md", "2026-01-03-draft.md"} {
		if strings.Contains(string(readme), name) {
			t.Errorf("untracked study %s reached docs/README.md", name)
		}
	}
	if !strings.Contains(string(readme), "2026-01-02-new.md") {
		t.Errorf("pulled study missing from docs/README.md:\n%s", readme)
	}
	if got := gitRunE2E(t, clone, nil, "diff", "--cached", "--name-only"); got != "" {
		t.Fatalf("pull staged files: %s", got)
	}
	agents, err := os.ReadFile(filepath.Join(clone, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agents), "local note") {
		t.Fatal("user edit to AGENTS.md was lost by the pull")
	}

	appendE2E(t, filepath.Join(clone, "issues", "001-base.md"), "\nFollow-up note.\n")
	gitRunE2E(t, clone, nil, "add", "issues/001-base.md")
	gitRunE2E(t, clone, env, "commit", "-q", "-m", "docs(issues): add follow-up note")

	if got := gitRunE2E(t, clone, nil, "diff", "--cached", "--name-only"); got != "" {
		t.Fatalf("issue-only commit left staged files: %s", got)
	}
	if got := gitRunE2E(t, clone, nil, "diff", "--name-only"); got != "AGENTS.md" {
		t.Fatalf("unstaged tracked changes after commit = %q, want only the user's AGENTS.md edit", got)
	}
}

func appendE2E(t *testing.T, path, text string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte(text)...), 0o644); err != nil {
		t.Fatal(err)
	}
}
