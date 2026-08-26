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
