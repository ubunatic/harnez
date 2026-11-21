package selfinstall

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stageFixture returns a 0700 parent dir and a fake source binary.
func stageFixture(t *testing.T) (parent, source string) {
	t.Helper()
	root := t.TempDir()
	parent = filepath.Join(root, "private space")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	source = filepath.Join(root, "harnez-src")
	if err := os.WriteFile(source, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return parent, source
}

func writeFakeMan(manDir string) error {
	return os.WriteFile(filepath.Join(manDir, "harnez.1"), []byte("man"), 0o644)
}

func TestStageWritesPromoterLayoutAndTouchesNoSharedRoot(t *testing.T) {
	parent, source := stageFixture(t)
	shared := t.TempDir()
	sentinels := map[string]string{}
	for _, env := range []string{"HOME", "GOBIN", "XDG_DATA_HOME"} {
		dir := filepath.Join(shared, env)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "keep"), []byte("sentinel"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(env, dir)
		sentinels[env] = dir
	}

	// No shell runs, so quotes, backticks and $(...) in the name are inert.
	dir := filepath.Join(parent, "stage 'q' `b` $(touch marker)")
	if err := Stage(StageOptions{Dir: dir, Source: source, WriteMan: writeFakeMan}, io.Discard); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(dir, "bin", "harnez")
	data, err := os.ReadFile(bin)
	if err != nil || string(data) != "binary" {
		t.Fatalf("staged binary = %q, %v", data, err)
	}
	for path, want := range map[string]os.FileMode{
		dir: 0o700, filepath.Join(dir, "bin"): 0o755, filepath.Join(dir, "share", "man", "man1"): 0o755, bin: 0o755,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode = %04o, want %04o", path, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "share", "man", "man1", "harnez.1")); err != nil {
		t.Errorf("man page missing: %v", err)
	}
	for env, dir := range sentinels {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != "keep" {
			t.Errorf("%s root changed: %v", env, entries)
		}
	}
}

func TestStageRejectsUnsafeDestinationsAndCreatesNothing(t *testing.T) {
	parent, source := stageFixture(t)
	existing := filepath.Join(parent, "existing")
	if err := os.Mkdir(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(parent), "link")
	if err := os.Symlink(parent, link); err != nil {
		t.Fatal(err)
	}
	permissive := filepath.Join(filepath.Dir(parent), "permissive")
	if err := os.Mkdir(permissive, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(permissive, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ name, dir, want string }{
		{"relative", "relative/path", "absolute clean"},
		{"unclean", parent + "/../escape", "absolute clean"},
		{"root", "/", "new directory"},
		{"existing", existing, "already exists"},
		{"symlink parent", filepath.Join(link, "stage"), "without symlink"},
		{"permissive parent", filepath.Join(permissive, "stage"), "mode 0700"},
		{"missing parent", filepath.Join(parent, "absent", "stage"), "parent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Stage(StageOptions{Dir: tc.dir, Source: source, WriteMan: writeFakeMan}, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Stage(%q) err = %v, want %q", tc.dir, err, tc.want)
			}
		})
	}
	for _, dir := range []string{filepath.Join(link, "stage"), filepath.Join(permissive, "stage")} {
		if _, err := os.Lstat(dir); err == nil {
			t.Errorf("rejected destination %s was created", dir)
		}
	}
}

func TestStageRemovesTheDirectoryOnFailure(t *testing.T) {
	parent, source := stageFixture(t)
	for _, tc := range []struct {
		name   string
		source string
		man    func(string) error
	}{
		{"binary copy fails", filepath.Join(parent, "no-such-binary"), writeFakeMan},
		{"man pages fail", source, func(string) error { return errors.New("man boom") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(parent, strings.ReplaceAll(tc.name, " ", "-"))
			if err := Stage(StageOptions{Dir: dir, Source: tc.source, WriteMan: tc.man}, io.Discard); err == nil {
				t.Fatal("Stage succeeded")
			}
			if _, err := os.Lstat(dir); err == nil {
				t.Errorf("failed stage left %s behind", dir)
			}
		})
	}
}
