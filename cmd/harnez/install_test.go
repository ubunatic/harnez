package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCommandDependency(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, tc := range []struct {
		args    []string
		wantErr string
	}{
		{args: []string{"neus", "--dry-run"}},
		{args: []string{"neus", "--dir", "/tmp/bin"}, wantErr: "flag --dir cannot be used"},
	} {
		cmd := newInstallCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append([]string{}, tc.args...))
		err := cmd.Execute()
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v", err)
			}
		} else if err != nil || out.String() != "go install ubunatic.com/neus/cmd/neus@latest\nneus install\n" {
			t.Fatalf("error = %v, output = %q", err, out.String())
		}
	}
}

func TestInstallCommandLocalSelfInstall(t *testing.T) {
	// The install target must never be a host directory: isolate HOME, GOBIN and
	// GOPATH, and fail if any real bin directory changes during the test.
	hostDirs := hostInstallDirs(t)
	before := snapshotInstallDirs(hostDirs)
	t.Cleanup(func() {
		if after := snapshotInstallDirs(hostDirs); after != before {
			t.Errorf("host bin directories changed during test:\n--- before\n%s--- after\n%s", before, after)
		}
	})
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", filepath.Join(home, "go"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("PATH", "/usr/bin:/bin")
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"install", "--strategy", "self"})
	if err := root.Execute(); err != nil {
		t.Fatalf("install failed: %v\n%s", err, out.String())
	}

	binPath := filepath.Join(home, ".local", "bin", "harnez")
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("installed binary missing at %s: %v", binPath, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "man", "man1", "harnez-install.1")); err != nil {
		t.Fatalf("man page missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".harnez", "completions", "harnez.bash")); err != nil {
		t.Fatalf("bash completion missing: %v", err)
	}
}

// hostInstallDirs lists the real bin directories an install could write to,
// taken from the environment before a test isolates it.
func hostInstallDirs(t *testing.T) []string {
	t.Helper()
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	dirs := []string{
		filepath.Join(realHome, ".local", "bin"),
		filepath.Join(realHome, "go", "bin"),
	}
	if gobin := strings.TrimSpace(os.Getenv("GOBIN")); gobin != "" {
		dirs = append(dirs, gobin)
	}
	if gopath := strings.TrimSpace(os.Getenv("GOPATH")); gopath != "" {
		dirs = append(dirs, filepath.Join(strings.Split(gopath, string(filepath.ListSeparator))[0], "bin"))
	}
	return dirs
}

// snapshotInstallDirs renders each directory's entries, symlink targets and
// modes, so a test can detect any new, removed or retargeted entry.
func snapshotInstallDirs(dirs []string) string {
	var b strings.Builder
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			b.WriteString(dir + ": " + err.Error() + "\n")
			continue
		}
		b.WriteString(dir + ":\n")
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				b.WriteString("  " + e.Name() + ": " + err.Error() + "\n")
				continue
			}
			target, _ := os.Readlink(filepath.Join(dir, e.Name()))
			b.WriteString("  " + e.Name() + " " + info.Mode().String() + " -> " + target + "\n")
		}
	}
	return b.String()
}
