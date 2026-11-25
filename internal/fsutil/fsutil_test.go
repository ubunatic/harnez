package fsutil_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/fsutil"
)

func TestEnsureGitExclude(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(tmpDir, "sub", "folder")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. Add pattern from sub directory
	changed, err := fsutil.EnsureGitExclude(subDir, "AGENTS.local.md")
	if err != nil {
		t.Fatalf("EnsureGitExclude failed: %v", err)
	}
	if !changed {
		t.Errorf("Expected changed=true on first call")
	}

	excludePath := filepath.Join(gitDir, "info", "exclude")
	content, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("Failed to read exclude file: %v", err)
	}
	if !strings.Contains(string(content), "AGENTS.local.md") {
		t.Errorf("exclude file missing pattern: %s", string(content))
	}

	// 2. Call again - idempotency check
	changed, err = fsutil.EnsureGitExclude(tmpDir, "AGENTS.local.md")
	if err != nil {
		t.Fatalf("EnsureGitExclude (2nd) failed: %v", err)
	}
	if changed {
		t.Errorf("Expected changed=false on second call")
	}

	// 3. Call in non-git directory
	nonGit := t.TempDir()
	changed, err = fsutil.EnsureGitExclude(nonGit, "AGENTS.local.md")
	if err != nil {
		t.Fatalf("EnsureGitExclude in non-git failed: %v", err)
	}
	if changed {
		t.Errorf("Expected changed=false for non-git dir")
	}
}

func TestFindGitDirSkipsStrayGitDirectory(t *testing.T) {
	root := t.TempDir()
	stray := filepath.Join(root, ".git")
	if err := os.MkdirAll(filepath.Join(stray, "info"), 0755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if got, ok := fsutil.FindGitDir(sub); ok {
		t.Fatalf("stray .git directory without HEAD accepted as repo: %s", got)
	}
	changed, err := fsutil.EnsureGitExclude(sub, "AGENTS.local.md")
	if err != nil || changed {
		t.Fatalf("EnsureGitExclude in non-repo = %v, %v; want false, nil", changed, err)
	}
	if _, err := os.Stat(filepath.Join(stray, "info", "exclude")); !os.IsNotExist(err) {
		t.Fatalf("stray .git exclude was written: stat err = %v", err)
	}

	// A real repository deeper in the tree is still found from below the stray one.
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, ok := fsutil.FindGitDir(filepath.Join(repo, "deep"))
	if !ok || got != filepath.Join(repo, ".git") {
		t.Fatalf("FindGitDir(repo/deep) = %q, %v; want repo/.git", got, ok)
	}
}

func TestFindGitDirFollowsGitfileToRealRepo(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "worktrees", "wt")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, ".git"), []byte("gitdir: "+target+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, ok := fsutil.FindGitDir(work); !ok || got != target {
		t.Fatalf("FindGitDir(gitfile) = %q, %v; want %q", got, ok, target)
	}
	dangling := filepath.Join(root, "dangling")
	if err := os.MkdirAll(dangling, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dangling, ".git"), []byte("gitdir: "+filepath.Join(root, "missing")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, ok := fsutil.FindGitDir(dangling); ok {
		t.Fatalf("dangling gitfile accepted: %q", got)
	}
}
