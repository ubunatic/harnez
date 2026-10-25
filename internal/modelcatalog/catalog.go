package modelcatalog

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
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
	Key       string
	Aliases   []string
	Preferred bool
	Effort    *bool
	Facts     map[string]any
	Guide     LegacyGuide
	Access    []AccessObservation
	Policy    *PolicyEntry
	Overrides map[string]any
	Enabled   bool
	Source    string
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
	Effort      *bool    `yaml:"effort"`
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
	for key, legacy := range spec.Models {
		// Legacy entries name a harness and exact backend ID, but contain no
		// upstream-provider or release-version metadata. Keep that limitation
		// explicit: use the harness for provider and the backend ID for version.
		identity := Identity{Harness: legacy.Provider, Provider: legacy.Provider, Model: legacy.Name, Version: legacy.Name, Tier: legacy.Tier}
		if identity.Tier == "" {
			identity.Tier = "low"
		}
		id := modelID(identity)
		alias := strings.TrimPrefix(key, legacy.Provider+":")
		catalog.Models[id] = Model{Identity: identity, Key: alias, Aliases: append([]string(nil), legacy.Aliases...), Preferred: legacy.Preferred, Effort: legacy.Effort, Facts: map[string]any{}, Guide: legacy.LegacyGuide, Enabled: true, Source: "spec-default"}
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
			current = Model{Identity: entry.Identity, Facts: map[string]any{}, Enabled: true, Source: "state"}
		} else {
			current.Source = "state"
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
			model = Model{Identity: observation.Identity, Facts: map[string]any{}, Enabled: true, Source: "state"}
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
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("model catalog: read %s: %w", filename, err)
	}
	var policy Policy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return fmt.Errorf("model catalog: parse %s: %w", filename, err)
	}
	if err := c.validatePolicyEnablement(policy); err != nil {
		return fmt.Errorf("model catalog: %s: %w", filename, err)
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
			model = Model{Identity: identity, Facts: map[string]any{}, Enabled: true, Source: "user-declared"}
		}
		if entry.VersionPin != "" && entry.VersionPin != model.Identity.Version {
			return fmt.Errorf("model catalog: %s: models.%s.version_pin does not match catalog identity.version %q", filename, policyID, model.Identity.Version)
		}
		model.Policy = &entry
		if model.Key == "" && entry.Kind == "custom" {
			model.Key = strings.TrimPrefix(policyID, strings.SplitN(policyID, ":", 2)[0]+":")
		}
		model.Source = "user-declared"
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

func (c *Catalog) validatePolicyEnablement(policy Policy) error {
	keys := make([]string, 0, len(policy.Models))
	for key := range policy.Models {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, leftKey := range keys {
		left := policy.Models[leftKey]
		if left.Enabled == nil {
			continue
		}
		leftIdentity := findPolicyIdentity(*c, leftKey)
		if left.Identity != nil {
			leftIdentity = *left.Identity
		}
		for _, rightKey := range keys[i+1:] {
			right := policy.Models[rightKey]
			if right.Enabled == nil || *left.Enabled == *right.Enabled {
				continue
			}
			rightIdentity := findPolicyIdentity(*c, rightKey)
			if right.Identity != nil {
				rightIdentity = *right.Identity
			}
			if modelID(leftIdentity) != modelID(rightIdentity) || modelID(leftIdentity) == modelID(Identity{}) {
				continue
			}
			leftTier, rightTier := policyIDTier(leftKey), policyIDTier(rightKey)
			if leftTier == "" || rightTier == "" || leftTier == rightTier {
				return fmt.Errorf("conflicting enabled values in models.%s and models.%s", leftKey, rightKey)
			}
		}
	}
	return nil
}

func policyIDTier(policyID string) string {
	parts := strings.Split(policyID, ":")
	if len(parts) == 3 && (parts[2] == "low" || parts[2] == "med" || parts[2] == "high") {
		return parts[2]
	}
	return ""
}

func findPolicyIdentity(c Catalog, policyID string) Identity {
	var found Identity
	for _, model := range c.Models {
		aliases := append(append([]string(nil), model.Aliases...), model.Key, model.Identity.Model)
		matches := model.Identity.Harness+":"+model.Identity.Model == policyID || model.Identity.Provider+":"+model.Identity.Model == policyID || policyKey(model) == policyID ||
			policyIDMatches(model.Identity.Harness, aliases, "low", policyID) || policyIDMatches(model.Identity.Provider, aliases, "low", policyID) ||
			policyIDMatches(model.Identity.Harness, aliases, "med", policyID) || policyIDMatches(model.Identity.Provider, aliases, "med", policyID) ||
			policyIDMatches(model.Identity.Harness, aliases, "high", policyID) || policyIDMatches(model.Identity.Provider, aliases, "high", policyID)
		if !matches {
			continue
		}
		if found != (Identity{}) && modelID(found) != modelID(model.Identity) {
			return Identity{}
		}
		found = model.Identity
	}
	return found
}

func policyKey(model Model) string {
	alias := model.Identity.Model
	if len(model.Aliases) > 0 {
		alias = model.Aliases[0]
		if i := strings.IndexByte(alias, ':'); i >= 0 {
			alias = alias[:i]
		}
	}
	provider := model.Identity.Harness
	if provider == "" {
		provider = model.Identity.Provider
	}
	return provider + ":" + alias + ":" + model.Identity.Tier
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// IsEnabled applies user policy to a fully resolved provider, model ID and
// tier. Unknown access observations do not disable a model.
func (c *Catalog) IsEnabled(provider, modelName, tier string) bool {
	if c == nil || c.Policy == nil {
		return true
	}
	provider, modelName, tier = strings.ToLower(provider), strings.ToLower(modelName), strings.ToLower(tier)
	var aliases []string
	providers := []string{provider}
	known := false
	for _, model := range c.Models {
		if !strings.EqualFold(model.Identity.Harness, provider) && !strings.EqualFold(model.Identity.Provider, provider) {
			continue
		}
		matches := strings.ToLower(model.Identity.Model) == modelName || strings.ToLower(model.Key) == modelName
		for _, alias := range model.Aliases {
			base := strings.ToLower(alias)
			if index := strings.IndexByte(base, ':'); index >= 0 {
				base = base[:index]
			}
			if strings.ToLower(alias) == modelName || base == modelName {
				matches = true
			}
		}
		if !matches {
			continue
		}
		known = true
		providers = append(providers, strings.ToLower(model.Identity.Harness), strings.ToLower(model.Identity.Provider))
		aliases = append(aliases, strings.ToLower(model.Key), strings.ToLower(model.Identity.Model))
		for _, alias := range model.Aliases {
			aliases = append(aliases, strings.ToLower(alias))
		}
	}
	for policyID, entry := range c.Policy.Models {
		identityMatches := entry.Identity != nil && (strings.EqualFold(entry.Identity.Harness, provider) || strings.EqualFold(entry.Identity.Provider, provider)) && strings.EqualFold(entry.Identity.Model, modelName) && (entry.Identity.Tier == "" || strings.EqualFold(entry.Identity.Tier, tier))
		idMatches := false
		for _, prefix := range providers {
			if policyIDMatches(prefix, aliases, tier, policyID) {
				idMatches = true
				break
			}
		}
		if !identityMatches && !idMatches {
			continue
		}
		if entry.Enabled != nil {
			return *entry.Enabled
		}
	}
	for _, alias := range aliases {
		base := alias
		if index := strings.IndexByte(base, ':'); index >= 0 {
			base = base[:index]
		}
		for _, prefix := range providers {
			if containsFold(c.Policy.Enabled, prefix+":"+base+":"+tier) || containsFold(c.Policy.Enabled, prefix+":"+base) {
				return true
			}
		}
	}
	for policyID, entry := range c.Policy.Models {
		if entry.Identity == nil || (!strings.EqualFold(entry.Identity.Harness, provider) && !strings.EqualFold(entry.Identity.Provider, provider)) || !strings.EqualFold(entry.Identity.Model, modelName) || (entry.Identity.Tier != "" && !strings.EqualFold(entry.Identity.Tier, tier)) {
			continue
		}
		if containsFold(c.Policy.Enabled, strings.ToLower(policyID)+":"+tier) || containsFold(c.Policy.Enabled, strings.ToLower(policyID)) {
			return true
		}
	}
	if !known {
		if containsFold(c.Policy.Enabled, provider+":"+modelName+":"+tier) || containsFold(c.Policy.Enabled, provider+":"+modelName) {
			return true
		}
	}
	return c.Policy.DefaultEnabled
}

func policyIDMatches(provider string, aliases []string, tier, policyID string) bool {
	parts := strings.Split(strings.ToLower(policyID), ":")
	if len(parts) < 2 || parts[0] != provider || (len(parts) == 3 && parts[2] != tier) {
		return false
	}
	for _, alias := range aliases {
		base := alias
		if index := strings.IndexByte(base, ':'); index >= 0 {
			base = base[:index]
		}
		if parts[1] == base {
			return true
		}
	}
	return false
}

func containsFold(values []string, value string) bool {
	for _, candidate := range values {
		if strings.EqualFold(candidate, value) {
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
