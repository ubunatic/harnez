package claude

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/gitflow"
)

// agitGitRun runs git in dir and fails the test on error. It returns trimmed stdout.
func agitGitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

// agitRepo makes a git work tree with an optional origin URL and points the
// global git config at an empty file, so nothing reaches the developer's ~/.gitconfig.
func agitRepo(t *testing.T, origin string) string {
	t.Helper()
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	dir := t.TempDir()
	agitGitRun(t, dir, "init", "-q", "-b", "main")
	if origin != "" {
		agitGitRun(t, dir, "remote", "add", "origin", origin)
	}
	return dir
}

// agitConfigValue returns the repo-local value of key, or "" when it is unset.
func agitConfigValue(t *testing.T, dir, key string) string {
	t.Helper()
	cmd := exec.Command("git", "config", "--local", "--get", key)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runAgitPullConfig runs installAgitPullConfig the way init does and returns the
// number of changes it reported.
func runAgitPullConfig(t *testing.T, dir string, repoConfig *RepoConfig) int {
	t.Helper()
	ir := &initReport{}
	if err := installAgitPullConfig(ir, dir, repoConfig); err != nil {
		t.Fatalf("installAgitPullConfig: %v", err)
	}
	return ir.changes
}

func TestInstallAgitPullConfigDetection(t *testing.T) {
	const bare = "/srv/git/harnez.git"
	for _, tc := range []struct {
		name   string
		origin string
		push   string
		mode   gitflow.Mode
		agit   bool
	}{
		{name: "team mode", origin: bare, mode: gitflow.ModeTeam, agit: true},
		{name: "solo on Codeberg", origin: "https://codeberg.org/ubunatic/harnez.git", agit: true},
		{name: "solo on Codeberg over ssh", origin: "git@codeberg.org:ubunatic/harnez.git", agit: true},
		{name: "refs/for push refspec", origin: bare, push: "HEAD:refs/for/main/topic", agit: true},
		{name: "solo on GitHub stays untouched", origin: "https://github.com/ubunatic/harnez.git"},
		{name: "solo local bare stays untouched", origin: bare},
		{name: "fork mode stays untouched", origin: bare, mode: gitflow.ModeFork},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := agitRepo(t, tc.origin)
			if tc.push != "" {
				agitGitRun(t, dir, "config", "--add", "remote.origin.push", tc.push)
			}
			changes := runAgitPullConfig(t, dir, &RepoConfig{Git: gitflow.Config{Mode: tc.mode}})
			autoSetup := agitConfigValue(t, dir, "branch.autoSetupMerge")
			defaultRemote := agitConfigValue(t, dir, "checkout.defaultRemote")
			if !tc.agit {
				if changes != 0 || autoSetup != "" || defaultRemote != "" {
					t.Fatalf("non-AGit repo changed: changes=%d autoSetupMerge=%q defaultRemote=%q", changes, autoSetup, defaultRemote)
				}
				return
			}
			if autoSetup != "always" || defaultRemote != "origin" {
				t.Fatalf("autoSetupMerge=%q defaultRemote=%q, want always and origin", autoSetup, defaultRemote)
			}
			if changes != 2 {
				t.Fatalf("changes = %d, want 2", changes)
			}
		})
	}
}

func TestInstallAgitPullConfigIdempotent(t *testing.T) {
	dir := agitRepo(t, "https://codeberg.org/ubunatic/harnez.git")
	if n := runAgitPullConfig(t, dir, &RepoConfig{}); n != 2 {
		t.Fatalf("first run changes = %d, want 2", n)
	}
	if n := runAgitPullConfig(t, dir, &RepoConfig{}); n != 0 {
		t.Fatalf("second run changes = %d, want 0", n)
	}
	if got := agitGitRun(t, dir, "config", "--local", "--get-all", "branch.autoSetupMerge"); got != "always" {
		t.Fatalf("branch.autoSetupMerge values = %q, want one value", got)
	}
}

func TestInstallAgitPullConfigSkipsNonGitDir(t *testing.T) {
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	dir := t.TempDir()
	if n := runAgitPullConfig(t, dir, &RepoConfig{Git: gitflow.Config{Mode: gitflow.ModeTeam}}); n != 0 {
		t.Fatalf("non-git dir changes = %d, want 0", n)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Fatalf("non-git dir gained a .git entry: %v", err)
	}
}

func TestInstallAgitPullConfigNeverWritesGlobalConfig(t *testing.T) {
	dir := agitRepo(t, "https://codeberg.org/ubunatic/harnez.git")
	global := os.Getenv("GIT_CONFIG_GLOBAL")
	runAgitPullConfig(t, dir, &RepoConfig{})
	data, err := os.ReadFile(global)
	if err != nil {
		t.Fatal(err)
	}
	if len(bytes.TrimSpace(data)) != 0 {
		t.Fatalf("global config was written:\n%s", data)
	}
}
