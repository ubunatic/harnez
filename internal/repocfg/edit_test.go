package repocfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez/internal/claude"
)

// fixture is hand-written with 2-space indent, comments and a flow list, so a
// round trip shows what the node edit keeps.
const fixture = `# Harnez repo config
version: 1
issues:
  tracker: harnez # recommended
  merge_driver: true
docs:
  studies: true
  copies: [golang, bash]
rules:
  quota: true
make:
  container_test: true
`

func writeFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".harnez", "repo.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readRepo(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".harnez", "repo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func mustKey(t *testing.T, path string) Key {
	t.Helper()
	k, err := Lookup(path)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func set(t *testing.T, dir, path, raw string) (Result, error) {
	t.Helper()
	return Set(dir, mustKey(t, path), raw)
}

func TestSetKeepsCommentsOrderAndIndent(t *testing.T) {
	dir := writeFixture(t, fixture)
	res, err := set(t, dir, "issues.merge_driver", "false")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("Changed = false after a real change")
	}
	want := strings.Replace(fixture, "merge_driver: true", "merge_driver: false", 1)
	if got := readRepo(t, dir); got != want {
		t.Fatalf("file after set:\n%s\nwant:\n%s", got, want)
	}
	// The line comment travels with the replaced value.
	if _, err := set(t, dir, "issues.tracker", "none"); err != nil {
		t.Fatal(err)
	}
	want = strings.Replace(want, "tracker: harnez # recommended", "tracker: none # recommended", 1)
	if got := readRepo(t, dir); got != want {
		t.Fatalf("file after tracker set:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetThenUnsetRestoresBytes(t *testing.T) {
	dir := writeFixture(t, fixture)
	if _, err := set(t, dir, "issues.active_cap", "12"); err != nil {
		t.Fatal(err)
	}
	if got := readRepo(t, dir); !strings.Contains(got, "  tracker: harnez # recommended\n  merge_driver: true\n  active_cap: 12\ndocs:") {
		t.Fatalf("active_cap not appended to the issues map:\n%s", got)
	}
	if _, err := Unset(dir, mustKey(t, "issues.active_cap")); err != nil {
		t.Fatal(err)
	}
	if got := readRepo(t, dir); got != fixture {
		t.Fatalf("after set+unset:\n%s\nwant the original:\n%s", got, fixture)
	}
}

func TestSetListKeepsFlowStyle(t *testing.T) {
	dir := writeFixture(t, fixture)
	if _, err := set(t, dir, "docs.copies", "make, website"); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(fixture, "[golang, bash]", "[make, website]", 1)
	if got := readRepo(t, dir); got != want {
		t.Fatalf("file:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetCreatesMissingParent(t *testing.T) {
	dir := writeFixture(t, fixture)
	if _, err := set(t, dir, "git.mode", "team"); err != nil {
		t.Fatal(err)
	}
	got := readRepo(t, dir)
	if !strings.HasSuffix(got, "make:\n  container_test: true\ngit:\n  mode: team\n") {
		t.Fatalf("git parent not created at the end:\n%s", got)
	}
	if _, err := set(t, dir, "git.protected_main", "true"); err != nil {
		t.Fatal(err)
	}
	if _, err := claude.LoadRepoConfig(dir); err != nil {
		t.Fatalf("result is not a valid repo.yaml: %v", err)
	}
}

func TestSetStringsStayStrings(t *testing.T) {
	for _, value := range []string{"true", "123", "a: b"} {
		dir := writeFixture(t, fixture)
		if _, err := set(t, dir, "git.default_branch", value); err != nil {
			t.Fatalf("%q: %v", value, err)
		}
		var doc struct {
			Git struct {
				DefaultBranch any `yaml:"default_branch"`
			} `yaml:"git"`
		}
		if err := yaml.Unmarshal([]byte(readRepo(t, dir)), &doc); err != nil {
			t.Fatal(err)
		}
		if doc.Git.DefaultBranch != value {
			t.Fatalf("default_branch = %#v, want the string %q", doc.Git.DefaultBranch, value)
		}
	}
}

func TestSetNoChangeLeavesFileAlone(t *testing.T) {
	// The file is hand-formatted in a way a re-encode would alter (blank line).
	body := strings.Replace(fixture, "docs:", "\ndocs:", 1)
	dir := writeFixture(t, body)
	res, err := set(t, dir, "issues.merge_driver", "true")
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed || readRepo(t, dir) != body {
		t.Fatalf("no-op set changed the file (Changed=%v):\n%s", res.Changed, readRepo(t, dir))
	}
}

// Every refusal must leave repo.yaml byte-identical.
func TestRefusedEditsLeaveFileUnchanged(t *testing.T) {
	team := fixture + "git:\n  mode: team\n  protected_main: true\n"
	cases := []struct {
		name  string
		body  string
		do    func(dir string) error
		wants []string
	}{
		{"protected_main on solo default", fixture, func(dir string) error {
			_, err := set(t, dir, "git.protected_main", "true")
			return err
		}, []string{"git.protected_main", "solo"}},
		{"protected_main on explicit solo", fixture + "git:\n  mode: solo\n", func(dir string) error {
			_, err := set(t, dir, "git.protected_main", "true")
			return err
		}, []string{"git.protected_main"}},
		{"mode solo while protected", team, func(dir string) error {
			_, err := set(t, dir, "git.mode", "solo")
			return err
		}, []string{"git.protected_main"}},
		{"unset mode leaves protected on solo default", team, func(dir string) error {
			_, err := Unset(dir, mustKey(t, "git.mode"))
			return err
		}, []string{"git.protected_main"}},
		{"bad enum", fixture, func(dir string) error {
			_, err := set(t, dir, "git.mode", "banana")
			return err
		}, []string{"git.mode", "solo, fork, team"}},
		{"wrong type", fixture, func(dir string) error {
			_, err := set(t, dir, "issues.merge_driver", "maybe")
			return err
		}, []string{"issues.merge_driver", "true or false"}},
		{"int below minimum", fixture, func(dir string) error {
			_, err := set(t, dir, "issues.active_cap", "0")
			return err
		}, []string{"issues.active_cap", "at least 1"}},
		{"unset required scalar", fixture, func(dir string) error {
			_, err := Unset(dir, mustKey(t, "issues.tracker"))
			return err
		}, []string{"issues", "tracker"}},
		{"unset required list", fixture, func(dir string) error {
			_, err := Unset(dir, mustKey(t, "docs.copies"))
			return err
		}, []string{"docs", "copies"}},
		{"unset version", fixture, func(dir string) error {
			_, err := Unset(dir, mustKey(t, "version"))
			return err
		}, []string{"version"}},
		{"unset key that is not set", fixture, func(dir string) error {
			_, err := Unset(dir, mustKey(t, "git.mode"))
			return err
		}, []string{"git.mode", "not set"}},
		{"parent is not a map", strings.Replace(fixture, "make:\n  container_test: true\n", "make:\n  container_test: true\ngit: 5\n", 1), func(dir string) error {
			_, err := set(t, dir, "git.mode", "team")
			return err
		}, []string{"git"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeFixture(t, tc.body)
			err := tc.do(dir)
			if err == nil {
				t.Fatal("edit succeeded, want a refusal")
			}
			for _, want := range tc.wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q lacks %q", err, want)
				}
			}
			if got := readRepo(t, dir); got != tc.body {
				t.Fatalf("file changed on refusal:\n%s", got)
			}
		})
	}
}

func TestUnsetLastGitKeyDropsGitMap(t *testing.T) {
	dir := writeFixture(t, fixture+"git:\n  default_branch: trunk\n")
	if _, err := Unset(dir, mustKey(t, "git.default_branch")); err != nil {
		t.Fatal(err)
	}
	if got := readRepo(t, dir); got != fixture {
		t.Fatalf("empty git map left behind:\n%s", got)
	}
}

func TestEditWithoutRepoYamlIsRefused(t *testing.T) {
	dir := t.TempDir()
	_, err := set(t, dir, "git.mode", "team")
	if err == nil || !strings.Contains(err.Error(), "harnez init -d") {
		t.Fatalf("set error = %v, want a pointer to harnez init", err)
	}
	if _, err := Unset(dir, mustKey(t, "git.mode")); err == nil {
		t.Fatal("unset on a missing file succeeded")
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".harnez")); !os.IsNotExist(statErr) {
		t.Fatalf(".harnez was created: %v", statErr)
	}
}

func TestResultNeedsInitFollowsSpecRenderKeys(t *testing.T) {
	dir := writeFixture(t, fixture)
	res, err := set(t, dir, "git.mode", "team")
	if err != nil || !res.NeedsInit {
		t.Fatalf("git.mode: %+v, %v; want NeedsInit", res, err)
	}
	for _, path := range []string{"issues.active_cap", "git.default_branch"} {
		res, err := set(t, dir, path, map[string]string{"issues.active_cap": "9", "git.default_branch": "trunk"}[path])
		if err != nil || res.NeedsInit {
			t.Fatalf("%s: %+v, %v; want no NeedsInit", path, res, err)
		}
	}
}
