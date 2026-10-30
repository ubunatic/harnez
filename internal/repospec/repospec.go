// Package repospec loads spec/repo-config.yaml: which .harnez/repo.yaml keys
// change rendered output, and the effective value of optional keys left out of
// the file. It sits below internal/claude so LoadRepoConfig can read it.
package repospec

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// Spec is the decoded spec/repo-config.yaml.
type Spec struct {
	RenderKeys []string       `yaml:"render_keys"`
	PlainKeys  []string       `yaml:"plain_keys"`
	Defaults   map[string]any `yaml:"defaults"`
}

// Renders reports whether changing key alters rendered output.
func (s Spec) Renders(key string) bool { return slices.Contains(s.RenderKeys, key) }

// BoolDefault returns the spec default of a boolean key; ok is false when the
// spec has no default for it.
func (s Spec) BoolDefault(key string) (value, ok bool) {
	v, found := s.Defaults[key].(bool)
	return v, found
}

var load = sync.OnceValues(func() (Spec, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/repo-config.yaml")
	if err != nil {
		return Spec{}, fmt.Errorf("read spec/repo-config.yaml: %w", err)
	}
	schemaData, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/repo-config.schema.json")
	if err != nil {
		return Spec{}, fmt.Errorf("read repo-config schema: %w", err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return Spec{}, fmt.Errorf("parse spec/repo-config.yaml: %w", err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return Spec{}, fmt.Errorf("parse repo-config schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return Spec{}, fmt.Errorf("resolve repo-config schema: %w", err)
	}
	if err := resolved.Validate(document); err != nil {
		return Spec{}, fmt.Errorf("validate spec/repo-config.yaml: %w", err)
	}
	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return Spec{}, fmt.Errorf("decode spec/repo-config.yaml: %w", err)
	}
	return spec, nil
})

// Load returns the embedded, schema-validated spec.
func Load() (Spec, error) { return load() }
