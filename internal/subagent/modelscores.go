package subagent

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// ModelScoreScale defines the rating scale boundaries and units.
type ModelScoreScale struct {
	Min  float64 `yaml:"min" json:"min"`
	Max  float64 `yaml:"max" json:"max"`
	Step float64 `yaml:"step" json:"step"`
	Unit string  `yaml:"unit" json:"unit"`
}

// ModelScoreEntry holds the ratings and source metadata for a single model.
type ModelScoreEntry struct {
	Source  string             `yaml:"source" json:"source"`
	Ratings map[string]float64 `yaml:"ratings" json:"ratings"`
}

// ModelScoresSpec is the structured representation of spec/model_scores.yaml.
type ModelScoresSpec struct {
	Version                 int                        `yaml:"version" json:"version"`
	Scale                   ModelScoreScale            `yaml:"scale" json:"scale"`
	SubscriptionMultipliers map[string]float64         `yaml:"subscription_multipliers" json:"subscription_multipliers"`
	MultiplierSource        string                     `yaml:"multiplier_source" json:"multiplier_source"`
	ProviderColors          map[string]string          `yaml:"provider_colors" json:"provider_colors"`
	Models                  map[string]ModelScoreEntry `yaml:"models" json:"models"`
}

var (
	modelScoresOnce sync.Once
	modelScoresSpec *ModelScoresSpec
	modelScoresErr  error
)

// LoadModelScores loads and validates spec/model_scores.yaml.
func LoadModelScores() (*ModelScoresSpec, error) {
	modelScoresOnce.Do(func() {
		spec, err := loadModelScores()
		if err != nil {
			modelScoresErr = err
			return
		}
		modelScoresSpec = spec
	})
	return modelScoresSpec, modelScoresErr
}

func loadModelScores() (*ModelScoresSpec, error) {
	schemaData, err := harnez.DefaultFS.ReadFile("spec/schemas/model_scores.schema.json")
	if err != nil {
		return nil, fmt.Errorf("read model_scores schema: %w", err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return nil, fmt.Errorf("unmarshal model_scores schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("resolve model_scores schema: %w", err)
	}

	data, err := harnez.DefaultFS.ReadFile("spec/model_scores.yaml")
	if err != nil {
		return nil, fmt.Errorf("read spec/model_scores.yaml: %w", err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("yaml unmarshal spec/model_scores.yaml: %w", err)
	}

	if err := resolved.Validate(raw); err != nil {
		return nil, fmt.Errorf("spec/model_scores.yaml violates schema: %w", err)
	}

	var spec ModelScoresSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("unmarshal spec struct: %w", err)
	}
	return &spec, nil
}

// Multiplier returns the subscription multiplier for a provider.
func (s *ModelScoresSpec) Multiplier(provider string) float64 {
	if m, ok := s.SubscriptionMultipliers[provider]; ok {
		return m
	}
	if def, ok := s.SubscriptionMultipliers["default"]; ok {
		return def
	}
	return 1.0
}

// ProviderColor returns the ANSI SGR color code for a provider.
func (s *ModelScoresSpec) ProviderColor(provider string) string {
	if c, ok := s.ProviderColors[provider]; ok {
		return c
	}
	if def, ok := s.ProviderColors["default"]; ok {
		return def
	}
	return "37"
}

// Rating looks up the capability rating for a model and effort tier.
func (s *ModelScoresSpec) Rating(provider, name, tier string) (float64, string, bool) {
	key := provider + ":" + name
	entry, ok := s.Models[key]
	if !ok {
		return 0, "", false
	}
	rating, hasRating := entry.Ratings[tier]
	if !hasRating {
		// Fallback to low tier if tier not found
		rating, hasRating = entry.Ratings["low"]
	}
	if !hasRating {
		return 0, "", false
	}
	return rating, entry.Source, true
}
