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
	home := t.TempDir()
	t.Setenv("HOME", home)
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
