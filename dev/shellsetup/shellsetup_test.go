package shellsetup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeInstall returns a New installer whose shell lookup finds only the named shells.
func fakeInstall(t *testing.T, cfg Config, installed ...string) *Installer {
	t.Helper()
	in, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	in.lookPath = func(name string) (string, error) {
		for _, s := range installed {
			if s == name {
				return "/usr/bin/" + name, nil
			}
		}
		return "", errors.New("not found: " + name)
	}
	return in
}

func fakeScript(shell Shell) ([]byte, error) {
	return []byte("# completion for " + string(shell) + "\n"), nil
}

func harnezCompletion() []Completion {
	return []Completion{{Command: "harnez", Script: fakeScript}}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestApplyPathWritesInstalledShellsOnce(t *testing.T) {
	home := t.TempDir()
	bashrc := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(bashrc, []byte("alias ls=ls\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, "go", "bin")
	cfg := Config{Home: home, Owner: "harnez", PathDir: dir}
	in := fakeInstall(t, cfg, "zsh", "fish")

	res, err := in.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 3 {
		t.Fatalf("first run changes = %d, want 3 (zshenv, fish conf.d, bashrc): %+v", len(res.Changes), res.Changes)
	}

	zshenv := mustRead(t, filepath.Join(home, ".zshenv"))
	for _, want := range []string{"# harnez:begin path", `export PATH="$HOME/go/bin:$PATH"`, "# harnez:end path"} {
		if !strings.Contains(zshenv, want) {
			t.Errorf(".zshenv missing %q:\n%s", want, zshenv)
		}
	}
	fish := mustRead(t, filepath.Join(home, ".config", "fish", "conf.d", "harnez-path.fish"))
	if !strings.Contains(fish, `set -gx PATH "$HOME/go/bin" $PATH`) {
		t.Errorf("fish conf.d missing PATH line:\n%s", fish)
	}
	rc := mustRead(t, bashrc)
	if !strings.HasPrefix(rc, "alias ls=ls\n") || !strings.Contains(rc, "# harnez:begin path") {
		t.Errorf(".bashrc should keep its line and gain the path section:\n%s", rc)
	}

	again, err := in.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Changes) != 0 {
		t.Fatalf("second run changes = %v, want none", again.Changes)
	}
}

func TestApplyPathSkipsShellsNotInstalled(t *testing.T) {
	home := t.TempDir()
	in := fakeInstall(t, Config{Home: home, Owner: "harnez", PathDir: filepath.Join(home, "go", "bin")})

	res, err := in.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 0 {
		t.Fatalf("changes = %v, want none when no shell is installed", res.Changes)
	}
	for _, p := range []string{".zshenv", ".bashrc", filepath.Join(".config", "fish")} {
		if _, err := os.Stat(filepath.Join(home, p)); err == nil {
			t.Errorf("%s was created although its shell is not installed", p)
		}
	}
}

func TestApplyPathReportsWhenDirIsNotOnPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PATH", "/usr/bin")
	in := fakeInstall(t, Config{Home: home, Owner: "harnez", PathDir: filepath.Join(home, "go", "bin")})

	res, err := in.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if !res.PathMissing {
		t.Fatal("PathMissing = false, want true when PathDir is not on PATH")
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

func TestCompletionWritesScriptsAndSourcesOnce(t *testing.T) {
	home := t.TempDir()
	zshrc := filepath.Join(home, ".zshrc")
	bashrc := filepath.Join(home, ".bashrc")
	for _, rc := range []string{zshrc, bashrc} {
		if err := os.WriteFile(rc, []byte("export EDITOR=vi\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	in := fakeInstall(t, Config{Home: home, Completions: harnezCompletion()}, "fish")

	res, err := in.Apply()
	if err != nil {
		t.Fatal(err)
	}
	// bash script and rc, zsh script and rc, fish script.
	if len(res.Changes) != 5 {
		t.Fatalf("first run changes = %d, want 5: %+v", len(res.Changes), res.Changes)
	}

	zsh := mustRead(t, zshrc)
	for _, want := range []string{"# harnez:begin completion", "compinit -i", `"$HOME/.harnez/completions/harnez.zsh"`, "export EDITOR=vi"} {
		if !strings.Contains(zsh, want) {
			t.Errorf(".zshrc missing %q:\n%s", want, zsh)
		}
	}
	if got := mustRead(t, filepath.Join(home, ".harnez", "completions", "harnez.bash")); got != "# completion for bash\n" {
		t.Errorf("bash script = %q", got)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "fish", "completions", "harnez.fish")); err != nil {
		t.Errorf("fish autoload file missing: %v", err)
	}

	again, err := in.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Changes) != 0 || len(again.Unchanged) != 5 {
		t.Fatalf("second run changes = %v unchanged = %d, want none and 5", again.Changes, len(again.Unchanged))
	}
}

func TestCompletionNeverCreatesBashOrZshRc(t *testing.T) {
	home := t.TempDir()
	in := fakeInstall(t, Config{Home: home, Completions: harnezCompletion(), Shells: []Shell{Bash, Zsh}})

	res, err := in.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 0 {
		t.Fatalf("changes = %v, want none without .bashrc or .zshrc", res.Changes)
	}
	for _, p := range []string{".bashrc", ".zshrc"} {
		if _, err := os.Stat(filepath.Join(home, p)); err == nil {
			t.Errorf("%s was created by apply", p)
		}
	}
}

func TestShellSelectionLimitsWiring(t *testing.T) {
	home := t.TempDir()
	zshenv := filepath.Join(home, ".zshenv")
	zshrc := filepath.Join(home, ".zshrc")
	for _, f := range []string{zshenv, zshrc} {
		if err := os.WriteFile(f, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := Config{
		Home:        home,
		PathDir:     filepath.Join(home, "go", "bin"),
		Completions: harnezCompletion(),
		Shells:      []Shell{Fish},
	}
	in := fakeInstall(t, cfg, "zsh", "fish")

	res, err := in.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 2 {
		t.Fatalf("changes = %+v, want fish PATH file and fish completion only", res.Changes)
	}
	for _, c := range res.Changes {
		if strings.Contains(c.Path, "zsh") || strings.Contains(c.Path, ".bashrc") {
			t.Errorf("unselected shell was wired: %+v", c)
		}
	}
	if got := mustRead(t, zshenv); got != "" {
		t.Errorf(".zshenv changed although zsh is not selected:\n%s", got)
	}
}

func TestMultipleCommandsEachGetScripts(t *testing.T) {
	home := t.TempDir()
	bashrc := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(bashrc, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Home: home,
		Completions: []Completion{
			{Command: "alpha", Script: fakeScript},
			{Command: "beta", Script: fakeScript},
		},
		Shells: []Shell{Bash},
	}
	in := fakeInstall(t, cfg)

	if _, err := in.Apply(); err != nil {
		t.Fatal(err)
	}
	rc := mustRead(t, bashrc)
	for _, want := range []string{"$HOME/.alpha/completions/alpha.bash", "$HOME/.alpha/completions/beta.bash"} {
		if !strings.Contains(rc, want) {
			t.Errorf(".bashrc missing %q:\n%s", want, rc)
		}
	}
	for _, p := range []string{".alpha/completions/alpha.bash", ".alpha/completions/beta.bash"} {
		if _, err := os.Stat(filepath.Join(home, p)); err != nil {
			t.Errorf("script %s missing: %v", p, err)
		}
	}
}

func TestNothingConfiguredIsNoOp(t *testing.T) {
	home := t.TempDir()
	in := fakeInstall(t, Config{Home: home, Owner: "harnez"}, "zsh", "fish")

	res, err := in.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 0 || len(res.Unchanged) != 0 || in.Pending() {
		t.Fatalf("empty config should do nothing: %+v", res)
	}
	if changes, err := in.Clean(); err != nil || len(changes) != 0 {
		t.Fatalf("Clean = %v, %v; want nothing", changes, err)
	}
}

func TestChecksDetectDriftAndCleanRemoves(t *testing.T) {
	home := t.TempDir()
	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("export EDITOR=vi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, "go", "bin")
	cfg := Config{
		Home:        home,
		Owner:       "harnez",
		PathDir:     dir,
		Completions: harnezCompletion(),
		Shells:      []Shell{Zsh},
	}
	in := fakeInstall(t, cfg, "zsh")

	if !in.Pending() {
		t.Fatal("Pending = false before apply, want true")
	}
	if _, err := in.Apply(); err != nil {
		t.Fatal(err)
	}
	if in.Pending() {
		t.Fatalf("Pending = true right after apply: %+v", in.Checks())
	}

	scriptPath := filepath.Join(home, ".harnez", "completions", "harnez.zsh")
	if err := os.WriteFile(scriptPath, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !in.Pending() {
		t.Fatal("Pending = false after the script was edited")
	}
	found := false
	for _, c := range in.Checks() {
		if c.Label == "~/.harnez/completions/harnez.zsh [completion zsh]" && c.State == "drifted" {
			found = true
		}
	}
	if !found {
		t.Errorf("no drifted check row for the zsh script: %+v", in.Checks())
	}

	changes, err := in.Clean()
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) == 0 {
		t.Fatal("Clean reported no changes")
	}
	if _, err := os.Stat(scriptPath); !os.IsNotExist(err) {
		t.Errorf("script still present after Clean: %v", err)
	}
	rc := mustRead(t, zshrc)
	if strings.Contains(rc, "harnez:") || !strings.Contains(rc, "export EDITOR=vi") {
		t.Errorf("Clean should keep the user's line and drop the sections:\n%s", rc)
	}
	if !in.Pending() {
		t.Error("Pending = false after Clean, want true")
	}
}

func TestCleanRemovesFileThatBecomesEmpty(t *testing.T) {
	home := t.TempDir()
	zshenv := filepath.Join(home, ".zshenv")
	in := fakeInstall(t, Config{Home: home, Owner: "harnez", PathDir: filepath.Join(home, "go", "bin"), Shells: []Shell{Zsh}}, "zsh")
	if _, err := in.Apply(); err != nil {
		t.Fatal(err)
	}

	changes, err := in.Clean()
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Action != "removed" {
		t.Fatalf("changes = %+v, want one removed .zshenv", changes)
	}
	if _, err := os.Stat(zshenv); !os.IsNotExist(err) {
		t.Errorf(".zshenv should be gone when only the managed section was in it: %v", err)
	}
}

func TestApplySectionReplacesInPlace(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".zshrc")
	before := "alias a=b\n# harnez:begin completion\nold body\n# harnez:end completion\nexport X=1\n"
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, existed, err := applySection(path, "harnez", completionSection, "new body\n")
	if err != nil {
		t.Fatal(err)
	}
	if !changed || !existed {
		t.Fatalf("changed = %v existed = %v, want true and true", changed, existed)
	}
	want := "alias a=b\n# harnez:begin completion\nnew body\n# harnez:end completion\nexport X=1\n"
	if got := mustRead(t, path); got != want {
		t.Errorf("section not replaced in place:\n%s\nwant:\n%s", got, want)
	}
}

func TestNewRejectsBadConfig(t *testing.T) {
	home := t.TempDir()
	ok := harnezCompletion()
	cases := map[string]Config{
		"unknown shell":          {Home: home, Shells: []Shell{"tcsh"}},
		"shell listed twice":     {Home: home, Shells: []Shell{Zsh, Zsh}},
		"path without owner":     {Home: home, PathDir: "/x"},
		"bad owner":              {Home: home, Owner: "a/b"},
		"bad command name":       {Home: home, Completions: []Completion{{Command: "a b", Script: fakeScript}}},
		"duplicate command":      {Home: home, Completions: []Completion{ok[0], ok[0]}},
		"command without script": {Home: home, Completions: []Completion{{Command: "harnez"}}},
	}
	for name, cfg := range cases {
		if _, err := New(cfg); err == nil {
			t.Errorf("%s: New accepted the config", name)
		}
	}
}

func TestNewDefaultsOwnerToFirstCommand(t *testing.T) {
	home := t.TempDir()
	in, err := New(Config{Home: home, Completions: harnezCompletion(), PathDir: "/x"})
	if err != nil {
		t.Fatal(err)
	}
	if in.owner != "harnez" {
		t.Errorf("owner = %q, want harnez", in.owner)
	}
}
