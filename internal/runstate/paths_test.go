package runstate

import (
	"os"
	"path/filepath"
	"testing"

	"ubunatic.com/harnez/internal/sessioncontract"
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
	spool, err := TelemetrySpoolDir(legacy)
	if err != nil || spool != filepath.Join(root, "state", "telemetry-spool") {
		t.Fatalf("spool = %q, %v", spool, err)
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
			if _, err := TelemetrySpoolDir(""); err == nil {
				t.Fatal("TelemetrySpoolDir accepted invalid runner root")
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

func TestUsageStateDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", root)
	dir, err := UsageStateDir("legacy")
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join(root, "state", "usage") {
		t.Fatalf("usage directory: %s", dir)
	}
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("directory mode: %v %v", info, err)
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := UsageStateDir("legacy"); err == nil {
		t.Fatal("accepted permissive descendant")
	}
	t.Setenv("TAIA_SESSION_DIR", "relative")
	if _, err := UsageStateDir("legacy"); err == nil {
		t.Fatal("accepted relative root")
	}
	t.Setenv("TAIA_SESSION_DIR", "")
	if dir, err := UsageStateDir("legacy"); err != nil || dir != "legacy" {
		t.Fatalf("legacy = %q, %v", dir, err)
	}
}

func TestUsageStateRejectsInvalidDescendants(t *testing.T) {
	for _, kind := range []string{"symlink", "permissive", "missing root", "permissive root"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "symlink":
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "state")); err != nil {
					t.Fatal(err)
				}
			case "permissive":
				if err := os.Mkdir(filepath.Join(root, "state"), 0755); err != nil {
					t.Fatal(err)
				}
			case "missing root":
				root = filepath.Join(root, "missing")
			case "permissive root":
				if err := os.Chmod(root, 0755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("TAIA_SESSION_DIR", root)
			if _, err := UsageStateDir("legacy"); err == nil {
				t.Fatal("accepted invalid path")
			}
		})
	}
}

func TestAuxiliaryDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", root)
	for name, resolve := range map[string]func(string) (string, error){"logs": LogDir, "feedback": FeedbackDir, "bench": BenchDir} {
		dir, err := resolve("legacy")
		if err != nil || dir != filepath.Join(root, "state", name) {
			t.Fatalf("%s: %q %v", name, dir, err)
		}
	}
	t.Setenv("TAIA_SESSION_DIR", "relative")
	for _, resolve := range []func(string) (string, error){LogDir, FeedbackDir, BenchDir} {
		if _, err := resolve("legacy"); err == nil {
			t.Fatal("accepted invalid root")
		}
	}
	t.Setenv("TAIA_SESSION_DIR", "")
	for _, resolve := range []func(string) (string, error){LogDir, FeedbackDir, BenchDir} {
		if dir, err := resolve("legacy"); err != nil || dir != "legacy" {
			t.Fatalf("legacy: %q %v", dir, err)
		}
	}
}

func TestRunstateConsumesSessionContractSpec(t *testing.T) {
	contract, err := sessioncontract.Load()
	if err != nil {
		t.Fatal(err)
	}
	if RunnerDirEnv() != contract.SessionDirEnv {
		t.Fatalf("RunnerDirEnv() = %q, want %q", RunnerDirEnv(), contract.SessionDirEnv)
	}
	root := t.TempDir()
	t.Setenv(contract.SessionDirEnv, root)
	want := filepath.Join(append([]string{root}, contract.RunnerPaths.SessionState...)...)
	if got := DefaultSessionStateDir("legacy"); got != want {
		t.Fatalf("DefaultSessionStateDir = %q, want %q", got, want)
	}
}
