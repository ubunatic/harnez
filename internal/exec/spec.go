package exec

import (
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"

	"ubunatic.com/harnez"
)

// ExecSpec holds the settings from spec/exec.yaml. Issue 771.
type ExecSpec struct {
	Truncate TruncateSpec `yaml:"truncate"`
}

// TruncateSpec is the truncate block: the shown-output cap for harnez exec.
type TruncateSpec struct {
	Enabled        bool   `yaml:"enabled"`
	MaxOutputBytes int64  `yaml:"max_output_bytes"`
	MaxOutputLines int    `yaml:"max_output_lines"`
	SpillDir       string `yaml:"spill_dir"`
	SpillPrefix    string `yaml:"spill_prefix"`
}

// LoadExecSpec reads and validates spec/exec.yaml from the embedded spec FS.
func LoadExecSpec() (ExecSpec, error) {
	return loadExecSpecFrom(harnez.DefaultFS, "spec/exec.yaml", "spec/schemas/exec.schema.json")
}

func loadExecSpecFrom(fsys fs.FS, specPath, schemaPath string) (ExecSpec, error) {
	data, err := fs.ReadFile(fsys, specPath)
	if err != nil {
		return ExecSpec{}, fmt.Errorf("read %s: %w", specPath, err)
	}
	schemaData, err := fs.ReadFile(fsys, schemaPath)
	if err != nil {
		return ExecSpec{}, fmt.Errorf("read %s: %w", schemaPath, err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return ExecSpec{}, fmt.Errorf("parse %s: %w", specPath, err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return ExecSpec{}, fmt.Errorf("parse %s: %w", schemaPath, err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return ExecSpec{}, fmt.Errorf("resolve %s: %w", schemaPath, err)
	}
	if err := resolved.Validate(document); err != nil {
		return ExecSpec{}, fmt.Errorf("validate %s: %w", specPath, err)
	}
	var s ExecSpec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return ExecSpec{}, fmt.Errorf("decode %s: %w", specPath, err)
	}
	return s, nil
}
