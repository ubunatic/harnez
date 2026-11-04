// Package forgetoken creates, stores and revokes a read-only Forgejo access token for the forge
// read paths (pr wait, forge tracker mode). Issue 1118.
package forgetoken

import (
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"

	"ubunatic.com/harnez"
)

// Spec holds the settings from spec/forge_token.yaml.
type Spec struct {
	Scopes         []string `yaml:"scopes"`
	NamePrefix     string   `yaml:"name_prefix"`
	KeyringService string   `yaml:"keyring_service"`
	TokenDir       string   `yaml:"token_dir"`
}

// LoadSpec reads and validates spec/forge_token.yaml from the embedded spec FS.
func LoadSpec() (Spec, error) {
	return loadSpecFrom(harnez.DefaultFS, "spec/forge_token.yaml", "spec/schemas/forge_token.schema.json")
}

func loadSpecFrom(fsys fs.FS, specPath, schemaPath string) (Spec, error) {
	data, err := fs.ReadFile(fsys, specPath)
	if err != nil {
		return Spec{}, fmt.Errorf("read %s: %w", specPath, err)
	}
	schemaData, err := fs.ReadFile(fsys, schemaPath)
	if err != nil {
		return Spec{}, fmt.Errorf("read %s: %w", schemaPath, err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return Spec{}, fmt.Errorf("parse %s: %w", specPath, err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return Spec{}, fmt.Errorf("parse %s: %w", schemaPath, err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return Spec{}, fmt.Errorf("resolve %s: %w", schemaPath, err)
	}
	if err := resolved.Validate(document); err != nil {
		return Spec{}, fmt.Errorf("validate %s: %w", specPath, err)
	}
	var s Spec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return Spec{}, fmt.Errorf("decode %s: %w", specPath, err)
	}
	return s, nil
}
