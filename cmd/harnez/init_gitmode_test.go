package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/claude"
)

// runInitCmd runs `harnez init -d dir --force --yes --no-wizard <args>` with an
// isolated HOME and returns the command error.
func runInitCmd(t *testing.T, dir string, args ...string) error {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	cmd := newInitCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"-d", dir, "--force", "--yes", "--no-wizard"}, args...))
	return cmd.Execute()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const protectedLine = "main is protected; harnez refuses commits on it"

func TestInitRepoModeWritesRepoYAML(t *testing.T) {
	dir := t.TempDir()
	if err := runInitCmd(t, dir, "--repo-mode", "team", "--protected-main"); err != nil {
		t.Fatal(err)
	}
	yaml := readFile(t, filepath.Join(dir, ".harnez", "repo.yaml"))
	for _, want := range []string{"git:", "mode: team", "protected_main: true"} {
		if !strings.Contains(yaml, want) {
			t.Errorf("repo.yaml lacks %q:\n%s", want, yaml)
		}
	}
	cfg, err := claude.LoadRepoConfig(dir)
	if err != nil {
		t.Fatalf("written repo.yaml must validate: %v", err)
	}
	if cfg.Git.Mode != "team" || !cfg.Git.ProtectedMain {
		t.Errorf("loaded git config = %+v", cfg.Git)
	}
	if agents := readFile(t, filepath.Join(dir, "AGENTS.md")); !strings.Contains(agents, protectedLine) {
		t.Errorf("AGENTS.md lacks the team_protected section:\n%s", agents)
	}
	if index := readFile(t, filepath.Join(dir, ".harnez", "rules", "Index.md")); !strings.Contains(index, "Git workflow: team, protected main") {
		t.Errorf("Index.md lacks the git workflow line:\n%s", index)
	}
}

func TestInitGitModeAliasMatchesRepoMode(t *testing.T) {
	dir := t.TempDir()
	if err := runInitCmd(t, dir, "--git-mode", "fork"); err != nil {
		t.Fatal(err)
	}
	cfg, err := claude.LoadRepoConfig(dir)
	if err != nil || cfg.Git.Mode != "fork" || cfg.Git.ProtectedMain {
		t.Fatalf("--git-mode fork: cfg=%+v err=%v", cfg, err)
	}
}

func TestInitForkProtectedReusesForkText(t *testing.T) {
	dir := t.TempDir()
	if err := runInitCmd(t, dir, "--repo-mode", "fork", "--protected-main"); err != nil {
		t.Fatal(err)
	}
	agents := readFile(t, filepath.Join(dir, "AGENTS.md"))
	for _, want := range []string{"This repo is a fork.", protectedLine} {
		if !strings.Contains(agents, want) {
			t.Errorf("AGENTS.md lacks %q:\n%s", want, agents)
		}
	}
}

func TestInitRendersFromRepoYAMLWithoutFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".harnez"), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "version: 1\nissues:\n  tracker: none\n  merge_driver: false\ndocs:\n  studies: false\n  copies: []\nrules:\n  quota: false\nmake:\n  scaffold: false\n  container_test: false\ngit:\n  mode: team\n  protected_main: true\n"
	if err := os.WriteFile(filepath.Join(dir, ".harnez", "repo.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInitCmd(t, dir); err != nil {
		t.Fatal(err)
	}
	if agents := readFile(t, filepath.Join(dir, "AGENTS.md")); !strings.Contains(agents, protectedLine) {
		t.Errorf("AGENTS.md lacks the team_protected section:\n%s", agents)
	}
	if got := readFile(t, filepath.Join(dir, ".harnez", "repo.yaml")); got != yaml {
		t.Errorf("init without flags rewrote repo.yaml:\n%s", got)
	}
}

func TestInitRepoModeSoloClearsProtection(t *testing.T) {
	dir := t.TempDir()
	if err := runInitCmd(t, dir, "--repo-mode", "team", "--protected-main"); err != nil {
		t.Fatal(err)
	}
	if err := runInitCmd(t, dir, "--repo-mode", "solo"); err != nil {
		t.Fatalf("switching to solo: %v", err)
	}
	cfg, err := claude.LoadRepoConfig(dir)
	if err != nil || cfg.Git.Mode != "solo" || cfg.Git.ProtectedMain {
		t.Fatalf("after --repo-mode solo: cfg=%+v err=%v", cfg.Git, err)
	}
	if agents := readFile(t, filepath.Join(dir, "AGENTS.md")); strings.Contains(agents, protectedLine) {
		t.Errorf("AGENTS.md still carries the protected line:\n%s", agents)
	}
}

func TestInitProtectedMainOnSoloRejected(t *testing.T) {
	for name, args := range map[string][]string{
		"default mode":  {"--protected-main"},
		"explicit solo": {"--repo-mode", "solo", "--protected-main"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			err := runInitCmd(t, dir, args...)
			if err == nil || !strings.Contains(err.Error(), "--protected-main") || !strings.Contains(err.Error(), "team or fork") {
				t.Fatalf("want a flag error naming --protected-main and team or fork, got %v", err)
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Errorf("rejected init wrote files: %v", entries)
			}
		})
	}
}

func TestInitUnknownRepoModeWritesNothing(t *testing.T) {
	dir := t.TempDir()
	err := runInitCmd(t, dir, "--repo-mode", "trunk")
	if err == nil || !strings.Contains(err.Error(), `unknown repo mode "trunk"`) {
		t.Fatalf("want an unknown repo mode error, got %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("rejected init wrote files: %v", entries)
	}
}

// TestInitSoloGoldenUnchanged pins a plain `init` (no git flags, no git key):
// the golden files were recorded before the git mode work and must not drift.
func TestInitSoloGoldenUnchanged(t *testing.T) {
	dir := t.TempDir()
	if err := runInitCmd(t, dir); err != nil {
		t.Fatal(err)
	}
	for golden, rel := range map[string]string{
		"AGENTS.md": "AGENTS.md",
		"Index.md":  filepath.Join(".harnez", "rules", "Index.md"),
		"repo.yaml": filepath.Join(".harnez", "repo.yaml"),
	} {
		want := readFile(t, filepath.Join("testdata", "init_solo", golden))
		if got := readFile(t, filepath.Join(dir, rel)); got != want {
			t.Errorf("%s drifted from testdata/init_solo/%s:\ngot:\n%s\nwant:\n%s", rel, golden, got, want)
		}
	}
}
