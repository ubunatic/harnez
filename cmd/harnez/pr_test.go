package main

import (
	"bytes"
	"strings"
	"testing"
)

// prWaitExit runs `harnez pr wait args...` through the root command and returns the exit code
// the process boundary would use, plus the captured stdout and stderr.
func prWaitExit(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	root := newRootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(append([]string{"pr", "wait"}, args...))
	err := root.Execute()
	return exitCodeFromRunError(err), out.String(), errOut.String()
}

func TestPRWaitUsageErrorsExitOne(t *testing.T) {
	// An unconfigured host fails before any network call, so this stays offline.
	code, out, errOut := prWaitExit(t, "https://gitlab.example/o/r/pulls/1")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing", out)
	}
	if !strings.Contains(errOut, "not configured") {
		t.Errorf("stderr = %q, want the unconfigured-host message", errOut)
	}

	if code, _, _ := prWaitExit(t); code != 1 {
		t.Errorf("no argument: exit = %d, want 1", code)
	}
}

func TestPRWaitHelpListsExitCodes(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"pr", "wait", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--remote", "--timeout", "--contains", "merged", "closed without a merge", "still open"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help lacks %q", want)
		}
	}
}
