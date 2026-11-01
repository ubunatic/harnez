package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type rebaseE2E struct {
	t    *testing.T
	root string
	env  []string
	bin  string
}

func (r rebaseE2E) cli(args ...string) (string, error) {
	r.t.Helper()
	cmd := exec.Command(r.bin, args...)
	cmd.Dir = r.root
	cmd.Env = r.env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (r rebaseE2E) commitAll(message string) {
	r.t.Helper()
	if out, err := r.cli("index", "-d", "."); err != nil {
		r.t.Fatalf("harnez index: %v\n%s", err, out)
	}
	gitRunE2E(r.t, r.root, r.env, "add", "-A")
	gitRunE2E(r.t, r.root, r.env, "commit", "-qm", message)
}

func (r rebaseE2E) ticket(name, title string) {
	r.t.Helper()
	writeMergeFixture(r.t, r.root, "issues/"+name, "# 1010 — "+title+"\n\n**Status**: Open\n")
	r.commitAll("file " + title)
}

func (r rebaseE2E) verifyRepaired(out string) {
	r.t.Helper()
	if _, err := os.Stat(filepath.Join(r.root, "issues", "1011-local.md")); err != nil {
		r.t.Fatalf("local ticket not renumbered: %v\n%s", err, out)
	}
	if status := gitRunE2E(r.t, r.root, r.env, "status", "--porcelain"); status != "" {
		r.t.Errorf("rebase left changes for a manual fix commit:\n%s", status)
	}
	if out, err := r.cli("issues", "lint", "-d", "."); err != nil {
		r.t.Errorf("issues lint after rebase: %v\n%s", err, out)
	}
	readme, err := os.ReadFile(filepath.Join(r.root, "issues", "README.md"))
	if err != nil {
		r.t.Fatal(err)
	}
	for _, want := range []string{"1011-local.md", "1010-upstream.md"} {
		if !strings.Contains(string(readme), want) {
			r.t.Errorf("index misses %s:\n%s", want, readme)
		}
	}
}

func setupRebaseE2E(t *testing.T) rebaseE2E {
	t.Helper()
	skipShort(t, "builds the CLI and performs a real Git rebase")
	binDir := t.TempDir()
	bin := buildUsageTestBinary(t)
	if err := os.Symlink(bin, filepath.Join(binDir, "harnez")); err != nil {
		t.Fatalf("symlink shared harnez binary: %v", err)
	}
	r := rebaseE2E{t: t, root: t.TempDir(), env: e2eEnv(t.TempDir(), binDir), bin: bin}
	if err := os.MkdirAll(filepath.Join(r.root, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeMergeFixture(t, r.root, "go.mod", "module fixture\n\ngo 1.24\n")
	writeMergeFixture(t, r.root, "AGENTS.md", "# Fixture\n")
	writeMergeFixture(t, r.root, "shared.txt", "base\n")
	writeMergeFixture(t, r.root, "issues/README.md", "<!-- managed by harnez index -->\n# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n")
	gitRunE2E(t, r.root, nil, "init", "-q", "-b", "main")
	gitRunE2E(t, r.root, nil, "config", "user.name", "Harnez E2E")
	gitRunE2E(t, r.root, nil, "config", "user.email", "harnez-e2e@example.invalid")
	if out, err := r.cli("init", "-d", "."); err != nil {
		t.Fatalf("harnez init: %v\n%s", err, out)
	}
	r.commitAll("base")
	return r
}

// The unit tests stub the merge driver with `true`. Only the real driver has the
// failure this guards against: replay stopping at `conflicting generated rows`
// before the repair step can run.
func TestIssuesRebaseRepairsCollisionWithRealMergeDriver(t *testing.T) {
	r := setupRebaseE2E(t)
	gitRunE2E(t, r.root, nil, "switch", "-qc", "upstream")
	r.ticket("1010-upstream.md", "upstream")
	gitRunE2E(t, r.root, nil, "switch", "-qc", "local", "main")
	r.ticket("1010-local.md", "local")

	out, err := r.cli("issues", "rebase", "-d", ".", "upstream")
	if err != nil {
		t.Fatalf("issues rebase: %v\n%s", err, out)
	}
	r.verifyRepaired(out)
}

// A substantive conflict hands control to the user. The later ticket commit then
// replays under a plain `git rebase --continue`, and a rerun does the repair.
func TestIssuesRebaseResumesAfterGitRebaseContinue(t *testing.T) {
	r := setupRebaseE2E(t)
	gitRunE2E(t, r.root, nil, "switch", "-qc", "upstream")
	writeMergeFixture(t, r.root, "shared.txt", "upstream\n")
	r.commitAll("upstream edit")
	r.ticket("1010-upstream.md", "upstream")
	gitRunE2E(t, r.root, nil, "switch", "-qc", "local", "main")
	writeMergeFixture(t, r.root, "shared.txt", "local\n")
	r.commitAll("local edit")
	r.ticket("1010-local.md", "local")

	if out, err := r.cli("issues", "rebase", "-d", ".", "upstream"); err == nil {
		t.Fatalf("expected the shared.txt conflict to stop the replay:\n%s", out)
	}
	writeMergeFixture(t, r.root, "shared.txt", "resolved\n")
	gitRunE2E(t, r.root, r.env, "add", "shared.txt")
	gitRunE2E(t, r.root, append(r.env, "GIT_EDITOR=true"), "rebase", "--continue")

	out, err := r.cli("issues", "rebase", "-d", ".", "upstream")
	if err != nil {
		t.Fatalf("rerun: %v\n%s", err, out)
	}
	r.verifyRepaired(out)
}
