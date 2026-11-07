package runstate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunnerDirectoriesAreScopedAndPrivate(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", root)

	legacy := filepath.Join(t.TempDir(), ".harnez", "sessions")
	state, err := SessionStateDir(legacy)
	if err != nil {
		t.Fatal(err)
	}
	process, err := ProcessRecordDir(filepath.Join(filepath.Dir(filepath.Dir(legacy)), ".harnez", "run", "procs"))
	if err != nil {
		t.Fatal(err)
	}
	wantState := filepath.Join(root, "state", "sessions")
	wantProcess := filepath.Join(root, "runtime", "procs")
	if state != wantState || process != wantProcess {
		t.Fatalf("runner dirs = %q, %q; want %q, %q", state, process, wantState, wantProcess)
	}
	for _, path := range []string{state, process} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Errorf("mode for %s = %04o, want 0700", path, info.Mode().Perm())
		}
	}
}

func TestRunnerDirectoryRejectsInvalidRoots(t *testing.T) {
	tests := []struct {
		name string
		root func(t *testing.T) string
	}{
		{name: "relative", root: func(t *testing.T) string { return "relative/runner" }},
		{name: "missing", root: func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing") }},
		{name: "not directory", root: func(t *testing.T) string {
			p := filepath.Join(t.TempDir(), "runner")
			if err := os.WriteFile(p, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			return p
		}},
		{name: "permissive", root: func(t *testing.T) string {
			p := t.TempDir()
			if err := os.Chmod(p, 0o755); err != nil {
				t.Fatal(err)
			}
			return p
		}},
		{name: "symlink", root: func(t *testing.T) string {
			target := t.TempDir()
			if err := os.Chmod(target, 0o700); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(t.TempDir(), "runner")
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			return link
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TAIA_SESSION_DIR", test.root(t))
			if _, err := SessionStateDir(""); err == nil {
				t.Fatal("SessionStateDir accepted invalid runner root")
			}
			if _, err := ProcessRecordDir(""); err == nil {
				t.Fatal("ProcessRecordDir accepted invalid runner root")
			}
		})
	}
}

func TestUnsetRunnerDirectoryKeepsLegacyPaths(t *testing.T) {
	t.Setenv("TAIA_SESSION_DIR", "")
	legacyState := filepath.Join(t.TempDir(), ".harnez", "sessions")
	state, err := SessionStateDir(legacyState)
	if err != nil {
		t.Fatal(err)
	}
	legacyProcess := filepath.Join(filepath.Dir(filepath.Dir(legacyState)), ".harnez", "run", "procs")
	process, err := ProcessRecordDir(legacyProcess)
	if err != nil {
		t.Fatal(err)
	}
	if state != legacyState || process != legacyProcess {
		t.Fatalf("legacy paths = %q and %q; want %q and %q", state, process, legacyState, legacyProcess)
	}
}

func TestDifferentRunnerDirectoriesStaySeparate(t *testing.T) {
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	for _, root := range []string{firstRoot, secondRoot} {
		if err := os.Chmod(root, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TAIA_SESSION_DIR", firstRoot)
	first, err := SessionStateDir("")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", secondRoot)
	second, err := SessionStateDir("")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("runner directories collide: %q", first)
	}
}
