package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const configFixture = `version: 1
issues:
  tracker: harnez
  merge_driver: true
docs:
  studies: true
  copies: [golang, bash]
rules:
  quota: true
make:
  container_test: true
`

func configRepo(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if body != "" {
		path := filepath.Join(dir, ".harnez", "repo.yaml")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func repoYAML(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".harnez", "repo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// runConfig executes `harnez config <args>` on a fresh root and returns stdout
// and stderr. The session-tip hook is off so tests never touch real session state.
func runConfig(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCmd()
	root.PersistentPreRunE = nil
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(append([]string{"config"}, args...))
	err = root.Execute()
	return out.String(), errb.String(), err
}

func TestConfigGetSetUnsetList(t *testing.T) {
	dir := configRepo(t, configFixture)

	out, _, err := runConfig(t, "-d", dir, "git.mode")
	if err != nil || out != "solo\n" {
		t.Fatalf("get git.mode = %q, %v; want the solo default", out, err)
	}
	out, _, err = runConfig(t, "-d", dir, "git.mode", "team")
	if err != nil {
		t.Fatal(err)
	}
	if want := "run: harnez init -d " + dir + "\n"; out != want {
		t.Fatalf("set git.mode printed %q, want the init hint %q", out, want)
	}
	if !strings.HasSuffix(repoYAML(t, dir), "git:\n  mode: team\n") {
		t.Fatalf("git.mode not written:\n%s", repoYAML(t, dir))
	}
	out, _, err = runConfig(t, "-d", dir, "git.mode")
	if err != nil || out != "team\n" {
		t.Fatalf("get after set = %q, %v", out, err)
	}
	out, _, err = runConfig(t, "-d", dir, "issues.active_cap", "7")
	if err != nil || out != "" {
		t.Fatalf("plain key set printed %q, %v; want no init hint", out, err)
	}
	out, _, err = runConfig(t, "-d", dir, "--list")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"git.mode=team\n", "issues.active_cap=7\n", "git.protected_main=false (default)\n", "make.scaffold=true (default)\n", "docs.copies=golang,bash\n"} {
		if !strings.Contains(out, line) {
			t.Errorf("--list lacks %q:\n%s", line, out)
		}
	}
	out, _, err = runConfig(t, "-d", dir, "--unset", "git.mode")
	if err != nil || out != "run: harnez init -d "+dir+"\n" {
		t.Fatalf("unset git.mode = %q, %v", out, err)
	}
	if strings.Contains(repoYAML(t, dir), "git:") {
		t.Fatalf("empty git map left behind:\n%s", repoYAML(t, dir))
	}
}

func TestConfigGetUnsetKeyFails(t *testing.T) {
	dir := configRepo(t, configFixture)
	out, _, err := runConfig(t, "-d", dir, "git.default_branch")
	if err == nil || !strings.Contains(err.Error(), "git.default_branch is not set") || out != "" {
		t.Fatalf("get of an unset key = %q, %v; want exit 1 with 'is not set'", out, err)
	}
}

func TestConfigRefusalsLeaveFileUnchanged(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"unknown key", []string{"git.nope", "x"}, []string{`"git.nope"`, "git.mode"}},
		{"unknown key get", []string{"git.nope"}, []string{`"git.nope"`}},
		{"bad enum", []string{"git.mode", "banana"}, []string{"git.mode", "solo, fork, team"}},
		{"wrong type", []string{"issues.merge_driver", "maybe"}, []string{"issues.merge_driver", "true or false"}},
		{"int below minimum", []string{"issues.active_cap", "0"}, []string{"issues.active_cap", "at least 1"}},
		{"negative int after --", []string{"issues.active_cap", "--", "-1"}, []string{"issues.active_cap", "at least 1"}},
		{"protected_main on solo", []string{"git.protected_main", "true"}, []string{"git.protected_main", "solo"}},
		{"unset required", []string{"--unset", "issues.tracker"}, []string{"issues.tracker"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := configRepo(t, configFixture)
			_, _, err := runConfig(t, append([]string{"-d", dir}, tc.args...)...)
			if err == nil {
				t.Fatal("command succeeded, want a refusal")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q lacks %q", err, want)
				}
			}
			if got := repoYAML(t, dir); got != configFixture {
				t.Fatalf("file changed on refusal:\n%s", got)
			}
		})
	}
}

func TestConfigArgumentShapes(t *testing.T) {
	dir := configRepo(t, configFixture)
	for name, args := range map[string][]string{
		"no args":           {},
		"list with key":     {"--list", "git.mode"},
		"unset without key": {"--unset"},
		"unset with value":  {"--unset", "git.mode", "team"},
		"list and unset":    {"--list", "--unset", "git.mode"},
		"three positionals": {"git.mode", "team", "extra"},
	} {
		_, _, err := runConfig(t, append([]string{"-d", dir}, args...)...)
		if err == nil {
			t.Errorf("%s: succeeded, want a usage error", name)
		}
	}
	if got := repoYAML(t, dir); got != configFixture {
		t.Fatalf("file changed:\n%s", got)
	}
}

func TestConfigWithoutRepoYaml(t *testing.T) {
	dir := configRepo(t, "")
	out, _, err := runConfig(t, "-d", dir, "git.mode")
	if err != nil || out != "solo\n" {
		t.Fatalf("get without a file = %q, %v; want the solo default", out, err)
	}
	out, _, err = runConfig(t, "-d", dir, "--list")
	if err != nil || !strings.Contains(out, "git.mode=solo (default)\n") {
		t.Fatalf("--list without a file = %q, %v", out, err)
	}
	for _, args := range [][]string{{"git.mode", "team"}, {"--unset", "git.mode"}} {
		_, _, err = runConfig(t, append([]string{"-d", dir}, args...)...)
		if err == nil || !strings.Contains(err.Error(), "harnez init -d") {
			t.Fatalf("%v without a file: error = %v, want a pointer to harnez init", args, err)
		}
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".harnez")); !os.IsNotExist(statErr) {
		t.Fatalf(".harnez was created: %v", statErr)
	}
}

func TestConfigListValueArrayRoundTrip(t *testing.T) {
	dir := configRepo(t, configFixture)
	if _, _, err := runConfig(t, "-d", dir, "docs.copies", "make,website"); err != nil {
		t.Fatal(err)
	}
	out, _, err := runConfig(t, "-d", dir, "docs.copies")
	if err != nil || out != "make,website\n" {
		t.Fatalf("get docs.copies = %q, %v", out, err)
	}
}
