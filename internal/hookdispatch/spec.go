package hookdispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// Spec mirrors spec/hooks.yaml.
type Spec struct {
	HandlerTimeout string           `yaml:"handler_timeout"`
	ClaudeCompact  claudeCompactRaw `yaml:"claude_compact"`
}

type claudeCompactRaw struct {
	FillRetry []string `yaml:"fill_retry"`
}

// LoadClaudeCompactFillRetry returns the collector's fill schedule for Claude's
// compact boundary: the pause after the PostCompact ingest before each re-read of
// the transcript (issue 1303 M1e). There is no Go default: a missing or
// non-positive value is an error.
func LoadClaudeCompactFillRetry() ([]time.Duration, error) {
	raw, err := harnez.DefaultFS.ReadFile("spec/hooks.yaml")
	if err != nil {
		return nil, fmt.Errorf("load hooks spec: %w", err)
	}
	return parseClaudeCompactFillRetry(raw)
}

// parseClaudeCompactFillRetry validates raw against the hooks schema and parses
// each claude_compact.fill_retry delay. Every delay must be positive.
func parseClaudeCompactFillRetry(raw []byte) ([]time.Duration, error) {
	if err := validateSpec(raw); err != nil {
		return nil, fmt.Errorf("hooks spec: %w", err)
	}
	var s Spec
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parse hooks spec: %w", err)
	}
	out := make([]time.Duration, 0, len(s.ClaudeCompact.FillRetry))
	for i, value := range s.ClaudeCompact.FillRetry {
		d, err := time.ParseDuration(value)
		if err != nil {
			return nil, fmt.Errorf("hooks spec claude_compact.fill_retry[%d] %q: %w", i, value, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("hooks spec claude_compact.fill_retry[%d] must be positive, got %s", i, d)
		}
		out = append(out, d)
	}
	return out, nil
}

// LoadBudget returns the per-handler time budget from the embedded spec/hooks.yaml.
// There is no Go default: a missing or non-positive value is an error.
func LoadBudget() (time.Duration, error) {
	raw, err := harnez.DefaultFS.ReadFile("spec/hooks.yaml")
	if err != nil {
		return 0, fmt.Errorf("load hooks spec: %w", err)
	}
	return parseBudget(raw)
}

// parseBudget validates raw against spec/schemas/hooks.schema.json and returns
// its handler_timeout.
func parseBudget(raw []byte) (time.Duration, error) {
	if err := validateSpec(raw); err != nil {
		return 0, fmt.Errorf("hooks spec: %w", err)
	}
	var s Spec
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return 0, fmt.Errorf("parse hooks spec: %w", err)
	}
	d, err := time.ParseDuration(s.HandlerTimeout)
	if err != nil {
		return 0, fmt.Errorf("hooks spec handler_timeout %q: %w", s.HandlerTimeout, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("hooks spec handler_timeout must be positive, got %s", d)
	}
	return d, nil
}

// validateSpec checks raw against the hooks JSON schema.
func validateSpec(raw []byte) error {
	schemaData, err := harnez.DefaultFS.ReadFile("spec/schemas/hooks.schema.json")
	if err != nil {
		return err
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return err
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return err
	}
	var yamlDoc any
	if err := yaml.NewDecoder(bytes.NewReader(raw)).Decode(&yamlDoc); err != nil {
		return err
	}
	encoded, err := json.Marshal(yamlDoc)
	if err != nil {
		return err
	}
	var doc any
	if err := json.Unmarshal(encoded, &doc); err != nil {
		return err
	}
	return resolved.Validate(doc)
}
