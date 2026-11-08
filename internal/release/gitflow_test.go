package release

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const releaseProtectedYAML = `version: 1
issues:
  tracker: none
  merge_driver: false
docs:
  studies: false
  copies: []
rules:
  quota: false
make:
  container_test: false
git:
  mode: team
  protected_main: true
`

// gitflowReleaseRepo builds a v1.0.0 repo on main with a bare origin that has
// main pushed. repoYAML, when set, is committed as .harnez/repo.yaml so the
// tree stays clean for the release.
func gitflowReleaseRepo(t *testing.T, repoYAML string) (dir, bare string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	dir = setupGitRepoWithVersion(t, "1.0.0", false)
	runGitCmd(t, dir, "branch", "-M", "main")
	if repoYAML != "" {
		if err := os.MkdirAll(filepath.Join(dir, ".harnez"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".harnez", "repo.yaml"), []byte(repoYAML), 0o644); err != nil {
			t.Fatal(err)
		}
		runGitCmd(t, dir, "add", ".harnez/repo.yaml")
		runGitCmd(t, dir, "commit", "-m", "chore: repo config")
	}
	// A forge-looking fetch URL satisfies forge detection; pushes go to the bare repo.
	stubs := t.TempDir()
	if err := os.WriteFile(filepath.Join(stubs, "fj"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubs+string(os.PathListSeparator)+os.Getenv("PATH"))
	bare = t.TempDir()
	runGitCmd(t, bare, "init", "--bare")
	runGitCmd(t, dir, "remote", "add", "origin", "https://codeberg.org/test/repo.git")
	runGitCmd(t, dir, "remote", "set-url", "--push", "origin", bare)
	runGitCmd(t, dir, "push", "origin", "main")
	return dir, bare
}

func gitflowReleaseOptions(dir string, out *bytes.Buffer) Options {
	return Options{Dir: dir, Bump: "patch", Force: true, SkipBuild: true, SkipSign: true, SkipPublish: true, Out: out}
}

func wantRemoteMatchesLocal(t *testing.T, dir, bare string, refs ...string) {
	t.Helper()
	for _, ref := range refs {
		want := pushTestGitOutput(t, dir, "rev-parse", ref)
		if got := pushTestGitOutput(t, bare, "rev-parse", ref); got != want {
			t.Errorf("remote %s = %s, want %s", ref, got, want)
		}
	}
}

// Protected main: the first run pushes the bump commit as an AGit PR and
// pauses without the tag; --continue after the merge pushes the tag.
func TestReleaseOnProtectedMainGoesThroughPR(t *testing.T) {
	dir, bare := gitflowReleaseRepo(t, releaseProtectedYAML)
	protectRemoteMain(t, bare)
	var out bytes.Buffer
	if err := Run(gitflowReleaseOptions(dir, &out)); err != nil {
		t.Fatalf("release on protected main: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "paused") {
		t.Errorf("no pause notice:\n%s", out.String())
	}
	bump := pushTestGitOutput(t, dir, "rev-parse", "v1.0.1^{commit}")
	if got := pushTestGitOutput(t, bare, "rev-parse", "refs/for/main/release-v1.0.1"); got != bump {
		t.Errorf("PR ref = %q, want bump %s", got, bump)
	}
	if strings.Contains(pushTestRefs(t, bare), "refs/tags/v1.0.1") {
		t.Fatal("tag pushed before the merge")
	}

	// Not merged yet: --continue only updates the PR.
	out.Reset()
	opt := gitflowReleaseOptions(dir, &out)
	opt.Continue = true
	if err := Run(opt); err != nil || !strings.Contains(out.String(), "paused") {
		t.Fatalf("--continue before merge: %v\n%s", err, out.String())
	}
	if strings.Contains(pushTestRefs(t, bare), "refs/tags/v1.0.1") {
		t.Fatal("tag pushed before the merge")
	}

	// The forge merges the PR with a merge commit.
	merge := pushTestGitOutput(t, dir, "commit-tree", bump+"^{tree}", "-p", bump+"^", "-p", bump, "-m", "Merge release PR")
	runGitCmd(t, dir, "push", "origin", merge+":refs/heads/merge-tmp")
	runGitCmd(t, bare, "update-ref", "refs/heads/main", merge)
	out.Reset()
	if err := Run(opt); err != nil {
		t.Fatalf("--continue after merge: %v\n%s", err, out.String())
	}
	wantRemoteMatchesLocal(t, dir, bare, "refs/tags/v1.0.1", "refs/heads/main")
}

func TestReleaseDryRunOnProtectedMainChangesNothing(t *testing.T) {
	dir, bare := gitflowReleaseRepo(t, releaseProtectedYAML)
	spec, err := os.ReadFile(filepath.Join(dir, "version.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	head := pushTestGitOutput(t, dir, "rev-parse", "HEAD")
	tags := pushTestGitOutput(t, dir, "tag")
	remoteRefs := pushTestRefs(t, bare)
	var out bytes.Buffer
	opt := gitflowReleaseOptions(dir, &out)
	opt.DryRun = true
	if err := Run(opt); err != nil {
		t.Fatalf("dry run on protected main: %v\n%s", err, out.String())
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "version.yaml")); !bytes.Equal(got, spec) {
		t.Errorf("version.yaml changed by a dry run:\n%s", got)
	}
	if got := pushTestGitOutput(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("dry run committed: %s -> %s", head, got)
	}
	if got := pushTestGitOutput(t, dir, "tag"); got != tags {
		t.Errorf("dry run tagged: %q -> %q", tags, got)
	}
	if got := pushTestRefs(t, bare); got != remoteRefs {
		t.Errorf("dry run pushed:\nbefore %s\nafter  %s", remoteRefs, got)
	}
}

func TestReleaseOnProtectedDetachedHeadRefusesBeforeBump(t *testing.T) {
	dir, bare := gitflowReleaseRepo(t, releaseProtectedYAML)
	runGitCmd(t, dir, "switch", "--detach")
	head := pushTestGitOutput(t, dir, "rev-parse", "HEAD")
	remoteRefs := pushTestRefs(t, bare)
	var out bytes.Buffer
	err := Run(gitflowReleaseOptions(dir, &out))
	if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("want a detached HEAD refusal, got %v\n%s", err, out.String())
	}
	if got := pushTestGitOutput(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("refused release committed: %s -> %s", head, got)
	}
	if got := pushTestRefs(t, bare); got != remoteRefs {
		t.Errorf("refused release pushed:\nbefore %s\nafter  %s", remoteRefs, got)
	}
}

func TestReleaseOnBranchPushesBranchAndTag(t *testing.T) {
	dir, bare := gitflowReleaseRepo(t, releaseProtectedYAML)
	mainBefore := pushTestGitOutput(t, bare, "rev-parse", "refs/heads/main")
	runGitCmd(t, dir, "switch", "-c", "release/v1.0.1")
	var out bytes.Buffer
	if err := Run(gitflowReleaseOptions(dir, &out)); err != nil {
		t.Fatalf("release on a topic branch: %v\n%s", err, out.String())
	}
	for _, ref := range []string{"refs/heads/release/v1.0.1", "refs/tags/v1.0.1"} {
		want := pushTestGitOutput(t, dir, "rev-parse", ref)
		if got := pushTestGitOutput(t, bare, "rev-parse", ref); got != want {
			t.Errorf("remote %s = %s, want %s", ref, got, want)
		}
	}
	if got := pushTestGitOutput(t, bare, "rev-parse", "refs/heads/main"); got != mainBefore {
		t.Errorf("remote main moved: %s -> %s", mainBefore, got)
	}
}

// protectRemoteMain installs an update hook that rejects only refs/heads/main,
// as a forge's branch protection does. Without --atomic the tag would still land.
func protectRemoteMain(t *testing.T, bare string) (hook string) {
	t.Helper()
	hook = filepath.Join(bare, "hooks", "update")
	script := "#!/bin/sh\nif test \"$1\" = refs/heads/main\nthen\n\techo 'branch main is protected' >&2\n\texit 1\nfi\n"
	if err := os.WriteFile(hook, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return hook
}

// addSecondRemote adds a bare remote named second that already has main.
func addSecondRemote(t *testing.T, dir string) (second string) {
	t.Helper()
	second = t.TempDir()
	runGitCmd(t, second, "init", "--bare")
	runGitCmd(t, dir, "remote", "add", "second", second)
	runGitCmd(t, dir, "push", "second", "main")
	return second
}

// The publishing forge is pushed first, so its rejection reaches no other remote.
func TestReleasePushForgeRejectionLeavesOtherRemotes(t *testing.T) {
	dir, bare := gitflowReleaseRepo(t, releaseProtectedYAML)
	second := addSecondRemote(t, dir)
	protectRemoteMain(t, second)
	runGitCmd(t, dir, "commit", "--allow-empty", "-m", "chore(release): bump version to v9.9.9")
	runGitCmd(t, dir, "tag", "-a", "v9.9.9", "-m", "v9.9.9")
	refs := pushTestRefs(t, bare) + "|" + pushTestRefs(t, second)
	// "origin" sorts first in `git remote`; only forge priority can put "second" ahead.
	err := pushRelease(Options{Dir: dir, Out: &bytes.Buffer{}}, "v9.9.9", &ForgeInfo{RemoteName: "second"})
	if err == nil || !strings.Contains(err.Error(), "branch main is protected") {
		t.Fatalf("want the forge's rejection, got %v", err)
	}
	if got := pushTestRefs(t, bare) + "|" + pushTestRefs(t, second); got != refs {
		t.Errorf("a remote changed despite the forge's rejection:\nbefore %s\nafter  %s", refs, got)
	}
}

// A later remote can reject after an earlier one accepted; retrying is idempotent.
func TestReleasePushLaterRemoteRejectionCanRetry(t *testing.T) {
	dir, bare := gitflowReleaseRepo(t, releaseProtectedYAML)
	second := addSecondRemote(t, dir)
	hook := protectRemoteMain(t, second)
	runGitCmd(t, dir, "commit", "--allow-empty", "-m", "chore(release): bump version to v9.9.9")
	runGitCmd(t, dir, "tag", "-a", "v9.9.9", "-m", "v9.9.9")
	secondRefs := pushTestRefs(t, second)
	opt := Options{Dir: dir, Out: &bytes.Buffer{}}
	forge := &ForgeInfo{RemoteName: "origin"}
	err := pushRelease(opt, "v9.9.9", forge)
	if err == nil || !strings.Contains(err.Error(), "--continue") {
		t.Fatalf("want a retryable failure, got %v", err)
	}
	wantRemoteMatchesLocal(t, dir, bare, "refs/tags/v9.9.9")
	if got := pushTestRefs(t, second); got != secondRefs {
		t.Errorf("rejected push changed the remote:\nbefore %s\nafter  %s", secondRefs, got)
	}
	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	if err := pushRelease(opt, "v9.9.9", forge); err != nil {
		t.Fatalf("retry after the rejection: %v", err)
	}
	wantRemoteMatchesLocal(t, dir, second, "refs/heads/main", "refs/tags/v9.9.9")
}

// wantSoloReleaseOutput was recorded from a solo release on main before the
// gitflow guard existed; the project name is the temp dir's base name.
const wantSoloReleaseOutput = `==> Preparing release for PROJECT (dry-run=false, continue=false)
  [version]   Bumping version: 1.0.0 -> 1.0.1 (tag: v1.0.1)
  [git]       Committed version bump (chore(release): bump version to v1.0.1)
  [git]       Created annotated tag: v1.0.1
  [forge]     Verified/Enabled 'has_releases' on test/repo
  [git]       Pushing branch main and tag v1.0.1 to origin...
==> Release v1.0.1 completed successfully!
`

func TestReleaseSoloUnchanged(t *testing.T) {
	dir, bare := gitflowReleaseRepo(t, "")
	var out bytes.Buffer
	if err := Run(gitflowReleaseOptions(dir, &out)); err != nil {
		t.Fatalf("solo release on main: %v\n%s", err, out.String())
	}
	got := strings.ReplaceAll(out.String(), filepath.Base(dir), "PROJECT")
	if got != wantSoloReleaseOutput {
		t.Errorf("solo release output changed:\ngot:\n%s\nwant:\n%s", got, wantSoloReleaseOutput)
	}
	for _, ref := range []string{"refs/heads/main", "refs/tags/v1.0.1"} {
		want := pushTestGitOutput(t, dir, "rev-parse", ref)
		if got := pushTestGitOutput(t, bare, "rev-parse", ref); got != want {
			t.Errorf("remote %s = %s, want %s", ref, got, want)
		}
	}
}
