package assess

import (
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

type rampSpec struct {
	Version          string                  `yaml:"version"`
	Levels           map[string]string       `yaml:"levels"`
	Categories       map[string]rampCategory `yaml:"categories"`
	Confidence       map[string]string       `yaml:"confidence"`
	ArtifactStatuses map[string]string       `yaml:"artifact_statuses"`
	Descriptions     struct {
		Levels     map[string]string `yaml:"levels"`
		Categories map[string]string `yaml:"categories"`
	} `yaml:"descriptions"`
}

type rampCategory struct {
	ID    string `yaml:"id"`
	Level string `yaml:"level"`
}

// The vocabulary is embedded application code. An invalid build must fail at
// boot rather than classify repositories using partial or fallback vocabulary.
var rampVocabulary = mustLoadRAMPSpec()

func mustLoadRAMPSpec() rampSpec {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/ramp.yaml")
	if err == nil {
		var spec rampSpec
		spec, err = parseRAMPSpec(data)
		if err == nil {
			return spec
		}
	}
	panic(fmt.Errorf("load embedded spec/ramp.yaml: %w", err))
}

func parseRAMPSpec(data []byte) (rampSpec, error) {
	var spec rampSpec
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return spec, fmt.Errorf("parse spec/ramp.yaml: %w", err)
	}
	schemaData, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/ramp.schema.json")
	if err != nil {
		return spec, fmt.Errorf("read RAMP schema: %w", err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return spec, fmt.Errorf("parse RAMP schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return spec, fmt.Errorf("resolve RAMP schema: %w", err)
	}
	if err := resolved.Validate(document); err != nil {
		return spec, fmt.Errorf("validate spec/ramp.yaml: %w", err)
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return spec, fmt.Errorf("decode spec/ramp.yaml: %w", err)
	}
	categoryIDs := make(map[string]string, len(spec.Categories))
	for key, cat := range spec.Categories {
		categoryIDs[key] = cat.ID
	}
	for group, values := range map[string]map[string]string{
		"levels": spec.Levels, "categories": categoryIDs,
		"confidence": spec.Confidence, "artifact_statuses": spec.ArtifactStatuses,
	} {
		seen := make(map[string]bool, len(values))
		for _, id := range values {
			if seen[id] {
				return spec, fmt.Errorf("spec/ramp.yaml: duplicate %s ID %q", group, id)
			}
			seen[id] = true
		}
	}
	for key, cat := range spec.Categories {
		found := false
		for _, level := range spec.Levels {
			if cat.Level == level {
				found = true
				break
			}
		}
		if !found {
			return spec, fmt.Errorf("spec/ramp.yaml: category %s references unknown level %q", key, cat.Level)
		}
	}
	return spec, nil
}

func categoryLevel(category RAMPCategory) RAMPLevel {
	for _, definition := range rampVocabulary.Categories {
		if definition.ID == string(category) {
			return RAMPLevel(definition.Level)
		}
	}
	return ""
}
