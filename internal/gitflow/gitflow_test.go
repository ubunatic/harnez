package gitflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// isolateGit keeps scratch-repo tests independent of the ambient HOME and git config.
func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRepo creates a scratch repo on branch `branch` with one commit.
func newRepo(t *testing.T, branch string) string {
	t.Helper()
	isolateGit(t)
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", branch)
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func writeRepoYAML(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".harnez"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".harnez", "repo.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDefaultsWhenGitKeyAbsent(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("missing repo.yaml: %v", err)
	}
	if cfg.Mode != ModeSolo || cfg.ProtectedMain {
		t.Fatalf("missing file: got %+v, want solo/false", cfg)
	}
	writeRepoYAML(t, dir, "version: 1\nissues:\n  tracker: harnez\n  merge_driver: true\n")
	cfg, err = Load(dir)
	if err != nil {
		t.Fatalf("repo.yaml without git key: %v", err)
	}
	if cfg.Mode != ModeSolo || cfg.ProtectedMain {
		t.Fatalf("absent key: got %+v, want solo/false", cfg)
	}
}

func TestLoadTeamProtected(t *testing.T) {
	dir := t.TempDir()
	writeRepoYAML(t, dir, "version: 1\ngit:\n  mode: team\n  protected_main: true\n")
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != ModeTeam || !cfg.ProtectedMain {
		t.Fatalf("got %+v, want team/true", cfg)
	}
}

func TestLoadBytesChecksCandidateWithoutDisk(t *testing.T) {
	cfg, err := LoadBytes("/nowhere/repo.yaml", []byte("version: 1\ngit:\n  mode: team\n  protected_main: true\n"))
	if err != nil || cfg.Mode != ModeTeam || !cfg.ProtectedMain {
		t.Fatalf("LoadBytes = %+v, %v; want team protected", cfg, err)
	}
	_, err = LoadBytes("/nowhere/repo.yaml", []byte("version: 1\ngit:\n  protected_main: true\n"))
	if err == nil || !strings.Contains(err.Error(), "/nowhere/repo.yaml") || !strings.Contains(err.Error(), "git.protected_main") {
		t.Fatalf("LoadBytes error = %v, want the path and git.protected_main (mode defaults to solo)", err)
	}
}

func TestLoadRejectsInvalidGitKey(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"unknown mode":      {"git:\n  mode: trunk\n", "git.mode"},
		"protected on solo": {"git:\n  mode: solo\n  protected_main: true\n", "git.protected_main"},
		"unknown key":       {"git:\n  mode: team\n  foo: 1\n", "git.foo"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeRepoYAML(t, dir, "version: 1\n"+tc.body)
			_, err := Load(dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load error = %v, want mention of %q", err, tc.want)
			}
			if !strings.Contains(err.Error(), "repo.yaml") {
				t.Fatalf("Load error = %v, want it to name repo.yaml", err)
			}
		})
	}
}

func TestDefaultBranchOverride(t *testing.T) {
	dir := newRepo(t, "main")
	writeRepoYAML(t, dir, "version: 1\ngit:\n  mode: team\n  default_branch: trunk\n")
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := DefaultBranch(dir, cfg)
	if !ok || got != "trunk" {
		t.Fatalf("DefaultBranch = %q, %v; want trunk, true", got, ok)
	}
}

func TestDefaultBranchFromOriginHEAD(t *testing.T) {
	dir := newRepo(t, "dev")
	bare := t.TempDir()
	runGit(t, bare, "init", "--bare", "-b", "dev")
	runGit(t, dir, "remote", "add", "origin", bare)
	runGit(t, dir, "push", "origin", "dev")
	runGit(t, dir, "remote", "set-head", "origin", "-a")
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := DefaultBranch(dir, cfg)
	if !ok || got != "dev" {
		t.Fatalf("DefaultBranch = %q, %v; want dev, true", got, ok)
	}
	if want := []string{"dev"}; !reflect.DeepEqual(ProtectedBranches(dir, cfg), want) {
		t.Fatalf("ProtectedBranches = %v, want %v", ProtectedBranches(dir, cfg), want)
	}
}

func TestDefaultBranchNoOriginHEAD(t *testing.T) {
	dir := newRepo(t, "master")
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := DefaultBranch(dir, cfg)
	if ok || got != "" {
		t.Fatalf("DefaultBranch = %q, %v; want empty, false", got, ok)
	}
	if want := []string{"main", "master"}; !reflect.DeepEqual(ProtectedBranches(dir, cfg), want) {
		t.Fatalf("ProtectedBranches = %v, want %v", ProtectedBranches(dir, cfg), want)
	}
}

const (
	teamYAML          = "version: 1\ngit:\n  mode: team\n"
	teamProtectedYAML = "version: 1\ngit:\n  mode: team\n  protected_main: true\n"
)

// guardRepo builds a scratch repo on branch with the given repo.yaml body and
// captures the guard's stderr warning.
func guardRepo(t *testing.T, branch, yaml string) (dir string, stderr *strings.Builder) {
	t.Helper()
	dir = newRepo(t, branch)
	if yaml != "" {
		writeRepoYAML(t, dir, yaml)
	}
	stderr = &strings.Builder{}
	old := Stderr
	Stderr = stderr
	t.Cleanup(func() { Stderr = old })
	return dir, stderr
}

func TestGuardSoloOnMain(t *testing.T) {
	dir, stderr := guardRepo(t, "main", "")
	if err := GuardCommit(dir, "close ticket"); err != nil {
		t.Fatalf("solo on main must commit: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("solo must not warn, got %q", stderr)
	}
}

func TestGuardTeamOnMainWarns(t *testing.T) {
	dir, stderr := guardRepo(t, "main", teamYAML)
	if err := GuardCommit(dir, "close ticket"); err != nil {
		t.Fatalf("unprotected team on main must still commit: %v", err)
	}
	if want := "git.mode=team: committing on main; use a topic branch"; !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, want it to contain %q", stderr, want)
	}
}

func TestGuardProtectedOnMainRefuses(t *testing.T) {
	dir, _ := guardRepo(t, "main", teamProtectedYAML)
	err := GuardCommit(dir, "close ticket")
	if err == nil {
		t.Fatal("protected main must refuse")
	}
	for _, want := range []string{"git.protected_main", "git switch -c", "close ticket", "'main'"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestGuardProtectedOnTopicBranch(t *testing.T) {
	dir, stderr := guardRepo(t, "main", teamProtectedYAML)
	runGit(t, dir, "switch", "-c", "issue/1039-git-mode")
	if err := GuardCommit(dir, "close ticket"); err != nil {
		t.Fatalf("topic branch must commit: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("topic branch must not warn, got %q", stderr)
	}
}

func TestGuardReleaseAllowsProtectedMainWithNote(t *testing.T) {
	dir, stderr := guardRepo(t, "main", teamProtectedYAML)
	if err := GuardRelease(dir); err != nil {
		t.Fatalf("release on protected main must pass: %v", err)
	}
	if want := "goes out as an AGit PR"; !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, want it to contain %q", stderr, want)
	}
	// Every other auto-committing command stays refused on the same branch.
	if err := GuardCommit(dir, "close ticket"); err == nil {
		t.Fatal("protected main must still refuse other commits")
	}
}

func TestGuardReleaseFallsBackToGuardCommit(t *testing.T) {
	t.Run("protected detached HEAD refuses", func(t *testing.T) {
		dir, _ := guardRepo(t, "main", teamProtectedYAML)
		runGit(t, dir, "checkout", "--detach")
		err := GuardRelease(dir)
		if err == nil || !strings.Contains(err.Error(), "refusing to release on a detached HEAD") {
			t.Fatalf("error = %v, want a detached HEAD refusal", err)
		}
	})
	t.Run("protected topic branch is silent", func(t *testing.T) {
		dir, stderr := guardRepo(t, "release/v1.0.1", teamProtectedYAML)
		if err := GuardRelease(dir); err != nil || stderr.Len() != 0 {
			t.Fatalf("err = %v, stderr = %q", err, stderr)
		}
	})
	t.Run("unprotected team main warns", func(t *testing.T) {
		dir, stderr := guardRepo(t, "main", teamYAML)
		if err := GuardRelease(dir); err != nil {
			t.Fatal(err)
		}
		if want := "use a topic branch"; !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want it to contain %q", stderr, want)
		}
	})
}

func TestGuardProtectedDetachedHEAD(t *testing.T) {
	dir, _ := guardRepo(t, "main", teamProtectedYAML)
	runGit(t, dir, "checkout", "--detach")
	err := GuardCommit(dir, "close ticket")
	if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("error = %v, want a detached HEAD refusal", err)
	}
	for _, want := range []string{"git switch -c topic", "git switch <branch>"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain actionable recovery %q", err, want)
		}
	}
}

func TestGuardTeamDetachedHEADWarns(t *testing.T) {
	dir, stderr := guardRepo(t, "main", teamYAML)
	runGit(t, dir, "checkout", "--detach")
	if err := GuardCommit(dir, "close ticket"); err != nil {
		t.Fatalf("unprotected team must still commit: %v", err)
	}
	if !strings.Contains(stderr.String(), "detached HEAD") {
		t.Fatalf("stderr = %q, want a detached HEAD warning", stderr)
	}
	for _, want := range []string{"git switch -c topic", "git switch <branch>"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr %q does not contain actionable recovery %q", stderr.String(), want)
		}
	}
}

func TestCheckDetachedHead(t *testing.T) {
	// Non-git directory returns empty warning
	nonGitDir := t.TempDir()
	if warn := CheckDetachedHead(nonGitDir); warn != "" {
		t.Fatalf("expected empty warning in non-git dir, got: %s", warn)
	}

	dir := newRepo(t, "main")
	if warn := CheckDetachedHead(dir); warn != "" {
		t.Fatalf("expected empty warning on main, got: %s", warn)
	}

	// Detach HEAD at commit
	runGit(t, dir, "checkout", "--detach")
	warn := CheckDetachedHead(dir)
	if warn == "" {
		t.Fatal("expected warning when HEAD is detached, got empty")
	}
	for _, want := range []string{"WARNING: HEAD is detached at", "Switch to a branch (git switch -c <topic> or git switch <branch>) before committing."} {
		if !strings.Contains(warn, want) {
			t.Errorf("warning %q does not contain %q", warn, want)
		}
	}

	// Detach HEAD at a tag
	runGit(t, dir, "tag", "v1.2.3")
	warn = CheckDetachedHead(dir)
	if !strings.Contains(warn, "HEAD is detached at v1.2.3") {
		t.Errorf("warning %q does not mention tag v1.2.3", warn)
	}
}

func TestDetachedHeadRefFallback(t *testing.T) {
	// In a non-git directory or repo where describe and rev-parse fail, DetachedHeadRef falls back to "HEAD"
	nonGitDir := t.TempDir()
	if ref := DetachedHeadRef(nonGitDir); ref != "HEAD" {
		t.Fatalf("expected HEAD fallback in non-git dir, got: %q", ref)
	}
}

func TestGuardSoloDetachedHEAD(t *testing.T) {
	dir, stderr := guardRepo(t, "main", "")
	runGit(t, dir, "checkout", "--detach")
	if err := GuardCommit(dir, "close ticket"); err != nil || stderr.Len() != 0 {
		t.Fatalf("solo detached: err=%v stderr=%q, want nil and silence", err, stderr)
	}
}

func TestGuardProtectedNoOriginHEADOnMaster(t *testing.T) {
	dir, _ := guardRepo(t, "master", teamProtectedYAML)
	err := GuardCommit(dir, "close ticket")
	if err == nil || !strings.Contains(err.Error(), "'master'") {
		t.Fatalf("error = %v, want a refusal naming 'master' (fallback list)", err)
	}
}

func TestGuardProtectedUsesOriginHEADNotFallbackNames(t *testing.T) {
	dir, _ := guardRepo(t, "dev", teamProtectedYAML)
	bare := t.TempDir()
	runGit(t, bare, "init", "--bare", "-b", "dev")
	runGit(t, dir, "remote", "add", "origin", bare)
	runGit(t, dir, "push", "origin", "dev")
	runGit(t, dir, "remote", "set-head", "origin", "-a")
	if err := GuardCommit(dir, "close ticket"); err == nil {
		t.Fatal("origin HEAD is dev: committing on dev must be refused")
	}
	runGit(t, dir, "switch", "-c", "main")
	if err := GuardCommit(dir, "close ticket"); err != nil {
		t.Fatalf("a branch named main is a topic branch when origin HEAD is dev: %v", err)
	}
}

func TestGuardInvalidRepoYAML(t *testing.T) {
	dir, _ := guardRepo(t, "main", "version: 1\ngit:\n  mode: trunk\n")
	err := GuardCommit(dir, "close ticket")
	if err == nil || !strings.Contains(err.Error(), "git.mode") {
		t.Fatalf("error = %v, want one naming git.mode", err)
	}
}
