package modelcatalog_test

import (
	"io/fs"
	"testing"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/modelcatalog"
)

// TestLoadThroughPublicPackage loads the catalog through the public import path
// from outside internal/. XDG dirs point at temp dirs so the host account's
// machine state and policy cannot change the result.
func TestLoadThroughPublicPackage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	catalog, err := modelcatalog.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if catalog == nil {
		t.Fatal("Load returned a nil catalog without an error")
	}
	// Expected identities come from spec/agent.yaml (issue 1252: no pinned models).
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/agent.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Models map[string]struct {
			Provider string `yaml:"provider"`
			Name     string `yaml:"name"`
			Tier     string `yaml:"tier"`
			Disabled bool   `yaml:"disabled"`
		} `yaml:"models"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	defaults := 0
	found := map[string]modelcatalog.Model{}
	for _, model := range catalog.Models() {
		if model.Source == "spec-default" {
			defaults++
			found[model.Identity.Provider+"\x00"+model.Identity.Model] = model
		}
	}
	if defaults == 0 {
		t.Fatalf("Load returned no spec-default models; got %d models in total", len(catalog.Models()))
	}
	// Identity is checked field by field, so a loader that fabricates
	// defaults with the right count still fails.
	for key, want := range spec.Models {
		if want.Disabled {
			continue
		}
		got, ok := found[want.Provider+"\x00"+want.Name]
		if !ok {
			t.Errorf("%s: no spec-default model %s:%s", key, want.Provider, want.Name)
			continue
		}
		if got.Identity.Tier != want.Tier {
			t.Errorf("%s: tier %q, want spec tier %q", key, got.Identity.Tier, want.Tier)
		}
	}
}
