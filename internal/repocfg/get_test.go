package repocfg

import (
	"strconv"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/issues"
)

func TestGetFileValueAndDefaults(t *testing.T) {
	cap, err := issues.ActiveIssueCap()
	if err != nil {
		t.Fatal(err)
	}
	dir := writeFixture(t, fixture+"git:\n  mode: team\n")
	cases := []struct {
		key     string
		want    string
		set     bool
		deflt   bool
		wantStr string
	}{
		{"issues.tracker", "harnez", true, false, "issues.tracker=harnez"},
		{"docs.copies", "golang,bash", true, false, "docs.copies=golang,bash"},
		{"git.mode", "team", true, false, "git.mode=team"},
		{"git.protected_main", "false", true, true, "git.protected_main=false (default)"},
		{"issues.active_cap", strconv.Itoa(cap), true, true, "issues.active_cap=" + strconv.Itoa(cap) + " (default)"},
		{"make.scaffold", "true", true, true, "make.scaffold=true (default)"},
		{"git.default_branch", "", false, false, ""},
	}
	for _, tc := range cases {
		e, err := Get(dir, mustKey(t, tc.key))
		if err != nil {
			t.Fatalf("%s: %v", tc.key, err)
		}
		if e.Set != tc.set || e.Value != tc.want || e.Default != tc.deflt || (tc.set && e.String() != tc.wantStr) {
			t.Errorf("%s = %+v (%q); want value %q set %v default %v", tc.key, e, e.String(), tc.want, tc.set, tc.deflt)
		}
	}
}

func TestGetExplicitValueIsNotMarkedDefault(t *testing.T) {
	dir := writeFixture(t, strings.Replace(fixture, "make:\n", "make:\n  scaffold: false\n", 1))
	e, err := Get(dir, mustKey(t, "make.scaffold"))
	if err != nil || e.Value != "false" || e.Default {
		t.Fatalf("make.scaffold = %+v, %v; want an explicit false", e, err)
	}
}

func TestGetWithoutRepoYamlUsesSpecDefaults(t *testing.T) {
	dir := t.TempDir()
	e, err := Get(dir, mustKey(t, "git.mode"))
	if err != nil || !e.Set || e.Value != "solo" || !e.Default {
		t.Fatalf("git.mode = %+v, %v; want solo (default)", e, err)
	}
	e, err = Get(dir, mustKey(t, "issues.tracker"))
	if err != nil || e.Set {
		t.Fatalf("issues.tracker = %+v, %v; want not set", e, err)
	}
}

func TestListShowsEffectiveValuesWithDefaultsMarked(t *testing.T) {
	cap, err := issues.ActiveIssueCap()
	if err != nil {
		t.Fatal(err)
	}
	dir := writeFixture(t, fixture+"git:\n  mode: team\n  protected_main: true\n")
	entries, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, e := range entries {
		lines = append(lines, e.String())
	}
	want := []string{
		"docs.copies=golang,bash",
		"docs.studies=true",
		"git.mode=team",
		"git.protected_main=true",
		"issues.active_cap=" + strconv.Itoa(cap) + " (default)",
		"issues.merge_driver=true",
		"issues.tracker=harnez",
		"make.container_test=true",
		"make.scaffold=true (default)",
		"rules.quota=true",
		"version=1",
	}
	if got := strings.Join(lines, "\n"); got != strings.Join(want, "\n") {
		t.Fatalf("list:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
}

func TestListWithoutRepoYamlShowsOnlyDefaults(t *testing.T) {
	entries, err := List(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.Default {
			t.Errorf("%s has a value without a repo.yaml", e)
		}
	}
	if len(entries) == 0 {
		t.Fatal("no defaults listed")
	}
}
