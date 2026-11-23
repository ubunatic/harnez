package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pullFixture is a bare origin, a work repository that pushes to it, and a
// clone cloned before the teammate's push, with hooks installed by harnez init.
type pullFixture struct {
	t      *testing.T
	bin    string
	env    []string
	origin string
	work   string
	clone  string
}

func newPullFixture(t *testing.T) *pullFixture {
	t.Helper()
	binDir := t.TempDir()
	bin := buildUsageTestBinary(t)
	if err := os.Symlink(bin, filepath.Join(binDir, "harnez")); err != nil {
		t.Fatalf("symlink shared harnez binary: %v", err)
	}
	f := &pullFixture{t: t, bin: bin, env: e2eEnv(t.TempDir(), binDir)}

	f.origin = t.TempDir()
	gitRunE2E(t, f.origin, f.env, "init", "-q", "--bare", "-b", "main")

	f.work = t.TempDir()
	gitRunE2E(t, f.work, f.env, "init", "-q", "-b", "main")
	f.setIdentity(f.work)
	writeMergeFixture(t, f.work, "go.mod", "module fixture\n\ngo 1.24\n")
	writeMergeFixture(t, f.work, "AGENTS.md", "# Fixture\n")
	writeMergeFixture(t, f.work, "issues/001-base.md", "# 001 — base\n\n**Status**: Open\n")
	writeMergeFixture(t, f.work, "issues/README.md", "<!-- managed by harnez index -->\n# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n")
	writeMergeFixture(t, f.work, "docs/README.md", "<!-- managed by harnez index: generated sections are overwritten; only the studies table is generated; other docs tables and prose are hand-written -->\n# Docs\n\nHandwritten preface.\n\n**`docs/studies/`** — case studies\n\n| File | Topic |\n|------|-------|\n")
	writeMergeFixture(t, f.work, "docs/studies/2026-01-01-a.md", "# Study: A\n\n**Scope**: base study\n")
	f.harnez(f.work, "init", "-d", ".")
	gitRunE2E(t, f.work, f.env, "add", "-A")
	f.harnez(f.work, "index", "-d", ".")
	gitRunE2E(t, f.work, f.env, "add", "-A")
	gitRunE2E(t, f.work, f.env, "commit", "-q", "-m", "docs: base")
	gitRunE2E(t, f.work, f.env, "remote", "add", "origin", f.origin)
	gitRunE2E(t, f.work, f.env, "push", "-q", "origin", "main")

	// The clone is taken before the teammate's push, so the later pull is a
	// real fast-forward.
	f.clone = filepath.Join(t.TempDir(), "clone")
	gitRunE2E(t, filepath.Dir(f.clone), f.env, "clone", "-q", f.origin, f.clone)
	f.setIdentity(f.clone)
	f.harnez(f.clone, "init", "-d", ".")
	if _, err := os.Stat(filepath.Join(f.clone, ".git", "hooks", "post-merge")); err != nil {
		t.Fatalf("init did not install the post-merge hook; the pull would not exercise it: %v", err)
	}
	return f
}

func (f *pullFixture) setIdentity(dir string) {
	f.t.Helper()
	gitRunE2E(f.t, dir, f.env, "config", "user.name", "Harnez E2E")
	gitRunE2E(f.t, dir, f.env, "config", "user.email", "harnez-e2e@example.invalid")
}

// harnez runs the fixture binary in dir and returns its combined output.
func (f *pullFixture) harnez(dir string, args ...string) string {
	f.t.Helper()
	cmd := exec.Command(f.bin, args...)
	cmd.Dir = dir
	cmd.Env = f.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("harnez %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// teammateStudy adds a study in the work repository and pushes it. With
// regenerate the generated index is rebuilt and staged in the same commit, as
// the pre-commit lint requires. Without it the commit is stale on purpose and
// needs --no-verify.
func (f *pullFixture) teammateStudy(name, body string, regenerate bool) {
	f.t.Helper()
	writeMergeFixture(f.t, f.work, "docs/studies/"+name, body)
	gitRunE2E(f.t, f.work, f.env, "add", "-A")
	if regenerate {
		f.harnez(f.work, "index", "-d", ".")
		gitRunE2E(f.t, f.work, f.env, "add", "-A")
		gitRunE2E(f.t, f.work, f.env, "commit", "-q", "-m", "docs(studies): add "+name)
	} else {
		gitRunE2E(f.t, f.work, f.env, "commit", "-q", "--no-verify", "-m", "docs(studies): add "+name)
	}
	gitRunE2E(f.t, f.work, f.env, "push", "-q", "origin", "main")
}

// pullFastForward fetches and fast-forwards the clone by exactly one commit.
func (f *pullFixture) pullFastForward() {
	f.t.Helper()
	gitRunE2E(f.t, f.clone, f.env, "fetch", "-q", "origin")
	if behind := gitRunE2E(f.t, f.clone, f.env, "rev-list", "--count", "HEAD..origin/main"); behind != "1" {
		f.t.Fatalf("clone is %s commits behind origin/main, want a single fast-forward commit", behind)
	}
	gitRunE2E(f.t, f.clone, f.env, "pull", "-q", "--ff-only")
}

// issueOnlyCommit appends to an issue, stages it and commits, returning the
// combined output and error of git commit.
func (f *pullFixture) issueOnlyCommit(dir, message string) (string, error) {
	f.t.Helper()
	appendE2E(f.t, filepath.Join(dir, "issues", "001-base.md"), "\nFollow-up note.\n")
	gitRunE2E(f.t, dir, f.env, "add", "issues/001-base.md")
	cmd := exec.Command("git", "commit", "-q", "-m", message)
	cmd.Dir = dir
	cmd.Env = f.env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestFastForwardPullKeepsStudiesIndexClean reproduces issue 1045: after a
// fast-forward pull in a clone with an untracked study, an untracked dotfile
// and a user-edited tracked file, the post-merge hook must leave docs/README.md
// clean, the user edit must stay unstaged, and an issue-only commit must pass
// the pre-commit lint.
func TestFastForwardPullKeepsStudiesIndexClean(t *testing.T) {
	skipShort(t, "builds the CLI and performs real Git pulls")
	f := newPullFixture(t)
	clone := f.clone

	f.teammateStudy("2026-01-02-new.md", "# Study: New\n\n**Scope**: pulled study\n", true)
	writeMergeFixture(t, clone, "docs/studies/.story-1121-facts.md", "# Study: Facts\n\n**Scope**: private notes\n")
	writeMergeFixture(t, clone, "docs/studies/2026-01-03-draft.md", "# Study: Draft\n\n**Scope**: unpublished draft\n")
	appendE2E(t, filepath.Join(clone, "AGENTS.md"), "local note\n")
	f.pullFastForward()

	if status := gitRunE2E(t, clone, f.env, "status", "--porcelain", "--", "docs/README.md"); status != "" {
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
	const row = "| [studies/2026-01-02-new.md](studies/2026-01-02-new.md) | pulled study |"
	if !strings.Contains(string(readme), row) {
		t.Errorf("pulled study row %q missing from docs/README.md:\n%s", row, readme)
	}
	if got := gitRunE2E(t, clone, f.env, "diff", "--cached", "--name-only"); got != "" {
		t.Fatalf("pull staged files: %s", got)
	}
	agents, err := os.ReadFile(filepath.Join(clone, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agents), "local note") {
		t.Fatal("user edit to AGENTS.md was lost by the pull")
	}

	if out, err := f.issueOnlyCommit(clone, "docs(issues): add follow-up note"); err != nil {
		t.Fatalf("issue-only commit failed the pre-commit lint: %v\n%s", err, out)
	}
	if got := gitRunE2E(t, clone, f.env, "diff", "--cached", "--name-only"); got != "" {
		t.Fatalf("issue-only commit left staged files: %s", got)
	}
	if got := gitRunE2E(t, clone, f.env, "diff", "--name-only"); got != "AGENTS.md" {
		t.Fatalf("unstaged tracked changes after commit = %q, want only the user's AGENTS.md edit", got)
	}
}

// TestFastForwardPullStaleReadmeNeedsStageHint is M2 scenario (b): the pulled
// commit's docs/README.md is stale, so post-merge regenerates it unstaged. The
// issue-only commit must fail with an actionable hint, and the hinted command
// must stage only docs/README.md while the user's edit stays unstaged.
func TestFastForwardPullStaleReadmeNeedsStageHint(t *testing.T) {
	skipShort(t, "builds the CLI and performs real Git pulls")
	f := newPullFixture(t)
	clone := f.clone

	f.teammateStudy("2026-01-02-new.md", "# Study: New\n\n**Scope**: pulled study\n", false)
	writeMergeFixture(t, clone, "docs/studies/.story-1121-facts.md", "# Study: Facts\n\n**Scope**: private notes\n")
	writeMergeFixture(t, clone, "docs/studies/2026-01-03-draft.md", "# Study: Draft\n\n**Scope**: unpublished draft\n")
	appendE2E(t, filepath.Join(clone, "AGENTS.md"), "local note\n")
	f.pullFastForward()

	if status := gitRunE2E(t, clone, f.env, "status", "--porcelain", "--", "docs/README.md"); status != "M docs/README.md" {
		t.Fatalf("post-merge status for docs/README.md = %q, want unstaged regeneration", status)
	}

	out, err := f.issueOnlyCommit(clone, "docs(issues): add follow-up note")
	if err == nil {
		t.Fatalf("issue-only commit passed with a stale staged docs/README.md:\n%s", out)
	}
	for _, want := range []string{"docs/README.md is generated studies index drift", "run 'harnez index --stage'", "docs/studies/2026-01-03-draft.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("lint output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, ".story-1121-facts.md") {
		t.Errorf("dotfile named in the untracked-study hint:\n%s", out)
	}

	f.harnez(clone, "index", "--stage", "-d", ".")
	const staged = "docs/README.md\nissues/001-base.md"
	if got := gitRunE2E(t, clone, f.env, "diff", "--cached", "--name-only"); got != staged {
		t.Fatalf("after index --stage, staged = %q, want %q", got, strings.ReplaceAll(staged, "\n", " "))
	}
	gitRunE2E(t, clone, f.env, "commit", "-q", "-m", "docs(issues): add follow-up note")

	if got := gitRunE2E(t, clone, f.env, "diff", "--name-only"); got != "AGENTS.md" {
		t.Fatalf("unstaged changes after commit = %q, want only the user's AGENTS.md edit", got)
	}
	if got := gitRunE2E(t, clone, f.env, "status", "--porcelain", "--", "AGENTS.md"); got != "M AGENTS.md" {
		t.Fatalf("user edit to AGENTS.md = %q, want it still unstaged", got)
	}
	if got := gitRunE2E(t, clone, f.env, "show", "HEAD:docs/README.md"); !strings.Contains(got, "| [studies/2026-01-02-new.md](studies/2026-01-02-new.md) | pulled study |") {
		t.Errorf("committed docs/README.md lacks the pulled study row:\n%s", got)
	}
}

// TestLinkedWorktreePullAndIssueCommitKeepStudyRows covers pre-work item 1 in
// a linked worktree, where hooks run with an absolute GIT_DIR: the
// fast-forward pull keeps the study rows and an issue-only commit passes the
// pre-commit lint.
func TestLinkedWorktreePullAndIssueCommitKeepStudyRows(t *testing.T) {
	skipShort(t, "builds the CLI and performs real Git pulls")
	f := newPullFixture(t)
	wt := filepath.Join(t.TempDir(), "wt")
	gitRunE2E(t, f.clone, f.env, "worktree", "add", "-q", "-b", "topic", wt, "main")

	f.teammateStudy("2026-01-02-new.md", "# Study: New\n\n**Scope**: pulled study\n", true)
	gitRunE2E(t, wt, f.env, "fetch", "-q", "origin")
	gitRunE2E(t, wt, f.env, "pull", "-q", "--ff-only", "origin", "main")

	if status := gitRunE2E(t, wt, f.env, "status", "--porcelain", "--", "docs/README.md"); status != "" {
		readme, _ := os.ReadFile(filepath.Join(wt, "docs", "README.md"))
		t.Fatalf("worktree post-merge left docs/README.md drifted: %s\n%s", status, readme)
	}
	readme, err := os.ReadFile(filepath.Join(wt, "docs", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	const row = "| [studies/2026-01-02-new.md](studies/2026-01-02-new.md) | pulled study |"
	if !strings.Contains(string(readme), row) {
		t.Fatalf("worktree lost the pulled study row %q:\n%s", row, readme)
	}

	if out, err := f.issueOnlyCommit(wt, "docs(issues): add follow-up note"); err != nil {
		t.Fatalf("issue-only commit in linked worktree failed the pre-commit lint: %v\n%s", err, out)
	}
}

// TestIndexStageLeavesHandEditUnstaged is M3 item 1: --stage stages a differing
// index file it did not rewrite only when regenerating its staged content gives
// the worktree content. A hand-written prose edit to docs/README.md must stay
// unstaged, with a warning, so the next commit does not carry it silently.
func TestIndexStageLeavesHandEditUnstaged(t *testing.T) {
	skipShort(t, "builds the CLI and performs real Git commits")
	f := newPullFixture(t)
	clone := f.clone
	appendE2E(t, filepath.Join(clone, "docs", "README.md"), "\nHand-written note.\n")

	out := f.harnez(clone, "index", "--stage", "-d", ".")
	if !strings.Contains(out, "index --stage: docs/README.md has unstaged hand edits; not staged") {
		t.Errorf("--stage output lacks the hand-edit warning:\n%s", out)
	}
	if got := gitRunE2E(t, clone, f.env, "diff", "--cached", "--name-only"); got != "" {
		t.Fatalf("--stage staged %q, want nothing: the hand edit must stay unstaged", got)
	}
	if got := gitRunE2E(t, clone, f.env, "diff", "--name-only"); got != "docs/README.md" {
		t.Fatalf("unstaged files after --stage = %q, want docs/README.md", got)
	}
}

// TestIndexStageKeepsHandEditOutOfRewrittenReadme is M4 item 1: a file this run
// rewrote meets the same rule as one it did not. A prose hand edit in
// docs/README.md stays unstaged even though a newly staged study rewrites its
// table, and the warning names the file.
func TestIndexStageKeepsHandEditOutOfRewrittenReadme(t *testing.T) {
	skipShort(t, "builds the CLI and performs real Git commits")
	f := newPullFixture(t)
	clone := f.clone
	appendE2E(t, filepath.Join(clone, "docs", "README.md"), "\nHand-written note.\n")
	writeMergeFixture(t, clone, "docs/studies/2026-01-04-new.md", "# Study: New\n\n**Scope**: staged study\n")
	gitRunE2E(t, clone, f.env, "add", "docs/studies/2026-01-04-new.md")

	out := f.harnez(clone, "index", "--stage", "-d", ".")
	if !strings.Contains(out, "index --stage: docs/README.md has unstaged hand edits; not staged") {
		t.Errorf("--stage output lacks the hand-edit warning for the rewritten README:\n%s", out)
	}
	if got := gitRunE2E(t, clone, f.env, "diff", "--cached", "--name-only"); got != "docs/studies/2026-01-04-new.md" {
		t.Fatalf("--stage staged %q, want only the new study", got)
	}
	if got := gitRunE2E(t, clone, f.env, "diff", "--name-only"); got != "docs/README.md" {
		t.Fatalf("unstaged files after --stage = %q, want docs/README.md", got)
	}
}

// TestIndexStageStagesPostMergeDriftThenCommitsClean is M4 item 2: a README that
// post-merge regenerated unstaged is staged with no hand-edit warning, and after
// --stage and a commit only the user's own AGENTS.md edit stays unstaged.
func TestIndexStageStagesPostMergeDriftThenCommitsClean(t *testing.T) {
	skipShort(t, "builds the CLI and performs real Git pulls")
	f := newPullFixture(t)
	clone := f.clone
	f.teammateStudy("2026-01-02-new.md", "# Study: New\n\n**Scope**: pulled study\n", false)
	appendE2E(t, filepath.Join(clone, "AGENTS.md"), "local note\n")
	f.pullFastForward()
	if status := gitRunE2E(t, clone, f.env, "status", "--porcelain", "--", "docs/README.md"); status != "M docs/README.md" {
		t.Fatalf("post-merge status for docs/README.md = %q, want unstaged regeneration", status)
	}

	out := f.harnez(clone, "index", "--stage", "-d", ".")
	if strings.Contains(out, "hand edits") {
		t.Errorf("--stage warned about hand edits in post-merge drift:\n%s", out)
	}
	if !strings.Contains(out, "staged docs/README.md") {
		t.Errorf("--stage did not report the regenerated README as staged:\n%s", out)
	}
	if got := gitRunE2E(t, clone, f.env, "diff", "--cached", "--name-only"); got != "docs/README.md" {
		t.Fatalf("after index --stage, staged = %q, want docs/README.md", got)
	}
	gitRunE2E(t, clone, f.env, "commit", "-q", "-m", "docs(issues): add follow-up note")

	if got := gitRunE2E(t, clone, f.env, "status", "--porcelain"); got != "M AGENTS.md" {
		t.Fatalf("git status after commit = %q, want only the user's AGENTS.md edit", got)
	}
}

// TestIndexStageSkipsUnmergedReadme is M4 item 3: a conflicted index file has no
// stage-0 entry. --stage must say it is unmerged, must not stage it (which would
// silently resolve the conflict), and must leave both conflict stages in place.
func TestIndexStageSkipsUnmergedReadme(t *testing.T) {
	skipShort(t, "builds the CLI and performs real Git commits")
	f := newPullFixture(t)
	clone := f.clone
	base := gitRunE2E(t, clone, f.env, "rev-parse", "HEAD:docs/README.md")
	gitRunE2E(t, clone, f.env, "update-index", "--force-remove", "--", "docs/README.md")
	info := exec.Command("git", "update-index", "--index-info")
	info.Dir = clone
	info.Env = f.env
	info.Stdin = strings.NewReader(fmt.Sprintf("100644 %s 1\tdocs/README.md\n100644 %s 2\tdocs/README.md\n", base, base))
	if out, err := info.CombinedOutput(); err != nil {
		t.Fatalf("git update-index --index-info: %v\n%s", err, out)
	}

	out := f.harnez(clone, "index", "--stage", "-d", ".")
	if !strings.Contains(out, "index --stage: docs/README.md is unmerged; not staged") {
		t.Errorf("--stage output lacks the unmerged warning:\n%s", out)
	}
	if strings.Contains(out, "hand edits") {
		t.Errorf("unmerged README reported as a hand edit:\n%s", out)
	}
	unmerged := gitRunE2E(t, clone, f.env, "ls-files", "--unmerged", "--", "docs/README.md")
	if strings.Count(unmerged, "\n")+1 != 2 {
		t.Fatalf("--stage changed the conflict; ls-files --unmerged = %q, want both stages 1 and 2", unmerged)
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
