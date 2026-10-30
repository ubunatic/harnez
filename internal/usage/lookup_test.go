package usage

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveExecutablePrefersPATH(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", pathDir)
	pathBinary := filepath.Join(pathDir, "agy")
	writeExecutable(t, pathBinary)
	writeExecutable(t, filepath.Join(home, ".local", "bin", "agy"))

	got, err := resolveExecutable("agy")
	if err != nil {
		t.Fatal(err)
	}
	if got != pathBinary {
		t.Fatalf("resolveExecutable() = %q, want PATH binary %q", got, pathBinary)
	}
}

func TestResolveExecutableFindsUserBinFallbacks(t *testing.T) {
	for _, dir := range []string{".local/bin", "go/bin", ".cargo/bin"} {
		t.Run(dir, func(t *testing.T) {
			home := t.TempDir()
			pathDir := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("PATH", pathDir)
			binary := filepath.Join(home, dir, "agy")
			writeExecutable(t, binary)

			got, err := resolveExecutable("agy")
			if err != nil {
				t.Fatal(err)
			}
			want, err := filepath.Abs(binary)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("resolveExecutable() = %q, want %q", got, want)
			}
		})
	}
}

func TestResolveExecutableReturnsLookPathErrorWhenMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	_, err := resolveExecutable("agy")
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("resolveExecutable() error = %v, want exec.ErrNotFound", err)
	}
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
}
