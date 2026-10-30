package repocfg

import (
	"slices"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/repospec"
)

func TestKeysComeFromSchema(t *testing.T) {
	keys, err := Keys()
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, k := range keys {
		paths = append(paths, k.Path)
	}
	want := []string{
		"docs.copies", "docs.studies", "git.default_branch", "git.mode", "git.protected_main",
		"issues.active_cap", "issues.merge_driver", "issues.tracker", "make.container_test",
		"make.scaffold", "rules.quota", "version",
	}
	if !slices.Equal(paths, want) {
		t.Fatalf("keys = %v, want %v", paths, want)
	}
	cases := map[string]struct {
		kind     Kind
		enum     []string
		required bool
	}{
		"git.mode":           {KindString, []string{"solo", "fork", "team"}, false},
		"git.protected_main": {KindBool, nil, false},
		"issues.tracker":     {KindString, []string{"harnez", "none"}, true},
		"issues.active_cap":  {KindInt, nil, false},
		"docs.copies":        {KindStringList, nil, true},
		"version":            {KindInt, []string{"1"}, true},
	}
	for path, tc := range cases {
		k, err := Lookup(path)
		if err != nil {
			t.Fatal(err)
		}
		if k.Kind != tc.kind || !slices.Equal(k.Enum, tc.enum) || k.Required != tc.required {
			t.Errorf("%s = kind %v enum %v required %v; want %v %v %v", path, k.Kind, k.Enum, k.Required, tc.kind, tc.enum, tc.required)
		}
	}
	if k, _ := Lookup("issues.active_cap"); k.Min == nil || *k.Min != 1 {
		t.Errorf("issues.active_cap min = %v, want 1", k.Min)
	}
}

// Every schema key must be classified in spec/repo-config.yaml, so a new
// schema key cannot ship without deciding whether it needs `harnez init`.
func TestEverySchemaKeyIsClassified(t *testing.T) {
	spec, err := repospec.Load()
	if err != nil {
		t.Fatal(err)
	}
	keys, err := Keys()
	if err != nil {
		t.Fatal(err)
	}
	var schemaPaths []string
	for _, k := range keys {
		schemaPaths = append(schemaPaths, k.Path)
	}
	classified := append(slices.Clone(spec.RenderKeys), spec.PlainKeys...)
	slices.Sort(classified)
	if !slices.Equal(schemaPaths, classified) {
		t.Fatalf("render_keys+plain_keys = %v\nschema keys = %v", classified, schemaPaths)
	}
}

func TestLookupUnknownKeyListsValidKeys(t *testing.T) {
	_, err := Lookup("git.nope")
	if err == nil || !strings.Contains(err.Error(), `"git.nope"`) || !strings.Contains(err.Error(), "git.mode") {
		t.Fatalf("error = %v, want the key and the valid keys", err)
	}
}

func TestParseValues(t *testing.T) {
	cases := []struct {
		key, raw string
		want     any
		errPart  string
	}{
		{"git.mode", "team", "team", ""},
		{"git.mode", "banana", nil, "git.mode: allowed values: solo, fork, team"},
		{"git.protected_main", "true", true, ""},
		{"git.protected_main", "maybe", nil, "git.protected_main: want true or false"},
		{"issues.active_cap", "12", 12, ""},
		{"issues.active_cap", "abc", nil, "issues.active_cap: want an integer"},
		{"issues.active_cap", "0", nil, "issues.active_cap: must be at least 1"},
		{"git.default_branch", "trunk", "trunk", ""},
		{"git.default_branch", "", nil, "git.default_branch: must not be empty"},
		{"docs.copies", "golang, bash", []string{"golang", "bash"}, ""},
		{"docs.copies", "", []string{}, ""},
		{"docs.copies", "a,,b", nil, "docs.copies: empty item"},
	}
	for _, tc := range cases {
		k, err := Lookup(tc.key)
		if err != nil {
			t.Fatal(err)
		}
		got, err := k.Parse(tc.raw)
		if tc.errPart != "" {
			if err == nil || !strings.Contains(err.Error(), tc.errPart) {
				t.Errorf("Parse(%s, %q) error = %v, want %q", tc.key, tc.raw, err, tc.errPart)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%s, %q): %v", tc.key, tc.raw, err)
			continue
		}
		if !equalValue(got, tc.want) {
			t.Errorf("Parse(%s, %q) = %#v, want %#v", tc.key, tc.raw, got, tc.want)
		}
	}
}

func equalValue(a, b any) bool {
	if as, ok := a.([]string); ok {
		bs, ok := b.([]string)
		return ok && slices.Equal(as, bs)
	}
	return a == b
}
