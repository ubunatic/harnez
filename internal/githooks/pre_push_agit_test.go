package githooks

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	codebergURL       = "https://codeberg.org/ubunatic/harnez.git"
	githubURL         = "https://github.com/ubunatic/harnez.git"
	zeroSHA           = "0000000000000000000000000000000000000000"
	agitProtectedYAML = "git:\n  mode: team\n  protected_main: true\n"
)

// runHook runs the installed pre-push hook the way git does for one push URL of
// remote: $1 is the remote name, $2 its URL, and stdin one ref line that moves
// refs/heads/main to localSHA. It returns the hook's output and exit code.
func (f *fixture) runHook(remote, url, localSHA string) (string, int) {
	f.t.Helper()
	hook := filepath.Join(f.work, ".git", "hooks", "pre-push")
	cmd := exec.Command(hook, remote, url)
	cmd.Dir = f.work
	cmd.Env = f.env
	cmd.Stdin = strings.NewReader("refs/heads/main " + localSHA + " refs/heads/main " + zeroSHA + "\n")
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return string(out), 0
	case errors.As(err, &exitErr):
		return string(out), exitErr.ExitCode()
	default:
		f.t.Fatalf("run pre-push hook: %v", err)
		return "", -1
	}
}

// pushOutput runs git push in the work clone and returns its output and exit code.
func (f *fixture) pushOutput(args ...string) (string, int) {
	f.t.Helper()
	return f.run(f.work, append([]string{"push"}, args...)...)
}

// setPath puts dir first on the fixture's PATH, so a harnez placed there shadows the built one.
func (f *fixture) setPath(dir string) {
	f.t.Helper()
	for i, kv := range f.env {
		if strings.HasPrefix(kv, "PATH=") {
			f.env[i] = "PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH")
			return
		}
	}
	f.t.Fatal("fixture has no PATH entry")
}

// TestPrePushReleaseLegs runs the pushes `harnez release` makes on a protected
// default branch, with the hook installed: the AGit release PR and the tag on
// origin, then the merged main and the tag mirrored to GitHub. Only a direct
// push of main to origin is refused.
func TestPrePushReleaseLegs(t *testing.T) {
	if testing.Short() {
		t.Skip("creates several repositories and runs git-lfs; skipped in -short")
	}
	const tag = "v0.1.0"
	f := newFixture(t)
	f.writeRepoYAML(agitProtectedYAML)
	sha := f.commit("release.txt")
	f.mustGit("tag", tag)

	if code := f.push("origin", "main"); code == 0 {
		t.Fatal("direct push origin main: exit 0, want refusal with protected_main")
	}
	f.assertRef(f.cb, "refs/heads/main", "")

	if code := f.push("origin", "HEAD:refs/for/main/release-"+tag); code != 0 {
		t.Fatalf("AGit release PR push: exit %d, want 0", code)
	}
	f.assertRef(f.cb, "refs/for/main/release-"+tag, sha)

	if code := f.push("origin", "refs/tags/"+tag); code != 0 {
		t.Fatalf("push origin tag %s: exit %d, want 0", tag, code)
	}
	f.assertRef(f.cb, "refs/tags/"+tag, sha)

	if code := f.push("--atomic", "--no-force", "github", sha+":refs/heads/main", "refs/tags/"+tag+":refs/tags/"+tag); code != 0 {
		t.Fatalf("mirror main and tag %s to github: exit %d, want 0", tag, code)
	}
	f.assertRef(f.gh, "refs/heads/main", sha)
	f.assertRef(f.gh, "refs/tags/"+tag, sha)
}

// TestPrePushStaleHarnez checks that a harnez on PATH without push-guard's
// --remote and --url does not reject every push. The hook warns and skips the
// policy, so AGit and topic pushes still go through.
func TestPrePushStaleHarnez(t *testing.T) {
	if testing.Short() {
		t.Skip("creates several repositories and runs git-lfs; skipped in -short")
	}
	for _, tc := range []struct{ name, script string }{
		{name: "no push-guard", script: "#!/bin/sh\nexit 1\n"},
		{name: "push-guard without --url", script: "#!/bin/sh\nprintf 'Usage:\\n  harnez push-guard <remote-ref> [flags]\\n'\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "harnez"), []byte(tc.script), 0o755); err != nil {
				t.Fatal(err)
			}
			f.setPath(dir)
			f.writeRepoYAML(agitProtectedYAML)
			f.mustGit("checkout", "-q", "-b", "topic")
			sha := f.commit("t.txt")

			out, code := f.pushOutput("origin", "HEAD:refs/for/main/topic")
			if code != 0 {
				t.Fatalf("AGit push with a stale harnez: exit %d, want 0\n%s", code, out)
			}
			if !strings.Contains(out, "stale binary?") {
				t.Errorf("stale harnez: want a warning that the policy was skipped, got:\n%s", out)
			}
			f.assertRef(f.cb, "refs/for/main/topic", sha)

			if code := f.push("origin", "topic"); code != 0 {
				t.Fatalf("push origin topic with a stale harnez: exit %d, want 0", code)
			}
			f.assertRef(f.cb, "refs/heads/topic", sha)
		})
	}
}
