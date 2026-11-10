package subagent

import (
	"encoding/json"
	"fmt"
	"strings"
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
	CostTicks               []float64                  `yaml:"cost_ticks" json:"cost_ticks"`
	FrontierStyle           string                     `yaml:"frontier_style" json:"frontier_style"`
	SubscriptionMultipliers map[string]float64         `yaml:"subscription_multipliers" json:"subscription_multipliers"`
	MultiplierSource        string                     `yaml:"multiplier_source" json:"multiplier_source"`
	MultiplierSources       map[string]string          `yaml:"multiplier_sources,omitempty" json:"multiplier_sources,omitempty"`
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

	// Validate every model provider has an explicit subscription multiplier
	for key := range spec.Models {
		parts := strings.Split(key, ":")
		if len(parts) > 0 {
			prov := parts[0]
			if _, ok := spec.SubscriptionMultipliers[prov]; !ok {
				return nil, fmt.Errorf("provider %q for model %q missing from subscription_multipliers in spec/model_scores.yaml", prov, key)
			}
		}
	}

	return &spec, nil
}

// Multiplier returns the subscription multiplier for a provider.
func (s *ModelScoresSpec) Multiplier(provider string) (float64, error) {
	if m, ok := s.SubscriptionMultipliers[provider]; ok {
		return m, nil
	}
	return 0, fmt.Errorf("missing subscription multiplier for provider %q in spec/model_scores.yaml", provider)
}

// MultiplierSourceFor returns the recorded source for a provider's subscription multiplier.
func (s *ModelScoresSpec) MultiplierSourceFor(provider string) string {
	if s.MultiplierSources != nil {
		if src, ok := s.MultiplierSources[provider]; ok && src != "" {
			return src
		}
		if def, ok := s.MultiplierSources["default"]; ok && def != "" {
			return def
		}
	}
	if s.MultiplierSource != "" {
		return s.MultiplierSource
	}
	return "owner estimate, unverified"
}

type colorsSpec struct {
	Colors map[string]struct {
		Title string `yaml:"title"`
		SGR   string `yaml:"sgr"`
	} `yaml:"colors"`
}

var (
	colorsSpecOnce sync.Once
	colorsSpecData *colorsSpec
	colorsSpecErr  error
)

func getColorsSpec() (*colorsSpec, error) {
	colorsSpecOnce.Do(func() {
		data, err := harnez.DefaultFS.ReadFile("spec/colors.yaml")
		if err != nil {
			colorsSpecErr = fmt.Errorf("read spec/colors.yaml: %w", err)
			return
		}
		var cs colorsSpec
		if err := yaml.Unmarshal(data, &cs); err != nil {
			colorsSpecErr = fmt.Errorf("parse spec/colors.yaml: %w", err)
			return
		}
		colorsSpecData = &cs
	})
	return colorsSpecData, colorsSpecErr
}

// ProviderColor returns the ANSI SGR color code for a provider from spec/colors.yaml.
func (s *ModelScoresSpec) ProviderColor(provider string) (string, error) {
	cs, err := getColorsSpec()
	if err != nil {
		return "", err
	}
	key := "provider-" + provider
	if c, ok := cs.Colors[key]; ok && c.SGR != "" {
		return c.SGR, nil
	}
	if def, ok := cs.Colors["provider-default"]; ok && def.SGR != "" {
		return def.SGR, nil
	}
	return "", fmt.Errorf("missing color %q in spec/colors.yaml", key)
}

// FrontierColor returns the ANSI SGR style for the Pareto frontier line from spec/colors.yaml.
func (s *ModelScoresSpec) FrontierColor() (string, error) {
	cs, err := getColorsSpec()
	if err != nil {
		return "", err
	}
	if c, ok := cs.Colors[s.FrontierStyle]; ok && c.SGR != "" {
		return c.SGR, nil
	}
	return "", fmt.Errorf("frontier style %q not found in spec/colors.yaml", s.FrontierStyle)
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
		rating, hasRating = entry.Ratings["low"]
	}
	if !hasRating {
		return 0, "", false
	}
	return rating, entry.Source, true
}
