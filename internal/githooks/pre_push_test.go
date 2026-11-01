package githooks

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hookSource is the versioned pre-push hook, relative to this package directory.
const hookSource = "../../scripts/git-hooks/pre-push"

// fixture is a work clone with two bare repositories: cb stands in for Codeberg
// (remote origin and the first URL of "both"), gh for GitHub. The GitHub path
// contains github.com, which is what the hook matches on.
type fixture struct {
	t    *testing.T
	env  []string
	work string
	cb   string
	gh   string
}

// newFixture builds the repositories and installs the hook into the work clone.
// It skips when git or git-lfs is missing.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	for _, tool := range []string{"git", "git-lfs"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0o755); err != nil {
		t.Fatal(err)
	}
	globalCfg := filepath.Join(home, "gitconfig")
	if err := os.WriteFile(globalCfg, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	f := &fixture{
		t: t,
		env: []string{
			"PATH=" + os.Getenv("PATH"),
			"HOME=" + home,
			"GIT_CONFIG_GLOBAL=" + globalCfg,
			"GIT_CONFIG_NOSYSTEM=1",
			"GIT_TERMINAL_PROMPT=0",
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@example.invalid",
		},
		work: filepath.Join(root, "work"),
		cb:   filepath.Join(root, "codeberg.org", "ubunatic", "harnez.git"),
		gh:   filepath.Join(root, "github.com", "ubunatic", "harnez.git"),
	}
	for _, dir := range []string{filepath.Dir(f.cb), filepath.Dir(f.gh)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.mustRun(root, "init", "--bare", "--initial-branch=main", f.cb)
	f.mustRun(root, "init", "--bare", "--initial-branch=main", f.gh)
	f.mustRun(root, "init", "--initial-branch=main", f.work)

	f.mustGit("remote", "add", "origin", f.cb)
	f.mustGit("remote", "add", "github", f.gh)
	f.mustGit("remote", "add", "both", f.cb)
	f.mustGit("remote", "set-url", "--push", "both", f.cb)
	f.mustGit("remote", "set-url", "--add", "--push", "both", f.gh)

	src, err := os.ReadFile(hookSource)
	if err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(f.work, ".git", "hooks", "pre-push")
	if err := os.WriteFile(hook, src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

// run executes git in dir and returns its combined output and exit code.
func (f *fixture) run(dir string, args ...string) (string, int) {
	f.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = f.env
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return string(out), 0
	case errors.As(err, &exitErr):
		return string(out), exitErr.ExitCode()
	default:
		f.t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		return "", -1
	}
}

// mustRun runs git in dir and fails the test on a non-zero exit.
func (f *fixture) mustRun(dir string, args ...string) string {
	f.t.Helper()
	out, code := f.run(dir, args...)
	if code != 0 {
		f.t.Fatalf("git %s: exit %d\n%s", strings.Join(args, " "), code, out)
	}
	return strings.TrimSpace(out)
}

// mustGit runs git in the work clone and fails the test on a non-zero exit.
func (f *fixture) mustGit(args ...string) string {
	f.t.Helper()
	return f.mustRun(f.work, args...)
}

// push runs git push in the work clone and returns the exit code.
func (f *fixture) push(args ...string) int {
	f.t.Helper()
	out, code := f.run(f.work, append([]string{"push"}, args...)...)
	f.t.Logf("git push %s: exit %d\n%s", strings.Join(args, " "), code, out)
	return code
}

// commit writes a new file, commits it on the current branch and returns the new HEAD.
func (f *fixture) commit(name string) string {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.work, name), []byte(name+"\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.mustGit("add", name)
	f.mustGit("commit", "-q", "-m", "change "+name)
	return f.mustGit("rev-parse", "HEAD")
}

// refIn returns the sha of ref in repo, or "" when the ref does not exist.
func (f *fixture) refIn(repo, ref string) string {
	f.t.Helper()
	out, code := f.run(repo, "rev-parse", "--verify", "-q", ref)
	if code != 0 {
		return ""
	}
	return strings.TrimSpace(out)
}

// assertRef fails the test unless ref in repo is at want ("" means absent).
func (f *fixture) assertRef(repo, ref, want string) {
	f.t.Helper()
	if got := f.refIn(repo, ref); got != want {
		f.t.Errorf("%s in %s = %q, want %q", ref, repo, got, want)
	}
}

func TestPrePushGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("creates several repositories and runs git-lfs; skipped in -short")
	}

	t.Run("both pushes main to Codeberg and GitHub", func(t *testing.T) {
		f := newFixture(t)
		sha := f.commit("a.txt")
		if code := f.push("both", "main"); code != 0 {
			t.Fatalf("push both main: exit %d, want 0", code)
		}
		f.assertRef(f.cb, "refs/heads/main", sha)
		f.assertRef(f.gh, "refs/heads/main", sha)
	})

	t.Run("no-op push exits 0", func(t *testing.T) {
		f := newFixture(t)
		f.commit("a.txt")
		if code := f.push("both", "main"); code != 0 {
			t.Fatalf("first push both main: exit %d, want 0", code)
		}
		if code := f.push("both", "main"); code != 0 {
			t.Fatalf("no-op push both main: exit %d, want 0", code)
		}
	})

	t.Run("topic via both exits 1 and never reaches GitHub", func(t *testing.T) {
		f := newFixture(t)
		f.mustGit("checkout", "-q", "-b", "topic")
		f.commit("t.txt")
		if code := f.push("both", "topic"); code != 1 {
			t.Fatalf("push both topic: exit %d, want 1", code)
		}
		f.assertRef(f.gh, "refs/heads/topic", "")
	})

	t.Run("tag via both reaches Codeberg and GitHub", func(t *testing.T) {
		f := newFixture(t)
		f.commit("a.txt")
		if code := f.push("both", "main"); code != 0 {
			t.Fatalf("push both main: exit %d, want 0", code)
		}
		f.mustGit("tag", "v0.1.0")
		want := f.mustGit("rev-parse", "refs/tags/v0.1.0")
		if code := f.push("both", "refs/tags/v0.1.0"); code != 0 {
			t.Fatalf("push both tag: exit %d, want 0", code)
		}
		f.assertRef(f.cb, "refs/tags/v0.1.0", want)
		f.assertRef(f.gh, "refs/tags/v0.1.0", want)
	})

	t.Run("Codeberg rejecting leaves GitHub unmoved", func(t *testing.T) {
		f := newFixture(t)
		reject := filepath.Join(f.cb, "hooks", "pre-receive")
		if err := os.WriteFile(reject, []byte("#!/bin/sh\necho 'rejected by test' >&2\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		f.commit("a.txt")
		if code := f.push("both", "main"); code == 0 {
			t.Fatal("push both main: exit 0, want non-zero when Codeberg rejects")
		}
		f.assertRef(f.cb, "refs/heads/main", "")
		f.assertRef(f.gh, "refs/heads/main", "")
	})

	t.Run("origin topic and refs/for push exit 0", func(t *testing.T) {
		f := newFixture(t)
		f.mustGit("checkout", "-q", "-b", "topic")
		sha := f.commit("t.txt")
		if code := f.push("origin", "topic"); code != 0 {
			t.Fatalf("push origin topic: exit %d, want 0", code)
		}
		f.assertRef(f.cb, "refs/heads/topic", sha)

		if code := f.push("origin", "HEAD:refs/for/main/x"); code != 0 {
			t.Fatalf("push origin HEAD:refs/for/main/x: exit %d, want 0", code)
		}
		f.assertRef(f.cb, "refs/for/main/x", sha)
	})

	t.Run("github remote is not guarded", func(t *testing.T) {
		f := newFixture(t)
		f.mustGit("checkout", "-q", "-b", "topic")
		sha := f.commit("t.txt")
		if code := f.push("github", "topic"); code != 0 {
			t.Fatalf("push github topic: exit %d, want 0", code)
		}
		f.assertRef(f.gh, "refs/heads/topic", sha)
	})

	t.Run("GitHub-only branch survives a both push of main", func(t *testing.T) {
		f := newFixture(t)
		f.commit("a.txt")
		if code := f.push("both", "main"); code != 0 {
			t.Fatalf("push both main: exit %d, want 0", code)
		}
		f.mustGit("checkout", "-q", "-b", "jules/1234-fix")
		julesSHA := f.commit("j.txt")
		if code := f.push("github", "jules/1234-fix"); code != 0 {
			t.Fatalf("push github jules/1234-fix: exit %d, want 0", code)
		}
		f.mustGit("checkout", "-q", "main")
		mainSHA := f.commit("b.txt")
		if code := f.push("both", "main"); code != 0 {
			t.Fatalf("push both main after jules branch: exit %d, want 0", code)
		}
		f.assertRef(f.gh, "refs/heads/jules/1234-fix", julesSHA)
		f.assertRef(f.gh, "refs/heads/main", mainSHA)
	})
}
