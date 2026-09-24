package quota1

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func finishSuccessfulRun(t *testing.T, res Result, finished time.Time) {
	t.Helper()
	exit := 0
	if err := FinishRun(res.StateFile, finished, &exit); err != nil {
		t.Fatalf("finish quota-1 run: %v", err)
	}
}

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
	finishSuccessfulRun(t, res1, now.Add(time.Second))

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
	finishSuccessfulRun(t, res3, now.Add(time.Second))
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

func TestIncompleteRunAllowsRetryThenConsumesQuota(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "quota.state")
	first, err := CheckAndRecord(CheckOptions{Dir: dir, StateFile: stateFile})
	if err != nil || !first.Allowed {
		t.Fatalf("initial run = %+v, %v; want allowed", first, err)
	}
	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("decode run record: %v", err)
	}
	for _, key := range []string{"started", "pgid", "pid_starttime", "finished", "exit"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("run record missing %q: %s", key, data)
		}
	}
	if err := FinishRun(first.StateFile, time.Now(), nil); err != nil {
		t.Fatalf("mark first run incomplete: %v", err)
	}

	retry, err := CheckAndRecord(CheckOptions{Dir: dir, StateFile: stateFile})
	if err != nil || !retry.Allowed || !strings.Contains(retry.Message, "incomplete") {
		t.Fatalf("retry = %+v, %v; want allowed incomplete retry", retry, err)
	}
	finishSuccessfulRun(t, retry, time.Now())

	blocked, err := CheckAndRecord(CheckOptions{Dir: dir, StateFile: stateFile})
	if err != nil || blocked.Allowed {
		t.Fatalf("run after successful retry = %+v, %v; want quota block", blocked, err)
	}
}

func TestUnfinishedRunBlocksWithCleanupGuidance(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "quota.state")
	started, err := CheckAndRecord(CheckOptions{Dir: dir, StateFile: stateFile})
	if err != nil || !started.Allowed {
		t.Fatalf("initial run = %+v, %v; want allowed", started, err)
	}

	blocked, err := CheckAndRecord(CheckOptions{Dir: dir, StateFile: stateFile})
	if err != nil || blocked.Allowed || !strings.Contains(blocked.Message, "clean procs q1 --kill") {
		t.Fatalf("active run check = %+v, %v; want cleanup guidance", blocked, err)
	}
	state, err := ReadRunRecord(stateFile)
	if err != nil || state.Retry {
		t.Fatalf("active run state = %+v, %v; blocked check must not mark retry", state, err)
	}
}

func TestIncompleteRunOnlyGetsOneRetry(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "quota.state")
	first, err := CheckAndRecord(CheckOptions{Dir: dir, StateFile: stateFile})
	if err != nil || !first.Allowed {
		t.Fatalf("initial run = %+v, %v; want allowed", first, err)
	}
	if err := FinishRun(first.StateFile, time.Now(), nil); err != nil {
		t.Fatal(err)
	}
	retry, err := CheckAndRecord(CheckOptions{Dir: dir, StateFile: stateFile})
	if err != nil || !retry.Allowed {
		t.Fatalf("first retry = %+v, %v; want allowed", retry, err)
	}
	state, err := ReadRunRecord(stateFile)
	if err != nil || !state.Retry {
		t.Fatalf("retry state = %+v, %v; want retry marker", state, err)
	}
	if err := FinishRun(retry.StateFile, time.Now(), nil); err != nil {
		t.Fatal(err)
	}
	secondRetry, err := CheckAndRecord(CheckOptions{Dir: dir, StateFile: stateFile})
	if err != nil || secondRetry.Allowed || !strings.Contains(secondRetry.Message, "test execution blocked") {
		t.Fatalf("second retry = %+v, %v; want source-change block", secondRetry, err)
	}
}

func TestReadRunRecordSupportsLegacyTimestamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quota.state")
	legacyTime := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	if err := writeState(path, legacyTime); err != nil {
		t.Fatal(err)
	}
	record, err := ReadRunRecord(path)
	if err != nil {
		t.Fatalf("read legacy timestamp: %v", err)
	}
	if !record.Started.Equal(legacyTime) || record.Exit == nil || *record.Exit != 0 {
		t.Fatalf("legacy record = %+v; want completed successful run at %s", record, legacyTime)
	}
}

func TestParseProcStarttime(t *testing.T) {
	fields := []string{"S"}
	for field := 4; field <= 21; field++ {
		fields = append(fields, "1")
	}
	fields = append(fields, "987654")
	line := "123 (name with ) parens) " + strings.Join(fields, " ")
	if got := parseProcStarttime(line); got != 987654 {
		t.Fatalf("parseProcStarttime() = %d, want 987654", got)
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

func TestChangesSinceLastRun(t *testing.T) {
	repoDir := t.TempDir()
	if err := exec.Command("git", "-C", repoDir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(repoDir, "changed.go")
	if err := os.WriteFile(source, []byte("package changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repoDir, "add", "changed.go").Run(); err != nil {
		t.Fatal(err)
	}
	runAt := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	state, _, err := ResolveStateFile(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeState(state, runAt); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(source, runAt.Add(time.Minute), runAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	files, gotAt, err := ChangesSinceLastRun(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != source || !gotAt.Equal(runAt) {
		t.Fatalf("ChangesSinceLastRun() = %v, %v; want [%s], %v", files, gotAt, source, runAt)
	}
	ignored := filepath.Join(repoDir, "ignored.go")
	markdown := filepath.Join(repoDir, "docs.md")
	issue := filepath.Join(repoDir, "issues", "525.md")
	for _, path := range []string{ignored, markdown, issue} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("changed\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, runAt.Add(2*time.Minute), runAt.Add(2*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte("ignored.go\n"), 0644); err != nil {
		t.Fatal(err)
	}
	turnStart := runAt.Add(90 * time.Second)
	files, _, err = ChangesSinceLastRunAfter(repoDir, turnStart)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("filtered changes = %v, want none", files)
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
