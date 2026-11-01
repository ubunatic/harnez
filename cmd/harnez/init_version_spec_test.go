package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newScratchGoRepo creates a throwaway git repo with a go.mod, so the
// gorelease doc is auto-detected. An isolated HOME keeps the test away from
// the real user config. tag, when non-empty, is created on an empty commit.
func newScratchGoRepo(t *testing.T, tag string) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/scratch\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "init", "-q")
	if tag == "" {
		return dir
	}
	gitIn(t, dir, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-q", "--allow-empty", "-m", "init")
	gitIn(t, dir, "tag", tag)
	return dir
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// runInitNonInteractive runs `harnez init -d dir -y --no-wizard <args>` through
// the real cobra command and fails the test on error.
func runInitNonInteractive(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := newInitCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"-d", dir, "-y", "--no-wizard"}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init failed: %v\n%s", err, out.String())
	}
}

func TestInitYesSkipsVersionYamlWithoutTags(t *testing.T) {
	dir := newScratchGoRepo(t, "")
	runInitNonInteractive(t, dir)
	// gorelease was auto-detected from go.mod, so its doc must have been copied.
	if _, err := os.Stat(filepath.Join(dir, "docs", "GoRelease.md")); err != nil {
		t.Fatalf("expected gorelease doc to be copied (precondition): %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "version.yaml")); !os.IsNotExist(err) {
		t.Fatalf("version.yaml must not be created by init -y without tags (stat err=%v)", err)
	}
}

func TestInitYesScaffoldsVersionYamlWithTags(t *testing.T) {
	dir := newScratchGoRepo(t, "v0.1.0")
	runInitNonInteractive(t, dir)
	if _, err := os.Stat(filepath.Join(dir, "version.yaml")); err != nil {
		t.Fatalf("tagged repo: expected version.yaml to be scaffolded: %v", err)
	}
}

func TestInitYesExplicitGoreleaseScaffoldsVersionYamlWithoutTags(t *testing.T) {
	dir := newScratchGoRepo(t, "")
	runInitNonInteractive(t, dir, "--docs", "gorelease")
	if _, err := os.Stat(filepath.Join(dir, "version.yaml")); err != nil {
		t.Fatalf("explicit --docs gorelease: expected version.yaml to be scaffolded: %v", err)
	}
}
