package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunClaudeInstructionsHook(t *testing.T) {
	var out bytes.Buffer
	if err := runClaudeInstructionsHook(&out); err != nil {
		t.Fatalf("runClaudeInstructionsHook: %v", err)
	}
	for _, want := range []string{"Never propose or edit the literal CxxxE.md", "Always target AGENTS.md", "check the repository's docs/ and AGENTS.md", "verify-first rule", `explicit "unverified" mark`, "still running", "primary sources"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("instruction output missing %q: %s", want, out.String())
		}
	}
}

func TestRunClaudeInstructionsHookDetachedHeadWarning(t *testing.T) {
	dir := t.TempDir()
	runCmd := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	runCmd("init", "-b", "main")
	runCmd("config", "user.name", "Tester")
	runCmd("config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd("add", "file.txt")
	runCmd("commit", "-m", "init")

	// On a normal branch, verify no warning is emitted and standard instruction line is present.
	t.Chdir(dir)

	var normalOut bytes.Buffer
	if err := runClaudeInstructionsHook(&normalOut); err != nil {
		t.Fatalf("runClaudeInstructionsHook on normal branch: %v", err)
	}
	if strings.Contains(normalOut.String(), "WARNING: HEAD is detached") {
		t.Errorf("did not expect detached HEAD warning on normal branch, got: %s", normalOut.String())
	}
	if !strings.Contains(normalOut.String(), "Harnez instruction rule:") {
		t.Errorf("expected standard instruction line on normal branch, got: %s", normalOut.String())
	}

	// Detach HEAD
	runCmd("checkout", "--detach")

	var out bytes.Buffer
	if err := runClaudeInstructionsHook(&out); err != nil {
		t.Fatalf("runClaudeInstructionsHook: %v", err)
	}
	if !strings.Contains(out.String(), "WARNING: HEAD is detached at") {
		t.Errorf("expected detached HEAD warning in output, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "Switch to a branch (git switch -c <topic> or git switch <branch>) before committing.") {
		t.Errorf("expected recovery command guidance in output, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "Harnez instruction rule:") {
		t.Errorf("expected standard instruction line alongside warning, got: %s", out.String())
	}
}
