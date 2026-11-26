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
	BoundaryWindow string `yaml:"boundary_window"`
	BoundaryWait   string `yaml:"boundary_wait"`
	BoundaryPoll   string `yaml:"boundary_poll"`
}

// ClaudeCompactWait is the parsed claude_compact section of spec/hooks.yaml: how
// long a PostCompact hook may look for Claude's compact boundary (issue 1303 M1b).
type ClaudeCompactWait struct {
	Window time.Duration
	Wait   time.Duration
	Poll   time.Duration
}

// LoadClaudeCompactWait returns the claude_compact bounds from the embedded spec.
// There is no Go default: a missing or inconsistent value is an error.
func LoadClaudeCompactWait() (ClaudeCompactWait, error) {
	raw, err := harnez.DefaultFS.ReadFile("spec/hooks.yaml")
	if err != nil {
		return ClaudeCompactWait{}, fmt.Errorf("load hooks spec: %w", err)
	}
	return parseClaudeCompactWait(raw)
}

// parseClaudeCompactWait validates raw against the hooks schema and parses the
// claude_compact durations. The wait must be at least one poll, or the loop never reads.
func parseClaudeCompactWait(raw []byte) (ClaudeCompactWait, error) {
	if err := validateSpec(raw); err != nil {
		return ClaudeCompactWait{}, fmt.Errorf("hooks spec: %w", err)
	}
	var s Spec
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return ClaudeCompactWait{}, fmt.Errorf("parse hooks spec: %w", err)
	}
	var out ClaudeCompactWait
	for name, dst := range map[string]struct {
		value string
		out   *time.Duration
	}{
		"boundary_window": {s.ClaudeCompact.BoundaryWindow, &out.Window},
		"boundary_wait":   {s.ClaudeCompact.BoundaryWait, &out.Wait},
		"boundary_poll":   {s.ClaudeCompact.BoundaryPoll, &out.Poll},
	} {
		d, err := time.ParseDuration(dst.value)
		if err != nil {
			return ClaudeCompactWait{}, fmt.Errorf("hooks spec claude_compact.%s %q: %w", name, dst.value, err)
		}
		if d <= 0 {
			return ClaudeCompactWait{}, fmt.Errorf("hooks spec claude_compact.%s must be positive, got %s", name, d)
		}
		*dst.out = d
	}
	if out.Wait < out.Poll {
		return ClaudeCompactWait{}, fmt.Errorf("hooks spec claude_compact.boundary_wait %s is shorter than boundary_poll %s", out.Wait, out.Poll)
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
