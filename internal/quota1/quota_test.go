package quota1

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckAndRecord_Lifecycle(t *testing.T) {
	repoDir := t.TempDir()

	// Simulate a git repository
	gitDir := filepath.Join(repoDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatalf("write .git/HEAD: %v", err)
	}

	// Create a dummy source file with initial timestamp t0
	baseTime := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	srcFile := filepath.Join(repoDir, "main.go")
	if err := os.WriteFile(srcFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	_ = os.Chtimes(srcFile, baseTime, baseTime)

	// Step 1: First run - state file does not exist
	now := baseTime.Add(1 * time.Minute)
	res1, err := CheckAndRecord(CheckOptions{
		Dir: repoDir,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}
	if !res1.Allowed {
		t.Errorf("first run should be allowed, got blocked: %s", res1.Message)
	}

	expectedStateFile := filepath.Join(gitDir, "harnez", "quota_1.state")
	if res1.StateFile != expectedStateFile {
		t.Errorf("state file = %q, want %q", res1.StateFile, expectedStateFile)
	}
	if _, err := os.Stat(expectedStateFile); err != nil {
		t.Fatalf("expected state file to exist: %v", err)
	}

	// Step 2: Immediate rerun without edits - should be blocked
	now = now.Add(10 * time.Second)
	res2, err := CheckAndRecord(CheckOptions{
		Dir: repoDir,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}
	if res2.Allowed {
		t.Errorf("second run without edits should be blocked, got allowed")
	}
	if !strings.Contains(res2.Message, "blocked") {
		t.Errorf("expected blocked message, got: %s", res2.Message)
	}

	// Step 3: Modify a file in .git or ignored file - should STILL be blocked
	gitInternal := filepath.Join(gitDir, "COMMIT_EDITMSG")
	_ = os.WriteFile(gitInternal, []byte("commit msg"), 0644)
	_ = os.Chtimes(gitInternal, now.Add(1*time.Minute), now.Add(1*time.Minute))

	tmpFile := filepath.Join(repoDir, "scratch.tmp")
	_ = os.WriteFile(tmpFile, []byte("temp content"), 0644)
	_ = os.Chtimes(tmpFile, now.Add(1*time.Minute), now.Add(1*time.Minute))

	resIgnored, err := CheckAndRecord(CheckOptions{
		Dir: repoDir,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("ignored edits run error: %v", err)
	}
	if resIgnored.Allowed {
		t.Errorf("modifying ignored/git files should not unblock quota")
	}

	// Step 4: Modify real source file - should be allowed
	fileModTime := now.Add(2 * time.Minute)
	if err := os.WriteFile(srcFile, []byte("package main\n// edit\n"), 0644); err != nil {
		t.Fatalf("edit main.go: %v", err)
	}
	_ = os.Chtimes(srcFile, fileModTime, fileModTime)

	now = now.Add(3 * time.Minute)
	res3, err := CheckAndRecord(CheckOptions{
		Dir: repoDir,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("third run error: %v", err)
	}
	if !res3.Allowed {
		t.Errorf("run after edit should be allowed, got blocked: %s", res3.Message)
	}
	if res3.ModifiedFile != srcFile {
		t.Errorf("expected ModifiedFile=%q, got %q", srcFile, res3.ModifiedFile)
	}

	// Step 5: Subsequent run without further edits - should be blocked again
	now = now.Add(10 * time.Second)
	res4, err := CheckAndRecord(CheckOptions{
		Dir: repoDir,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("fourth run error: %v", err)
	}
	if res4.Allowed {
		t.Errorf("subsequent run without edits should be blocked again")
	}
}

func TestCheckAndRecord_Bypass(t *testing.T) {
	repoDir := t.TempDir()

	// Initial run
	res1, err := CheckAndRecord(CheckOptions{Dir: repoDir})
	if err != nil || !res1.Allowed {
		t.Fatalf("initial run failed: %v", err)
	}

	// Immediate rerun with QUOTA_BYPASS=1
	resBypass1, err := CheckAndRecord(CheckOptions{
		Dir: repoDir,
		Getenv: func(key string) string {
			if key == "QUOTA_BYPASS" {
				return "1"
			}
			return ""
		},
	})
	if err != nil {
		t.Fatalf("bypass run 1 failed: %v", err)
	}
	if !resBypass1.Allowed {
		t.Errorf("QUOTA_BYPASS=1 should allow execution")
	}

	// Immediate rerun with HARNEZ_QUOTA_BYPASS=true
	resBypass2, err := CheckAndRecord(CheckOptions{
		Dir: repoDir,
		Getenv: func(key string) string {
			if key == "HARNEZ_QUOTA_BYPASS" {
				return "true"
			}
			return ""
		},
	})
	if err != nil {
		t.Fatalf("bypass run 2 failed: %v", err)
	}
	if !resBypass2.Allowed {
		t.Errorf("HARNEZ_QUOTA_BYPASS=true should allow execution")
	}
}

func TestCheckAndRecord_NonGitDir(t *testing.T) {
	nonGitDir := t.TempDir()

	res, err := CheckAndRecord(CheckOptions{Dir: nonGitDir})
	if err != nil {
		t.Fatalf("non-git run error: %v", err)
	}
	if !res.Allowed {
		t.Errorf("first run in non-git dir should be allowed")
	}

	expectedStateFile := filepath.Join(nonGitDir, ".harnez", "quota_1.state")
	if res.StateFile != expectedStateFile {
		t.Errorf("state file = %q, want %q", res.StateFile, expectedStateFile)
	}
}

func TestResolveStateFile_IgnoresJunkGitAncestor(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("mkdir junk .git: %v", err)
	}
	start := filepath.Join(root, "nested")
	if err := os.Mkdir(start, 0755); err != nil {
		t.Fatalf("mkdir start: %v", err)
	}

	stateFile, resolvedRoot, err := ResolveStateFile(start)
	if err != nil {
		t.Fatalf("resolve state file: %v", err)
	}
	wantStateFile := filepath.Join(start, ".harnez", "quota_1.state")
	if stateFile != wantStateFile || resolvedRoot != start {
		t.Fatalf("resolved state = %q, root = %q; want %q, %q", stateFile, resolvedRoot, wantStateFile, start)
	}
}

func TestResolveStateFile_WorktreeGitFileUsesHarnezRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /tmp/real-worktree/.git\n"), 0644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}
	start := filepath.Join(root, "nested")
	if err := os.Mkdir(start, 0755); err != nil {
		t.Fatalf("mkdir start: %v", err)
	}

	stateFile, resolvedRoot, err := ResolveStateFile(start)
	if err != nil {
		t.Fatalf("resolve state file: %v", err)
	}
	wantStateFile := filepath.Join(root, ".harnez", "quota_1.state")
	if stateFile != wantStateFile || resolvedRoot != root {
		t.Fatalf("resolved state = %q, root = %q; want %q, %q", stateFile, resolvedRoot, wantStateFile, root)
	}
}

func TestCheckAndRecord_CustomStateFile(t *testing.T) {
	dir := t.TempDir()
	customState := filepath.Join(dir, "custom.state")

	res, err := CheckAndRecord(CheckOptions{
		Dir:       dir,
		StateFile: customState,
	})
	if err != nil {
		t.Fatalf("custom state error: %v", err)
	}
	if !res.Allowed {
		t.Errorf("first run should be allowed")
	}
	if res.StateFile != customState {
		t.Errorf("state file = %q, want %q", res.StateFile, customState)
	}
	if _, err := os.Stat(customState); err != nil {
		t.Errorf("custom state file was not created: %v", err)
	}
}

func TestCheckAndRecord_CorruptedState(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "corrupt.state")
	_ = os.WriteFile(stateFile, []byte("NOT_A_VALID_TIMESTAMP\n"), 0644)

	res, err := CheckAndRecord(CheckOptions{
		Dir:       dir,
		StateFile: stateFile,
	})
	if err != nil {
		t.Fatalf("expected graceful handle of corrupted state, got error: %v", err)
	}
	if !res.Allowed {
		t.Errorf("corrupted state should be reset and allowed")
	}
}
