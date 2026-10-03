package agy

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// specPath is the embedded AGY tool policy registry; see docs/other/Spec.md.
const specPath = "spec/agy_tools.yaml"

// ToolPolicySpec defines a disallowed tool and its denial explanation.
type ToolPolicySpec struct {
	Reason string `yaml:"reason"`
}

// Spec is the top-level shape of spec/agy_tools.yaml.
type Spec struct {
	DisallowedTools map[string]ToolPolicySpec `yaml:"disallowed_tools"`
}

var (
	specOnce      sync.Once
	cachedSpec    *Spec
	cachedSpecErr error
)

// LoadSpec reads and parses the embedded spec/agy_tools.yaml.
func LoadSpec() (*Spec, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, specPath)
	if err != nil {
		return nil, fmt.Errorf("read embedded %s: %w", specPath, err)
	}
	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("parse embedded %s: %w", specPath, err)
	}
	return &s, nil
}

// MustLoadSpec returns the cached spec or panics if invalid.
func MustLoadSpec() *Spec {
	specOnce.Do(func() {
		cachedSpec, cachedSpecErr = LoadSpec()
	})
	if cachedSpecErr != nil {
		panic(fmt.Sprintf("harnez: embedded %s is invalid: %v", specPath, cachedSpecErr))
	}
	return cachedSpec
}

// NormalizeToolName normalizes a tool name for policy lookups.
func NormalizeToolName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.TrimPrefix(name, "default_api:")
	name = strings.TrimPrefix(name, "cortex_step_type_")
	return name
}

// CheckDisallowedTool checks if a tool name is disallowed, returning its reason if rejected.
func (s *Spec) CheckDisallowedTool(toolName string) (disallowed bool, reason string) {
	if s == nil || len(s.DisallowedTools) == 0 {
		return false, ""
	}
	norm := NormalizeToolName(toolName)
	if policy, ok := s.DisallowedTools[norm]; ok {
		return true, policy.Reason
	}
	return false, ""
}

// IsDisallowedTool is a convenience function checking the embedded spec.
// It fails closed by using MustLoadSpec, ensuring broken embedded specs cannot bypass policy.
func IsDisallowedTool(toolName string) (bool, string) {
	return MustLoadSpec().CheckDisallowedTool(toolName)
}
