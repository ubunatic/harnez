package prwait

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return dir
}

// Only a missing remote reads as "no such remote"; other git failures keep git's message (issue 1249).
func TestSystemGitRemoteURLKeepsGitErrors(t *testing.T) {
	dir := gitRepo(t)
	_, err := SystemGit{}.RemoteURL(dir, "nope")
	if err == nil || !strings.Contains(err.Error(), "no such remote") {
		t.Fatalf("missing remote: err = %v, want no such remote", err)
	}

	bad := filepath.Join(t.TempDir(), "broken.gitconfig")
	if err := os.WriteFile(bad, []byte("[broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", bad)
	_, err = SystemGit{}.RemoteURL(dir, "origin")
	if err == nil || strings.Contains(err.Error(), "no such remote") {
		t.Fatalf("broken config: err = %v, want git's own error, not no such remote", err)
	}
	for _, want := range []string{"git remote get-url origin", "exit 128", "bad config"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("broken config: err = %q lacks %q", err, want)
		}
	}
}

func TestSystemGitIsAncestorKeepsGitErrors(t *testing.T) {
	dir := gitRepo(t)
	_, err := SystemGit{}.IsAncestor(dir, "deadbeef", "HEAD")
	if err == nil {
		t.Fatal("unknown objects must be an error, not 'not an ancestor'")
	}
	for _, want := range []string{"exit 128", "deadbeef"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q lacks %q", err, want)
		}
	}
}
