package modelcatalog

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

const (
	stateCatalogName = "catalog.yaml"
	stateAccessName  = "access.yaml"
	policyName       = "models.yaml"
)

// Catalog is the merged immutable view of embedded defaults, machine state,
// and the user's policy. Models is keyed by exact model identity.
type Catalog struct {
	Models         map[string]Model
	Access         []AccessObservation
	AccessComplete bool
	Policy         *Policy
}

// Model contains public catalog facts and optional legacy listing guidance.
type Model struct {
	Identity  Identity
	Aliases   []string
	Facts     map[string]any
	Guide     LegacyGuide
	Access    []AccessObservation
	Policy    *PolicyEntry
	Overrides map[string]any
	Enabled   bool
}

// Identity identifies one exact model variant.
type Identity struct {
	Harness  string `yaml:"harness" json:"harness"`
	Provider string `yaml:"provider" json:"provider"`
	Model    string `yaml:"model" json:"model"`
	Version  string `yaml:"version" json:"version"`
	Tier     string `yaml:"tier,omitempty" json:"tier,omitempty"`
}

// LegacyGuide preserves spec/agent.yaml listing fields until consumers migrate.
type LegacyGuide struct {
	Cost       int            `yaml:"cost"`
	CostByTier map[string]int `yaml:"cost_by_tier"`
	Eff        string         `yaml:"eff"`
	Skills     string         `yaml:"skills"`
	Roles      string         `yaml:"roles"`
	Use        string         `yaml:"use"`
	UseMed     string         `yaml:"use_med"`
}

type legacyModel struct {
	Aliases     []string `yaml:"aliases"`
	Preferred   bool     `yaml:"preferred"`
	Provider    string   `yaml:"provider"`
	Name        string   `yaml:"name"`
	Tier        string   `yaml:"tier"`
	LegacyGuide `yaml:",inline"`
}

type modelFile struct {
	Version int `yaml:"version"`
	Models  []struct {
		Identity Identity       `yaml:"identity"`
		Aliases  []string       `yaml:"aliases"`
		Facts    map[string]any `yaml:"facts"`
	} `yaml:"models"`
}

type accessFile struct {
	Version      int                 `yaml:"version"`
	Complete     bool                `yaml:"complete"`
	Observations []AccessObservation `yaml:"observations"`
}

// AccessObservation records one machine's access evidence without treating
// unknown or stale observations as denials.
type AccessObservation struct {
	Identity Identity `yaml:"identity" json:"identity"`
	Status   string   `yaml:"status" json:"status"`
	Method   string   `yaml:"method" json:"method"`
	Account  *string  `yaml:"account,omitempty" json:"account,omitempty"`
	Facts    []any    `yaml:"facts,omitempty" json:"facts,omitempty"`
}

// Policy is the validated user policy document.
type Policy struct {
	DefaultEnabled bool                   `yaml:"default_enabled"`
	Enabled        []string               `yaml:"enabled"`
	Models         map[string]PolicyEntry `yaml:"models"`
}

// PolicyEntry describes a custom model or a field-level patch.
type PolicyEntry struct {
	Kind       string         `yaml:"kind"`
	Identity   *Identity      `yaml:"identity,omitempty"`
	VersionPin string         `yaml:"version_pin,omitempty"`
	Enabled    *bool          `yaml:"enabled,omitempty"`
	Fields     map[string]any `yaml:"fields,omitempty"`
}

// Load reads the embedded defaults and overlays optional state and policy
// files from the user's XDG directories.
func Load() (*Catalog, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("model catalog: resolve home: %w", err)
	}
	configBase := os.Getenv("XDG_CONFIG_HOME")
	if configBase == "" {
		configBase = filepath.Join(home, ".config")
	}
	stateBase := os.Getenv("XDG_STATE_HOME")
	if stateBase == "" {
		stateBase = filepath.Join(home, ".local", "state")
	}
	return LoadFromDirs(filepath.Join(stateBase, "harnez", "models"), filepath.Join(configBase, "harnez"))
}

// LoadFromDirs loads the catalog from explicit machine directories. It is
// exported to let callers and tests isolate user state from the host account.
func LoadFromDirs(stateDir, configDir string) (*Catalog, error) {
	catalog, err := loadDefaults()
	if err != nil {
		return nil, err
	}
	if err := catalog.mergeMachineFile(filepath.Join(stateDir, stateCatalogName)); err != nil {
		return nil, err
	}
	if err := catalog.mergeAccessFile(filepath.Join(stateDir, stateAccessName)); err != nil {
		return nil, err
	}
	if err := catalog.mergePolicyFile(filepath.Join(configDir, policyName)); err != nil {
		return nil, err
	}
	return catalog, nil
}

func loadDefaults() (*Catalog, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/agent.yaml")
	if err != nil {
		return nil, fmt.Errorf("model catalog: read embedded defaults: %w", err)
	}
	var spec struct {
		Models map[string]legacyModel `yaml:"models"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("model catalog: parse embedded defaults: %w", err)
	}
	catalog := &Catalog{Models: make(map[string]Model, len(spec.Models))}
	for _, legacy := range spec.Models {
		// Legacy entries name a harness and exact backend ID, but contain no
		// upstream-provider or release-version metadata. Keep that limitation
		// explicit: use the harness for provider and the backend ID for version.
		identity := Identity{Harness: legacy.Provider, Provider: legacy.Provider, Model: legacy.Name, Version: legacy.Name, Tier: legacy.Tier}
		if identity.Tier == "" {
			identity.Tier = "low"
		}
		id := modelID(identity)
		catalog.Models[id] = Model{Identity: identity, Aliases: append([]string(nil), legacy.Aliases...), Facts: map[string]any{}, Guide: legacy.LegacyGuide, Enabled: true}
	}
	return catalog, nil
}

func (c *Catalog) mergeMachineFile(filename string) error {
	_, err := readValidated(filename, "models.schema.json")
	if err != nil {
		if errorsIsNotExist(err) {
			return nil
		}
		return fmt.Errorf("model catalog: %w", err)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("model catalog: read %s: %w", filename, err)
	}
	var file modelFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("model catalog: parse %s: %w", filename, err)
	}
	for _, entry := range file.Models {
		id := modelID(entry.Identity)
		current, ok := c.Models[id]
		if !ok {
			current = Model{Identity: entry.Identity, Facts: map[string]any{}, Enabled: true}
		}
		current.Aliases = appendUnique(current.Aliases, entry.Aliases...)
		mergeFactMaps(current.Facts, entry.Facts)
		c.Models[id] = current
	}
	return nil
}

func (c *Catalog) mergeAccessFile(filename string) error {
	_, err := readValidated(filename, "model-access.schema.json")
	if err != nil {
		if errorsIsNotExist(err) {
			return nil
		}
		return fmt.Errorf("model catalog: %w", err)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("model catalog: read %s: %w", filename, err)
	}
	var file accessFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("model catalog: parse %s: %w", filename, err)
	}
	c.AccessComplete = file.Complete
	c.Access = append(c.Access, file.Observations...)
	for _, observation := range file.Observations {
		id := modelID(observation.Identity)
		model, ok := c.Models[id]
		if !ok {
			model = Model{Identity: observation.Identity, Facts: map[string]any{}, Enabled: true}
		}
		model.Access = append(model.Access, observation)
		c.Models[id] = model
	}
	return nil
}

func (c *Catalog) mergePolicyFile(filename string) error {
	doc, err := readValidated(filename, "model-policy.schema.json")
	if err != nil {
		if errorsIsNotExist(err) {
			return nil
		}
		return fmt.Errorf("model catalog: %w", err)
	}
	if err := ValidatePolicyPins(doc); err != nil {
		return fmt.Errorf("model catalog: %s: %w", filename, err)
	}
	data, _ := os.ReadFile(filename)
	var policy Policy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return fmt.Errorf("model catalog: parse %s: %w", filename, err)
	}
	c.Policy = &policy
	for policyID, entry := range policy.Models {
		var identity Identity
		if entry.Identity != nil {
			identity = *entry.Identity
		} else {
			identity = findPolicyIdentity(*c, policyID)
		}
		modelID := modelID(identity)
		model, found := c.Models[modelID]
		if !found {
			if entry.Kind != "custom" || entry.Identity == nil {
				return fmt.Errorf("model catalog: %s: patch %s does not identify an existing model", filename, policyID)
			}
			model = Model{Identity: identity, Facts: map[string]any{}, Enabled: true}
		}
		if entry.VersionPin != "" && entry.VersionPin != model.Identity.Version {
			return fmt.Errorf("model catalog: %s: models.%s.version_pin does not match catalog identity.version %q", filename, policyID, model.Identity.Version)
		}
		model.Policy = &entry
		if model.Overrides == nil {
			model.Overrides = map[string]any{}
		}
		for field, value := range entry.Fields {
			model.Overrides[field] = value
		}
		model.Enabled = entry.Enabled != nil && *entry.Enabled
		if entry.Enabled == nil {
			model.Enabled = contains(policy.Enabled, policyID) || (policy.DefaultEnabled && !contains(policy.Enabled, policyID))
		}
		c.Models[modelID] = model
	}
	for id, model := range c.Models {
		if model.Policy != nil && model.Policy.Enabled != nil {
			continue
		}
		policyID := policyKey(model)
		model.Enabled = contains(policy.Enabled, policyID) || policy.DefaultEnabled
		c.Models[id] = model
	}
	return nil
}

func findPolicyIdentity(c Catalog, policyID string) Identity {
	for _, model := range c.Models {
		if model.Identity.Provider+":"+model.Identity.Model == policyID || policyKey(model) == policyID {
			return model.Identity
		}
		for _, alias := range model.Aliases {
			if model.Identity.Provider+":"+alias == policyID || model.Identity.Provider+":"+alias+":"+model.Identity.Tier == policyID {
				return model.Identity
			}
		}
	}
	return Identity{}
}

func policyKey(model Model) string {
	alias := model.Identity.Model
	if len(model.Aliases) > 0 {
		alias = model.Aliases[0]
		if i := strings.IndexByte(alias, ':'); i >= 0 {
			alias = alias[:i]
		}
	}
	return model.Identity.Provider + ":" + alias + ":" + model.Identity.Tier
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func modelID(identity Identity) string {
	return strings.Join([]string{identity.Harness, identity.Provider, identity.Model, identity.Version, identity.Tier}, ":")
}

func appendUnique(values []string, more ...string) []string {
	seen := make(map[string]bool, len(values)+len(more))
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range more {
		if !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}

func mergeFactMaps(dst, src map[string]any) {
	for key, value := range src {
		if previous, exists := dst[key]; exists {
			before, _ := previous.([]any)
			after, _ := value.([]any)
			dst[key] = append(append([]any(nil), before...), after...)
		} else {
			dst[key] = value
		}
	}
}

func readValidated(filename, schemaName string) (any, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var document any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}
	if err := validateDocument(schemaName, document); err != nil {
		return nil, fmt.Errorf("validate %s: %w", filename, err)
	}
	return document, nil
}

func validateDocument(schemaName string, document any) error {
	load := func(name string) (*jsonschema.Schema, error) {
		data, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/"+name)
		if err != nil {
			return nil, err
		}
		var schema jsonschema.Schema
		if err := json.Unmarshal(data, &schema); err != nil {
			return nil, err
		}
		return &schema, nil
	}
	root, err := load(schemaName)
	if err != nil {
		return err
	}
	resolved, err := root.Resolve(&jsonschema.ResolveOptions{Loader: func(uri *url.URL) (*jsonschema.Schema, error) {
		name := path.Base(uri.Path)
		if strings.Contains(name, "..") {
			return nil, fs.ErrNotExist
		}
		return load(name)
	}})
	if err != nil {
		return err
	}
	return resolved.Validate(document)
}

func errorsIsNotExist(err error) bool { return os.IsNotExist(err) }
