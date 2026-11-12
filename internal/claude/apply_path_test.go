package claude

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeShells replaces lookPath so only the named shells count as installed.
func fakeShells(t *testing.T, installed ...string) {
	t.Helper()
	prev := lookPath
	t.Cleanup(func() { lookPath = prev })
	lookPath = func(name string) (string, error) {
		for _, s := range installed {
			if s == name {
				return "/usr/bin/" + name, nil
			}
		}
		return "", errors.New("not found: " + name)
	}
}

func TestApplyPathIntegrationWritesInstalledShellsOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakeShells(t, "zsh", "fish")
	bashrc := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(bashrc, []byte("alias ls=ls\n"), 0644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, "go", "bin")

	r := &applyReport{}
	if err := applyPathIntegration(r, home, dir); err != nil {
		t.Fatal(err)
	}
	if r.changes != 3 {
		t.Fatalf("first run changes = %d, want 3 (zshenv, fish conf.d, bashrc)", r.changes)
	}

	zshenv, err := os.ReadFile(filepath.Join(home, ".zshenv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# harnez:begin path", `export PATH="$HOME/go/bin:$PATH"`, "# harnez:end path"} {
		if !strings.Contains(string(zshenv), want) {
			t.Errorf(".zshenv missing %q:\n%s", want, zshenv)
		}
	}

	fish, err := os.ReadFile(filepath.Join(home, ".config", "fish", "conf.d", fishPathFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fish), `set -gx PATH "$HOME/go/bin" $PATH`) {
		t.Errorf("fish conf.d missing PATH line:\n%s", fish)
	}

	rc, err := os.ReadFile(bashrc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(rc), "alias ls=ls\n") || !strings.Contains(string(rc), "# harnez:begin path") {
		t.Errorf(".bashrc should keep its line and gain the path section:\n%s", rc)
	}

	again := &applyReport{}
	if err := applyPathIntegration(again, home, dir); err != nil {
		t.Fatal(err)
	}
	if again.changes != 0 {
		t.Fatalf("second run changes = %d, want 0", again.changes)
	}
}

func TestApplyPathIntegrationSkipsShellsNotInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakeShells(t)

	r := &applyReport{}
	if err := applyPathIntegration(r, home, filepath.Join(home, "go", "bin")); err != nil {
		t.Fatal(err)
	}
	if r.changes != 0 {
		t.Fatalf("changes = %d, want 0 when no shell is installed", r.changes)
	}
	for _, p := range []string{".zshenv", ".bashrc", filepath.Join(".config", "fish")} {
		if _, err := os.Stat(filepath.Join(home, p)); err == nil {
			t.Errorf("%s was created although its shell is not installed", p)
		}
	}
}

func TestPathRefUsesHomeOnlyUnderHome(t *testing.T) {
	cases := map[string]string{
		"/home/u/go/bin": "$HOME/go/bin",
		"/home/u":        "$HOME",
		"/home/uu/bin":   "/home/uu/bin",
		"/opt/tools/bin": "/opt/tools/bin",
	}
	for dir, want := range cases {
		if got := pathRef(dir, "/home/u"); got != want {
			t.Errorf("pathRef(%q) = %q, want %q", dir, got, want)
		}
	}
}

func TestPathEntryDirSkipsSystemBinDirs(t *testing.T) {
	if got := pathEntryDir("/usr/bin"); got != "" {
		t.Errorf("pathEntryDir(/usr/bin) = %q, want empty", got)
	}
	if got := pathEntryDir("/home/u/go/bin"); got != "/home/u/go/bin" {
		t.Errorf("pathEntryDir(go/bin) = %q", got)
	}
}

func TestDiffPathIntegrationDetectsMissingAndCleanup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakeShells(t, "zsh", "fish")
	dir := filepath.Join(home, "go", "bin")

	if !diffPathIntegration(home, dir) {
		t.Fatal("diff should report a change before apply writes the entry")
	}
	if err := applyPathIntegration(&applyReport{}, home, dir); err != nil {
		t.Fatal(err)
	}
	if diffPathIntegration(home, dir) {
		t.Fatal("diff should be clean after apply")
	}

	cleanPathIntegration(home)
	if _, err := os.Stat(filepath.Join(home, ".config", "fish", "conf.d", fishPathFile)); err == nil {
		t.Error("fish conf.d file should be removed by revert")
	}
	// The section was the only content, so revert may remove the file entirely.
	zshenv, err := os.ReadFile(filepath.Join(home, ".zshenv"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if strings.Contains(string(zshenv), "harnez:begin path") {
		t.Errorf(".zshenv still has the path section after cleanup:\n%s", zshenv)
	}
}
