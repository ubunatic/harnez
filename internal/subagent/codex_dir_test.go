package subagent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestExistingDirOrEmpty(t *testing.T) {
	dir := t.TempDir()
	if got := existingDirOrEmpty(dir); got != dir {
		t.Fatalf("existing dir: got %q", got)
	}
	if got := existingDirOrEmpty(filepath.Join(dir, "gone")); got != "" {
		t.Fatalf("missing dir must map to empty, got %q", got)
	}
	if got := existingDirOrEmpty(""); got != "" {
		t.Fatalf("empty stays empty, got %q", got)
	}
}

// A removed worktree must be named as the cause, not reported as a missing binary (issue 1117).
func TestStartProcessNamesMissingDir(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "taia-wt31")
	_, _, err := startProcess(context.Background(), gone, "true")
	if err == nil || !strings.Contains(err.Error(), gone) || strings.Contains(err.Error(), "fork/exec") {
		t.Fatalf("want error naming %s, got %v", gone, err)
	}
	rd, wait, err := startProcess(context.Background(), existingDirOrEmpty(gone), "true")
	if err != nil {
		t.Fatalf("fallback to current dir must start: %v", err)
	}
	_ = rd
	if err := wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
}
