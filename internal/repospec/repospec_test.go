package repospec

import (
	"slices"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	spec, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := spec.BoolDefault("make.scaffold")
	if !ok || !got {
		t.Fatalf("make.scaffold default = %v, %v; want true, true", got, ok)
	}
	if _, ok := spec.BoolDefault("git.mode"); ok {
		t.Fatal("git.mode default belongs to spec/git.yaml, not repo-config.yaml")
	}
}

func TestLoadKeyClasses(t *testing.T) {
	spec, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !spec.Renders("git.mode") || !spec.Renders("make.scaffold") {
		t.Fatalf("git.mode and make.scaffold must be render keys: %v", spec.RenderKeys)
	}
	if spec.Renders("issues.active_cap") || spec.Renders("git.default_branch") || spec.Renders("version") {
		t.Fatalf("plain keys must not render: %v", spec.RenderKeys)
	}
	for _, key := range spec.RenderKeys {
		if slices.Contains(spec.PlainKeys, key) {
			t.Fatalf("%s is in both render_keys and plain_keys", key)
		}
	}
}
