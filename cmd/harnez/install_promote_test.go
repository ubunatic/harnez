package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestInstallStagedCommandAndRunnerManGuards(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GOBIN", filepath.Join(home, "gobin"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("TAIA_SESSION_DIR", "")

	manifest, err := expectedManManifest(newRootCmd())
	if err != nil {
		t.Fatalf("expectedManManifest: %v", err)
	}
	if len(manifest) < 2 || !slices.Contains(manifest, "harnez.1") {
		t.Fatalf("unexpected man manifest: %v", manifest)
	}

	// Stage valid tree under 0700 dir.
	staged := filepath.Join(t.TempDir(), "staged-release")
	if err := os.Mkdir(staged, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"bin", filepath.Join("share", "man", "man1")} {
		if err := os.MkdirAll(filepath.Join(staged, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(staged, "bin", "harnez"), []byte("#!/bin/sh\necho harnez\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	manCmd := newRootCmd()
	manCmd.SetArgs([]string{"man", "--dir", filepath.Join(staged, "share", "man", "man1")})
	if err := manCmd.Execute(); err != nil {
		t.Fatalf("generate staged man pages: %v", err)
	}

	// Conflicting flags with --staged must be rejected.
	badCmd := newRootCmd()
	badCmd.SetArgs([]string{"install", "--staged", staged, "--dir", "/tmp/evil"})
	if err := badCmd.Execute(); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("conflicting --dir error = %v", err)
	}

	// Valid host promotion installs into GOBIN + XDG_DATA_HOME/man/man1 + ⚙ symlink.
	installCmd := newRootCmd()
	var out bytes.Buffer
	installCmd.SetOut(&out)
	installCmd.SetErr(&out)
	installCmd.SetArgs([]string{"install", "--staged", staged})
	if err := installCmd.Execute(); err != nil {
		t.Fatalf("install --staged failed: %v\n%s", err, out.String())
	}
	if target, err := os.Readlink(filepath.Join(home, "gobin", "⚙")); err != nil || target != "harnez" {
		t.Fatalf("alias symlink = (%q, %v), want harnez", target, err)
	}
	if _, err := os.Stat(filepath.Join(home, "data", "man", "man1", "harnez.1")); err != nil {
		t.Fatalf("promoted man page missing: %v", err)
	}

	// Runner mode blocks shared `man --install` and `install` while allowing private --dir inside TAIA_SESSION_DIR.
	runnerRoot := filepath.Join(t.TempDir(), "runner-sess")
	if err := os.Mkdir(runnerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", runnerRoot)

	sharedManCmd := newRootCmd()
	sharedManCmd.SetArgs([]string{"man", "--install"})
	if err := sharedManCmd.Execute(); err == nil || !strings.Contains(err.Error(), "host-only") {
		t.Fatalf("runner man --install err = %v, want host-only", err)
	}
	outsideManCmd := newRootCmd()
	outsideManCmd.SetArgs([]string{"man", "--dir", filepath.Join(home, "outside-man")})
	if err := outsideManCmd.Execute(); err == nil || !strings.Contains(err.Error(), "escapes $TAIA_SESSION_DIR") {
		t.Fatalf("runner man --dir outside session err = %v", err)
	}
	// Symlink escape inside TAIA_SESSION_DIR must also be rejected (M2).
	escapeLink := filepath.Join(runnerRoot, "escape-man")
	if err := os.Symlink(filepath.Join(home, "data"), escapeLink); err != nil {
		t.Fatal(err)
	}
	symManCmd := newRootCmd()
	symManCmd.SetArgs([]string{"man", "--dir", filepath.Join(escapeLink, "man1")})
	if err := symManCmd.Execute(); err == nil || !strings.Contains(err.Error(), "escapes $TAIA_SESSION_DIR") {
		t.Fatalf("runner symlink man --dir err = %v", err)
	}
	insideManDir := filepath.Join(runnerRoot, "staged-man")
	insideManCmd := newRootCmd()
	insideManCmd.SetArgs([]string{"man", "--dir", insideManDir})
	if err := insideManCmd.Execute(); err != nil {
		t.Fatalf("runner man --dir inside session failed: %v", err)
	}

	runnerInstallCmd := newRootCmd()
	runnerInstallCmd.SetArgs([]string{"install", "--strategy", "self"})
	if err := runnerInstallCmd.Execute(); err == nil || !strings.Contains(err.Error(), "host-only") {
		t.Fatalf("runner install err = %v, want host-only", err)
	}
	// Dependency install (harnez install neus) must also be blocked in runner mode (M3).
	depInstallCmd := newRootCmd()
	depInstallCmd.SetArgs([]string{"install", "neus"})
	if err := depInstallCmd.Execute(); err == nil || !strings.Contains(err.Error(), "host-only") {
		t.Fatalf("runner dependency install err = %v, want host-only", err)
	}
}

// TestInstallStageInRunnerModeStaysInsideTheSession covers `install --stage`
// (issue 1254): allowed in runner mode inside $TAIA_SESSION_DIR, with man pages
// that match the promoter's manifest, refused outside it, and exclusive of the
// install flags.
func TestInstallStageInRunnerModeStaysInsideTheSession(t *testing.T) {
	runnerRoot := filepath.Join(t.TempDir(), "runner-sess")
	if err := os.Mkdir(runnerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", runnerRoot)

	staged := filepath.Join(runnerRoot, "stage")
	stageCmd := newRootCmd()
	stageCmd.SetArgs([]string{"install", "--stage", staged})
	if err := stageCmd.Execute(); err != nil {
		t.Fatalf("runner install --stage inside session failed: %v", err)
	}
	if info, err := os.Stat(filepath.Join(staged, "bin", "harnez")); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("staged binary = %v, %v", info, err)
	}
	manifest, err := expectedManManifest(newRootCmd())
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(staged, "share", "man", "man1"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if strings.Join(got, ",") != strings.Join(manifest, ",") {
		t.Errorf("staged man pages differ from the promoter manifest:\n got %v\nwant %v", got, manifest)
	}

	outsideParent := filepath.Join(t.TempDir(), "outside")
	if err := os.Mkdir(outsideParent, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(outsideParent, "stage")
	outsideCmd := newRootCmd()
	outsideCmd.SetArgs([]string{"install", "--stage", outside})
	if err := outsideCmd.Execute(); err == nil || !strings.Contains(err.Error(), "escapes $TAIA_SESSION_DIR") {
		t.Fatalf("runner install --stage outside session err = %v", err)
	}
	if _, err := os.Lstat(outside); err == nil {
		t.Errorf("refused stage left %s behind", outside)
	}

	conflictCmd := newRootCmd()
	conflictCmd.SetArgs([]string{"install", "--stage", filepath.Join(runnerRoot, "other"), "--strategy", "self"})
	if err := conflictCmd.Execute(); err == nil || !strings.Contains(err.Error(), "cannot be combined with --stage") {
		t.Fatalf("install --stage --strategy err = %v", err)
	}
}
