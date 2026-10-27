package glossary

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

const (
	BaseSpecPath     = "spec/glossary.yaml"
	SchemaPath       = "spec/schemas/glossary.schema.json"
	RepoOverrideRel  = ".harnez/glossary.yaml"
	StatusPreferred  = "preferred"
	StatusDeprecated = "deprecated"
)

// Scope defines path glob inclusion and exclusion lists.
type Scope struct {
	Include []string `yaml:"include,omitempty" json:"include,omitempty"`
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`
}

// ForbiddenTerm defines a disallowed phrase and its recommended replacement.
type ForbiddenTerm struct {
	Phrase        string `yaml:"phrase" json:"phrase"`
	Replacement   string `yaml:"replacement" json:"replacement"`
	Reason        string `yaml:"reason,omitempty" json:"reason,omitempty"`
	CaseSensitive bool   `yaml:"case_sensitive,omitempty" json:"case_sensitive,omitempty"`
}

// Term defines a canonical ubiquitous-language concept.
type Term struct {
	Term       string          `yaml:"term" json:"term"`
	Status     string          `yaml:"status,omitempty" json:"status,omitempty"`
	Enabled    *bool           `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Definition string          `yaml:"definition" json:"definition"`
	Synonyms   []string        `yaml:"synonyms,omitempty" json:"synonyms,omitempty"`
	CodeNames  []string        `yaml:"code_names,omitempty" json:"code_names,omitempty"`
	Forbidden  []ForbiddenTerm `yaml:"forbidden,omitempty" json:"forbidden,omitempty"`
	Scope      *Scope          `yaml:"scope,omitempty" json:"scope,omitempty"`
	Examples   []string        `yaml:"examples,omitempty" json:"examples,omitempty"`
	SeeAlso    []string        `yaml:"see_also,omitempty" json:"see_also,omitempty"`
}

// IsEnabled reports whether the term is active (defaults to true when omitted).
func (t Term) IsEnabled() bool {
	return t.Enabled == nil || *t.Enabled
}

// Glossary holds the parsed and merged glossary specification.
type Glossary struct {
	Version      int      `yaml:"version" json:"version"`
	IncludeBase  *bool    `yaml:"include_base,omitempty" json:"include_base,omitempty"`
	DefaultScope Scope    `yaml:"default_scope,omitempty" json:"default_scope,omitempty"`
	Terms        []Term   `yaml:"terms" json:"terms"`
	Sources      []string `yaml:"-" json:"sources,omitempty"`
}

// IncludesBase reports whether the base glossary should be included (defaults to true).
func (g *Glossary) IncludesBase() bool {
	return g == nil || g.IncludeBase == nil || *g.IncludeBase
}

// Validate validates raw YAML bytes against spec/schemas/glossary.schema.json.
func Validate(data []byte) error {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse glossary yaml: %w", err)
	}
	if doc == nil {
		return errors.New("validate glossary: empty document")
	}
	schemaData, err := fs.ReadFile(harnez.DefaultFS, SchemaPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", SchemaPath, err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return fmt.Errorf("parse %s: %w", SchemaPath, err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", SchemaPath, err)
	}
	if err := resolved.Validate(doc); err != nil {
		return fmt.Errorf("validate glossary: %w", err)
	}
	return nil
}

func parseAndValidate(data []byte, source string) (*Glossary, error) {
	if err := Validate(data); err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	var g Glossary
	if err := yaml.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("decode %s: %w", source, err)
	}
	for i := range g.Terms {
		if g.Terms[i].Status == "" {
			g.Terms[i].Status = StatusPreferred
		}
	}
	g.Sources = []string{source}
	return &g, nil
}

// Load loads and validates the embedded base glossary from spec/glossary.yaml.
func Load() (*Glossary, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, BaseSpecPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", BaseSpecPath, err)
	}
	return parseAndValidate(data, BaseSpecPath)
}

// LoadForRepo loads the base glossary and merges <repoDir>/.harnez/glossary.yaml if present.
func LoadForRepo(repoDir string) (*Glossary, error) {
	base, err := Load()
	if err != nil {
		return nil, err
	}
	overridePath := filepath.Join(repoDir, RepoOverrideRel)
	data, err := os.ReadFile(overridePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return base, nil
		}
		return nil, fmt.Errorf("read %s: %w", overridePath, err)
	}
	override, err := parseAndValidate(data, RepoOverrideRel)
	if err != nil {
		return nil, err
	}
	return Merge(base, override), nil
}

func normalizeTermKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// Merge combines base and override glossaries according to include_base, term key matching,
// and enabled filtering. Disabled terms (enabled: false) are omitted from the returned Terms slice.
func Merge(base, override *Glossary) *Glossary {
	if override == nil {
		return base
	}
	if base == nil || !override.IncludesBase() {
		out := &Glossary{
			Version:      override.Version,
			IncludeBase:  override.IncludeBase,
			DefaultScope: override.DefaultScope,
			Sources:      append([]string(nil), override.Sources...),
		}
		for _, t := range override.Terms {
			if t.Status == "" {
				t.Status = StatusPreferred
			}
			if t.IsEnabled() {
				out.Terms = append(out.Terms, t)
			}
		}
		return out
	}

	out := &Glossary{
		Version:      override.Version,
		IncludeBase:  override.IncludeBase,
		DefaultScope: base.DefaultScope,
	}
	if len(override.DefaultScope.Include) > 0 || len(override.DefaultScope.Exclude) > 0 {
		out.DefaultScope = override.DefaultScope
	}
	out.Sources = append(out.Sources, base.Sources...)
	out.Sources = append(out.Sources, override.Sources...)

	overrideByKey := make(map[string]Term, len(override.Terms))
	for _, t := range override.Terms {
		if t.Status == "" {
			t.Status = StatusPreferred
		}
		overrideByKey[normalizeTermKey(t.Term)] = t
	}

	matchedKeys := make(map[string]bool, len(override.Terms))
	for _, bt := range base.Terms {
		key := normalizeTermKey(bt.Term)
		if ot, ok := overrideByKey[key]; ok {
			matchedKeys[key] = true
			if ot.IsEnabled() {
				out.Terms = append(out.Terms, ot)
			}
			continue
		}
		if bt.IsEnabled() {
			out.Terms = append(out.Terms, bt)
		}
	}

	for _, ot := range override.Terms {
		key := normalizeTermKey(ot.Term)
		if matchedKeys[key] {
			continue
		}
		if ot.Status == "" {
			ot.Status = StatusPreferred
		}
		if ot.IsEnabled() {
			out.Terms = append(out.Terms, ot)
		}
	}

	return out
}
