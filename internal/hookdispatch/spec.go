package hookdispatch

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// Spec mirrors spec/hooks.yaml.
type Spec struct {
	HandlerTimeout string `yaml:"handler_timeout"`
}

// LoadBudget returns the per-handler time budget from the embedded spec/hooks.yaml.
// There is no Go default: a missing or non-positive value is an error.
func LoadBudget() (time.Duration, error) {
	raw, err := harnez.DefaultFS.ReadFile("spec/hooks.yaml")
	if err != nil {
		return 0, fmt.Errorf("load hooks spec: %w", err)
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
