package exec

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/disable"
)

// runTwice runs `echo` twice under Quota-1 with no source edit in between and
// returns the second run's exit code and stderr.
func runTwiceUnderQuota1(t *testing.T, repoDir string, session string) (int, string) {
	t.Helper()
	opts := testExecOptions(t)
	opts.Quota1 = true
	opts.Quota1Dir = repoDir
	opts.Getenv = func(k string) string {
		if k == "CLAUDE_CODE_SESSION_ID" {
			return session
		}
		return ""
	}
	var out, errOut bytes.Buffer
	if code, err := runExecWrapper([]string{"echo", "one"}, opts, strings.NewReader(""), &out, &errOut); err != nil || code != 0 {
		t.Fatalf("run 1: code %d err %v stderr %q", code, err, errOut.String())
	}
	out.Reset()
	errOut.Reset()
	code, err := runExecWrapper([]string{"echo", "two"}, opts, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	return code, errOut.String()
}

func newQuota1Repo(t *testing.T, stateSession string) string {
	t.Helper()
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "app.go"), []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if stateSession != "" {
		spec, err := disable.LoadSpec()
		if err != nil {
			t.Fatal(err)
		}
		st := disable.State{Class: "process", Item: "quota-1", SetAt: time.Date(2026, 11, 8, 21, 0, 0, 0, time.UTC), SessionID: stateSession}
		if err := spec.Write(repoDir, st); err != nil {
			t.Fatal(err)
		}
	}
	return repoDir
}

func TestQuota1EnforcedWithoutOwnerSuspension(t *testing.T) {
	code, stderr := runTwiceUnderQuota1(t, newQuota1Repo(t, ""), "s1")
	if code != 1 || !strings.Contains(stderr, "Quota-1: test execution blocked") {
		t.Fatalf("second run: code %d stderr %q, want blocked", code, stderr)
	}
}

func TestQuota1SuspendedByOwnerInSameSession(t *testing.T) {
	code, stderr := runTwiceUnderQuota1(t, newQuota1Repo(t, "s1"), "s1")
	if code != 0 {
		t.Fatalf("second run: code %d stderr %q, want allowed", code, stderr)
	}
	if !strings.Contains(stderr, "suspended by the owner") {
		t.Errorf("stderr %q does not say the owner suspended Quota-1", stderr)
	}
}

// After /enable removes the state, Quota-1 blocks again: the suspended runs
// were still recorded, so a third run without an edit is refused.
func TestQuota1RestoredAfterEnable(t *testing.T) {
	repoDir := newQuota1Repo(t, "s1")
	if code, stderr := runTwiceUnderQuota1(t, repoDir, "s1"); code != 0 {
		t.Fatalf("suspended second run: code %d stderr %q", code, stderr)
	}
	spec, err := disable.LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Remove(repoDir, "process", "quota-1"); err != nil {
		t.Fatal(err)
	}
	opts := testExecOptions(t)
	opts.Quota1 = true
	opts.Quota1Dir = repoDir
	opts.Getenv = func(k string) string {
		if k == "CLAUDE_CODE_SESSION_ID" {
			return "s1"
		}
		return ""
	}
	var out, errOut bytes.Buffer
	code, err := runExecWrapper([]string{"echo", "three"}, opts, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 || !strings.Contains(errOut.String(), "Quota-1: test execution blocked") {
		t.Fatalf("after /enable: code %d stderr %q, want blocked", code, errOut.String())
	}
}

func TestQuota1StaleSuspensionIsEnforcedAndWarns(t *testing.T) {
	code, stderr := runTwiceUnderQuota1(t, newQuota1Repo(t, "old-session"), "s1")
	if code != 1 || !strings.Contains(stderr, "Quota-1: test execution blocked") {
		t.Fatalf("second run: code %d stderr %q, want blocked", code, stderr)
	}
	if !strings.Contains(stderr, "from another session") {
		t.Errorf("stderr %q lacks the stale warning", stderr)
	}
}
