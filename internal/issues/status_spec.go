package issues

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

const lifecycleSpecPath = "spec/issues.yaml"

type lifecycleSpec struct {
	Lifecycle     lifecycleDefinition     `yaml:"lifecycle"`
	DuplicateGate duplicateGateDefinition `yaml:"duplicate_gate"`
}

type duplicateGateDefinition struct {
	Threshold          float64  `yaml:"threshold"`
	MaxResults         int      `yaml:"max_results"`
	MinimumQueryTokens int      `yaml:"minimum_query_tokens"`
	MinimumTokenLength int      `yaml:"minimum_token_length"`
	TitleWeight        int      `yaml:"title_weight"`
	BodyWeight         int      `yaml:"body_weight"`
	Stopwords          []string `yaml:"stopwords"`
}

// DuplicateGatePolicy is the spec-owned threshold and tokenization policy
// used by `find issues similar` and the `issues new` duplicate gate.
type DuplicateGatePolicy struct {
	Threshold          float64
	MaxResults         int
	MinimumQueryTokens int
	MinimumTokenLength int
	TitleWeight        int
	BodyWeight         int
	Stopwords          []string
}

type lifecycleDefinition struct {
	Unresolved    []string     `yaml:"unresolved"`
	FallbackOrder []string     `yaml:"fallback_order"`
	Statuses      []statusRule `yaml:"statuses"`
}

type statusRule struct {
	Key           string         `yaml:"key"`
	Label         string         `yaml:"label"`
	Category      StatusCategory `yaml:"category"`
	Filters       []string       `yaml:"filters"`
	MatchMode     string         `yaml:"match_mode"`
	MatchTerms    []string       `yaml:"match_terms"`
	FallbackTerms []string       `yaml:"fallback_terms,omitempty"`
}

func loadLifecycleSpec() (lifecycleSpec, error) {
	data, err := harnez.DefaultFS.ReadFile(lifecycleSpecPath)
	if err != nil {
		return lifecycleSpec{}, fmt.Errorf("issues lifecycle spec: read %s: %w", lifecycleSpecPath, err)
	}
	var spec lifecycleSpec
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&spec); err != nil {
		return lifecycleSpec{}, fmt.Errorf("issues lifecycle spec: parse: %w", err)
	}
	if err := validateLifecycleSpec(spec); err != nil {
		return lifecycleSpec{}, err
	}
	return spec, nil
}

var lifecycleSpecOnce = sync.OnceValues(loadLifecycleSpec)

func currentLifecycleSpec() (lifecycleSpec, error) {
	return lifecycleSpecOnce()
}

func validateLifecycleSpec(spec lifecycleSpec) error {
	if len(spec.Lifecycle.Statuses) == 0 || len(spec.Lifecycle.Unresolved) == 0 || len(spec.Lifecycle.FallbackOrder) == 0 {
		return fmt.Errorf("issues lifecycle spec: statuses, unresolved, and fallback_order must not be empty")
	}
	keys := make(map[string]bool)
	filters := make(map[string]bool)
	for _, status := range spec.Lifecycle.Statuses {
		if status.Key == "" || status.Label == "" || len(status.MatchTerms) == 0 {
			return fmt.Errorf("issues lifecycle spec: every status needs key, label, and match_terms")
		}
		if keys[status.Key] {
			return fmt.Errorf("issues lifecycle spec: duplicate status key %q", status.Key)
		}
		keys[status.Key] = true
		if status.Category != StatusOpen && status.Category != StatusClosed && status.Category != StatusDraft && status.Category != StatusBacklog {
			return fmt.Errorf("issues lifecycle spec: status %q has unknown category %q", status.Key, status.Category)
		}
		switch status.MatchMode {
		case "lead_contains", "lead_exact", "lead_exact_dash", "lead_token":
		default:
			return fmt.Errorf("issues lifecycle spec: status %q has unknown match_mode %q", status.Key, status.MatchMode)
		}
		for _, filter := range status.Filters {
			if filter == "" {
				return fmt.Errorf("issues lifecycle spec: status %q has an empty filter", status.Key)
			}
			filters[filter] = true
		}
	}
	for _, key := range spec.Lifecycle.Unresolved {
		if !keys[key] {
			return fmt.Errorf("issues lifecycle spec: unresolved set references unknown status %q", key)
		}
	}
	for _, key := range spec.Lifecycle.FallbackOrder {
		if !keys[key] {
			return fmt.Errorf("issues lifecycle spec: fallback order references unknown status %q", key)
		}
	}
	gate := spec.DuplicateGate
	if math.IsNaN(gate.Threshold) || math.IsInf(gate.Threshold, 0) || gate.Threshold <= 0 || gate.Threshold > 1 ||
		gate.MaxResults < 1 || gate.MinimumQueryTokens < 1 || gate.MinimumTokenLength < 1 ||
		gate.TitleWeight < 1 || gate.BodyWeight < 1 || len(gate.Stopwords) == 0 {
		return fmt.Errorf("issues lifecycle spec: duplicate_gate requires a threshold in (0, 1], positive limits/weights, and stopwords")
	}
	stopwords := make(map[string]bool)
	for _, word := range gate.Stopwords {
		if word == "" || stopwords[word] {
			return fmt.Errorf("issues lifecycle spec: duplicate_gate stopwords must be nonempty and unique")
		}
		stopwords[word] = true
	}
	return nil
}

// DuplicateGateSettings returns a copy of the duplicate-search policy from
// the embedded issue spec.
func DuplicateGateSettings() (DuplicateGatePolicy, error) {
	spec, err := currentLifecycleSpec()
	if err != nil {
		return DuplicateGatePolicy{}, err
	}
	gate := spec.DuplicateGate
	return DuplicateGatePolicy{
		Threshold:          gate.Threshold,
		MaxResults:         gate.MaxResults,
		MinimumQueryTokens: gate.MinimumQueryTokens,
		MinimumTokenLength: gate.MinimumTokenLength,
		TitleWeight:        gate.TitleWeight,
		BodyWeight:         gate.BodyWeight,
		Stopwords:          append([]string(nil), gate.Stopwords...),
	}, nil
}

// StatusFilterValues returns the accepted status and is filter names from the embedded lifecycle spec.
func StatusFilterValues() []string {
	spec, err := currentLifecycleSpec()
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var values []string
	for _, status := range spec.Lifecycle.Statuses {
		for _, filter := range status.Filters {
			if !seen[filter] {
				seen[filter] = true
				values = append(values, filter)
			}
		}
	}
	if len(spec.Lifecycle.Unresolved) > 0 && !seen["unresolved"] {
		values = append(values, "unresolved")
	}
	return values
}

// StatusLabel returns the display label for a lifecycle status key.
func StatusLabel(key string) (string, bool) {
	spec, err := currentLifecycleSpec()
	if err != nil {
		return "", false
	}
	for _, status := range spec.Lifecycle.Statuses {
		if status.Key == key {
			return status.Label, true
		}
	}
	return "", false
}

// StatusFilterMatches reports whether rawStatus matches an embedded status filter.
func StatusFilterMatches(rawStatus, filter string) bool {
	spec, err := currentLifecycleSpec()
	if err != nil {
		return false
	}
	if filter == "unresolved" {
		for _, key := range spec.Lifecycle.Unresolved {
			for _, status := range spec.Lifecycle.Statuses {
				if status.Key == key && statusRuleMatches(status, rawStatus) {
					return true
				}
			}
		}
		return false
	}
	for _, status := range spec.Lifecycle.Statuses {
		if !containsString(status.Filters, filter) {
			continue
		}
		if statusRuleMatches(status, rawStatus) {
			return true
		}
	}
	return false
}

func statusRuleMatches(rule statusRule, raw string) bool {
	lead := LeadingLifecycle(raw)
	full := normalizeStatusText(raw)
	for _, term := range rule.MatchTerms {
		switch rule.MatchMode {
		case "lead_contains":
			if strings.Contains(lead, term) {
				return true
			}
		case "lead_exact":
			if lead == term {
				return true
			}
		case "lead_exact_dash":
			if full == term || strings.HasPrefix(full, term+" — ") {
				return true
			}
		case "lead_token":
			fields := strings.Fields(full)
			if len(fields) > 0 && fields[0] == term {
				return true
			}
		}
	}
	return false
}

func statusFallbackMatches(rule statusRule, raw string) bool {
	full := normalizeStatusText(raw)
	for _, term := range rule.FallbackTerms {
		if strings.Contains(full, term) {
			return true
		}
	}
	return false
}

func statusRuleHasNearMiss(rule statusRule, raw string) bool {
	if rule.MatchMode != "lead_token" {
		return false
	}
	fields := strings.Fields(normalizeStatusText(raw))
	if len(fields) == 0 {
		return false
	}
	for _, term := range rule.MatchTerms {
		if strings.HasPrefix(fields[0], term) && fields[0] != term {
			return true
		}
	}
	return false
}

func normalizeStatusText(status string) string {
	s := strings.ToLower(status)
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "_", "")
	return strings.TrimSpace(s)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
