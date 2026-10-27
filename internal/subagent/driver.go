package subagent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"ubunatic.com/harnez/internal/modelcatalog"
)

// Driver executes and manages a provider session.
type Driver interface {
	Run(context.Context, RunOptions) (*TurnResult, error)
	Resume(context.Context, string, string, Model) (*TurnResult, error)
	Compact(context.Context, string) (*TurnResult, error)
	Stop(context.Context, string) error
	Delete(context.Context, string) error
}

type ResumeChecker interface {
	CheckResumable(providerID string) (ok bool, reason string)
}

// DriverFor returns the batch driver for m's provider, pinned to dir.
// Providers without a driver get an UnsupportedDriver.
func DriverFor(m Model, dir string) Driver {
	switch m.Provider {
	case "claude":
		return ClaudeDriver{Dir: dir}
	case "codex":
		return CodexDriver{Dir: dir}
	case "agy":
		return AgyDriver{Dir: dir, ModelSpec: m.Spec()}
	case "pi":
		return PiDriver{Dir: dir}
	default:
		return UnsupportedDriver{Provider: m.Provider}
	}
}

// StreamDriverFor is DriverFor for interactive callers (harnez tui): claude
// and pi get drivers that also implement StreamingDriver. Batch callers keep
// DriverFor, whose claude and pi drivers do not stream.
func StreamDriverFor(m Model, dir string) Driver {
	switch d := DriverFor(m, dir).(type) {
	case ClaudeDriver:
		return ClaudeStreamDriver{d}
	case PiDriver:
		return PiStreamDriver{d}
	default:
		return d
	}
}

// UnsupportedDriver reports a provider capability error without misrouting it.
type UnsupportedDriver struct{ Provider string }

func (d UnsupportedDriver) unsupported() error {
	return fmt.Errorf("batch agent lifecycle is not supported for provider %q", d.Provider)
}
func (d UnsupportedDriver) Run(context.Context, RunOptions) (*TurnResult, error) {
	return nil, d.unsupported()
}
func (d UnsupportedDriver) Resume(context.Context, string, string, Model) (*TurnResult, error) {
	return nil, d.unsupported()
}
func (d UnsupportedDriver) Compact(context.Context, string) (*TurnResult, error) {
	return nil, d.unsupported()
}
func (d UnsupportedDriver) Stop(context.Context, string) error   { return d.unsupported() }
func (d UnsupportedDriver) Delete(context.Context, string) error { return d.unsupported() }

// RunOptions describes a new provider turn.
type RunOptions struct {
	Prompt string
	Model  Model
	Dir    string
}

// Model is a resolved provider model specification.
type Model struct {
	Provider string
	Name     string
	Tier     string
	// Effort is nil (supported, the default) or false for models that
	// reject an effort/reasoning flag (e.g. agy:sonnet, agy:opus).
	Effort *bool
}

type modelAlias struct {
	Model     Model    `yaml:",inline"`
	Aliases   []string `yaml:"aliases"`
	Preferred bool     `yaml:"preferred"`
}

// ModelGuide is the listing guidance of a spec/agent.yaml model entry; it
// stays out of Model so session records do not carry it.
type ModelGuide struct {
	Cost              int            `yaml:"cost"` // fallback relative cost when no tier override exists
	CostByTier        map[string]int `yaml:"cost_by_tier"`
	Eff               string         `yaml:"eff"`    // tokens per goal: + ~ - ?
	Skills            string         `yaml:"skills"` // e.g. "Go+ TUI~ SQL?"
	Roles             string         `yaml:"roles"`
	Use               string         `yaml:"use"`
	UseMed            string         `yaml:"use_med"` // replaces Use for the :med variant
	GuidanceSourceURL string         `yaml:"guidance_source_url"`
}

var (
	mergedModelCatalogOnce = sync.OnceValues(modelcatalog.Load)
	mergedModelCatalog     *modelcatalog.Catalog
	modelAliases           map[string]modelAlias
	modelGuides            map[string]ModelGuide
	modelAliasNames        map[string]string
	modelCatalogInit       = sync.OnceValue(loadMergedModelCatalog)
)

func loadMergedModelCatalog() error {
	catalog, err := mergedModelCatalogOnce()
	if err != nil {
		return err
	}
	catalogModels := catalog.Models()
	aliases := make(map[string]modelAlias, len(catalogModels))
	guides := make(map[string]ModelGuide, len(catalogModels))
	aliasNames := make(map[string]string, len(catalogModels))
	ids := make([]string, 0, len(catalogModels))
	for id := range catalogModels {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		entry := catalogModels[id]
		provider := entry.Identity.Harness
		if provider == "" {
			provider = entry.Identity.Provider
		}
		modelAliases := append([]string(nil), entry.Aliases...)
		if value, ok := entry.Overrides["aliases"]; ok {
			modelAliases = stringSlice(value)
		}
		name := entry.Key
		if name == "" {
			name = entry.Identity.Model
			if len(modelAliases) > 0 {
				name = modelAliases[0]
				if index := strings.IndexByte(name, ':'); index >= 0 {
					name = name[:index]
				}
			}
		}
		model := Model{Provider: provider, Name: entry.Identity.Model, Tier: entry.Identity.Tier, Effort: entry.Effort}
		if model.Tier == "" {
			model.Tier = "low"
		}
		key := provider + ":" + name
		aliases[key] = modelAlias{Model: model, Aliases: modelAliases, Preferred: entry.Preferred}
		guide := ModelGuide{Cost: entry.Guide.Cost, CostByTier: entry.Guide.CostByTier, Eff: entry.Guide.Eff, Skills: entry.Guide.Skills, Roles: entry.Guide.Roles, Use: entry.Guide.Use, UseMed: entry.Guide.UseMed, GuidanceSourceURL: entry.Guide.GuidanceSourceURL}
		if value, ok := entry.Overrides["purpose"]; ok {
			if value == nil {
				guide.Use = ""
			} else if purpose, isString := value.(string); isString {
				guide.Use = purpose
			}
		}
		if value, ok := entry.Overrides["tier"]; ok {
			if tier, isString := value.(string); isString {
				model.Tier = tier
				aliases[key] = modelAlias{Model: model, Aliases: modelAliases, Preferred: entry.Preferred}
			}
		}
		guides[key] = guide
		aliasNames[provider+"\x00"+model.Name] = name
	}
	mergedModelCatalog = catalog
	modelAliases = aliases
	modelGuides = guides
	modelAliasNames = aliasNames
	return nil
}

func stringSlice(value any) []string {
	switch values := value.(type) {
	case []string:
		return append([]string(nil), values...)
	case []any:
		out := make([]string, 0, len(values))
		for _, item := range values {
			if value, ok := item.(string); ok {
				out = append(out, value)
			}
		}
		return out
	default:
		return nil
	}
}

func ensureModelCatalog() error { return modelCatalogInit() }

func modelAliasMap() (map[string]modelAlias, error) {
	if err := ensureModelCatalog(); err != nil {
		return nil, err
	}
	aliases := make(map[string]modelAlias, len(modelAliases))
	for key, value := range modelAliases {
		aliases[key] = value
	}
	return aliases, nil
}

// CostForTier returns the tier override when present, falling back to Cost.
func (g ModelGuide) CostForTier(tier string) int {
	if cost, ok := g.CostByTier[tier]; ok {
		return cost
	}
	return g.Cost
}

// SupportsEffort reports whether an effort/reasoning-tier flag should be
// passed for this model. Unset (nil) means supported.
func (m Model) SupportsEffort() bool {
	return m.Effort == nil || *m.Effort
}

// TurnResult is the normalized result of a provider turn.
type TurnResult struct {
	SessionID string `json:"session_id,omitempty"`
	Response  string `json:"response"`
	// Messages holds every agent message of the turn in arrival order;
	// Response is the last one.
	Messages             []string `json:"messages,omitempty"`
	Warnings             []string `json:"warnings,omitempty"`
	InputTokens          int      `json:"input_tokens"`
	OutputTokens         int      `json:"output_tokens"`
	CachedTokens         int      `json:"cached_tokens"`
	ReasoningTokens      int      `json:"reasoning_tokens,omitempty"`
	ReasoningTokensKnown bool     `json:"reasoning_tokens_known,omitempty"`
	// ContextTokens is the provider's input-token count for the last turn,
	// including cached input. It measures the full context the model read.
	ContextTokens     int `json:"context_tokens"`
	ContextWindowSize int `json:"context_window_size,omitempty"`
	// CompactionObserved reports a new provider compaction record during this turn.
	CompactionObserved bool  `json:"compaction_observed,omitempty"`
	TokensTurn         int   `json:"tokens_turn"`
	TokensCumulative   int   `json:"tokens_cumulative"`
	DurationMS         int64 `json:"duration_ms"`
}

// KnownModels returns the configured shorthand specifications in stable order.
func KnownModels() []Model {
	models, err := KnownModelsE()
	if err != nil {
		panic(err)
	}
	return models
}

// KnownModelsE returns known models and any catalog loading error.
func KnownModelsE() ([]Model, error) {
	if err := ensureModelCatalog(); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(modelAliases))
	for key := range modelAliases {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	models := make([]Model, 0, len(keys))
	for _, key := range keys {
		candidate := modelAliases[key].Model
		if modelEnabledAtAnyTier(mergedModelCatalog, candidate) {
			models = append(models, candidate)
		}
	}
	return models, nil
}

// KnownModelSpecs lists every selectable spec: each configured model at its
// default tier, plus :med and :high variants for effort-aware models.
func KnownModelSpecs() []string {
	specs, err := KnownModelSpecsE()
	if err != nil {
		panic(err)
	}
	return specs
}

// KnownModelSpecsE lists enabled selectable model specs.
func KnownModelSpecsE() ([]string, error) {
	var specs []string
	entries, err := KnownModelEntriesE()
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		specs = append(specs, e.Spec)
	}
	return specs, nil
}

// ModelEntry is one selectable spec with its listing guidance.
type ModelEntry struct {
	Spec  string `json:"spec"`
	Model Model  `json:"model"`
	// Batch reports whether the selected provider driver supports batch turns.
	Batch bool `json:"batch"`
	// Effort reports whether the batch driver passes a tier flag.
	Effort bool   `json:"effort"`
	Cost   int    `json:"cost"`
	Eff    string `json:"eff"`
	Skills string `json:"skills"`
	Roles  string `json:"roles"`
	Use    string `json:"use"`
	Source string `json:"source"`
}

// ModelEntriesWithDriver marks models according to their selected batch driver.
func ModelEntriesWithDriver(entries []ModelEntry, driver func(Model) Driver) []ModelEntry {
	marked := make([]ModelEntry, len(entries))
	for i, entry := range entries {
		_, unsupported := driver(entry.Model).(UnsupportedDriver)
		entry.Batch = !unsupported
		marked[i] = entry
	}
	return marked
}

// KnownModelEntries backs KnownModelSpecs and `harnez agent models`.
func KnownModelEntries() []ModelEntry {
	entries, err := KnownModelEntriesE()
	if err != nil {
		panic(err)
	}
	return entries
}

// KnownModelEntriesE lists selectable models from the merged catalog.
func KnownModelEntriesE() ([]ModelEntry, error) {
	if err := ensureModelCatalog(); err != nil {
		return nil, err
	}
	var entries []ModelEntry
	models, err := KnownModelsE()
	if err != nil {
		return nil, err
	}
	for _, m := range models {
		guide := modelGuides[m.Provider+":"+modelAliasName(m)]
		effort := m.SupportsEffort()
		alias := modelAliasName(m)
		catalogModel, ok := findCatalogModel(mergedModelCatalog, m)
		if !ok {
			continue
		}
		tiers := []string{m.Tier}
		if effort {
			tiers = []string{"low", "med", "high"}
		}
		for _, tier := range tiers {
			if !mergedModelCatalog.IsEnabled(m.Provider, m.Name, tier) {
				continue
			}
			model := m
			model.Tier = tier
			use := guide.Use
			if tier == "med" && guide.UseMed != "" {
				use = guide.UseMed
			}
			entries = append(entries, ModelEntry{Spec: m.Provider + ":" + alias + ":" + tier, Model: model, Effort: effort, Cost: guide.CostForTier(tier), Eff: guide.Eff, Skills: guide.Skills, Roles: guide.Roles, Use: use, Source: catalogModel.Source})
		}
	}
	return entries, nil
}

func (m Model) Spec() string {
	return m.Provider + ":" + modelAliasName(m) + ":" + m.Tier
}

func modelAliasName(m Model) string {
	if err := ensureModelCatalog(); err != nil {
		panic(err)
	}
	if name := modelAliasNames[m.Provider+"\x00"+m.Name]; name != "" {
		return name
	}
	return m.Name
}

// ResolveModel expands provider:model[:tier] shorthand.
func ResolveModel(spec string) (Model, error) {
	if err := ensureModelCatalog(); err != nil {
		return Model{}, err
	}
	return resolveModelWithCatalog(modelAliases, mergedModelCatalog, spec)
}

func resolveModelWithCatalog(aliases map[string]modelAlias, catalog *modelcatalog.Catalog, spec string) (Model, error) {
	modelSpec := spec
	agent, err := agentSpecOnce()
	if err != nil {
		return Model{}, err
	}
	role := strings.ToLower(strings.SplitN(strings.TrimSpace(spec), ":", 2)[0])
	if _, ok := agent.PurposeRoles[role]; ok {
		modelSpec, err = resolvePurpose(agent.PurposeAliases, strings.ToLower(strings.TrimSpace(spec)), map[string]bool{}, 1)
		if err != nil {
			return Model{}, err
		}
	}
	model, err := resolveModelIn(aliases, modelSpec)
	if err != nil {
		return Model{}, err
	}
	if !catalog.IsEnabled(model.Provider, model.Name, model.Tier) {
		return Model{}, fmt.Errorf("model %q is disabled by user policy", spec)
	}
	return model, nil
}

func resolvePurpose(aliases map[string][]string, name string, visiting map[string]bool, depth int) (string, error) {
	if depth > 4 {
		return "", fmt.Errorf("purpose alias depth exceeds 4 at %q", name)
	}
	if visiting[name] {
		return "", fmt.Errorf("purpose alias cycle at %q", name)
	}
	targets, ok := aliases[name]
	if !ok {
		return "", fmt.Errorf("unknown purpose alias %q", name)
	}
	if len(targets) == 0 {
		return "", fmt.Errorf("purpose alias %q is unset", name)
	}
	visiting[name] = true
	defer delete(visiting, name)
	target := targets[0]
	if purposeName.MatchString(target) {
		return resolvePurpose(aliases, target, visiting, depth+1)
	}
	return target, nil
}

func resolveDefaultModelWithCatalog(aliases map[string]modelAlias, catalog *modelcatalog.Catalog, spec string) (Model, error) {
	model, err := resolveModelWithCatalog(aliases, catalog, spec)
	if err != nil && strings.Contains(err.Error(), "disabled by user policy") {
		return Model{}, fmt.Errorf("default model %q is disabled by user policy; pass --model <spec> to choose another model or enable it in models.yaml", spec)
	}
	return model, err
}

// ModelEnabled reports whether policy allows this resolved model variant.
func ModelEnabled(provider, modelName, tier string) (bool, error) {
	if err := ensureModelCatalog(); err != nil {
		return false, err
	}
	return mergedModelCatalog.IsEnabled(provider, modelName, tier), nil
}

func modelEnabledAtAnyTier(catalog *modelcatalog.Catalog, model Model) bool {
	if model.SupportsEffort() {
		for _, tier := range []string{"low", "med", "high"} {
			if catalog.IsEnabled(model.Provider, model.Name, tier) {
				return true
			}
		}
		return false
	}
	return catalog.IsEnabled(model.Provider, model.Name, model.Tier)
}

func findCatalogModel(catalog *modelcatalog.Catalog, model Model) (modelcatalog.Model, bool) {
	for _, entry := range catalog.Models() {
		provider := entry.Identity.Harness
		if provider == "" {
			provider = entry.Identity.Provider
		}
		if provider == model.Provider && entry.Identity.Model == model.Name {
			return entry, true
		}
	}
	return modelcatalog.Model{}, false
}

func resolveModelIn(aliases map[string]modelAlias, spec string) (Model, error) {
	original := strings.TrimSpace(spec)
	if m, ok := resolvePiModel(aliases, original); ok {
		return m, nil
	}
	clean := strings.ToLower(original)
	if strings.Contains(clean, ":") {
		parts := strings.Split(clean, ":")
		if len(parts) == 3 {
			parts[2] = normalizeTier(parts[2])
		} else if len(parts) == 2 {
			for _, candidate := range aliases {
				if modelHasAlias(candidate, parts[0]) {
					parts[1] = normalizeTier(parts[1])
					if !isTier(parts[1]) {
						return Model{}, fmt.Errorf("unknown model tier in requested spec %q; known specs: %s", spec, knownModelSpecs())
					}
					break
				}
			}
		}
		clean = strings.Join(parts, ":")
		if len(parts) == 3 && isTier(parts[2]) {
			m, err := resolveModelIn(aliases, parts[0]+":"+parts[1])
			if err == nil {
				m.Tier = parts[2]
				return m, nil
			}
		}
		if len(parts) == 2 {
			if entry, ok := aliases[clean]; ok {
				m := entry.Model
				if isTier(parts[1]) {
					m.Tier = parts[1]
				}
				return m, nil
			}
			if isTier(parts[1]) {
				if m, err := resolveModelIn(aliases, parts[0]); err == nil {
					m.Tier = parts[1]
					return m, nil
				}
				for _, candidate := range aliases {
					if modelHasAlias(candidate, parts[0]+":"+parts[1]) {
						m := candidate.Model
						m.Tier = parts[1]
						return m, nil
					}
				}
			}
		}
	}
	if !strings.Contains(clean, ":") {
		var match Model
		count := 0
		for _, candidate := range aliases {
			if modelHasAlias(candidate, clean) {
				match, count = candidate.Model, count+1
			}
		}
		if count == 1 {
			return match, nil
		}
		if count > 1 {
			for _, candidate := range aliases {
				if candidate.Preferred && modelHasAlias(candidate, clean) {
					return candidate.Model, nil
				}
			}
		}
		return Model{}, fmt.Errorf("unknown or ambiguous model %q; known specs: %s", spec, knownModelSpecs())
	}
	parts := strings.Split(clean, ":")
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] == "" {
		return Model{}, fmt.Errorf("invalid model %q: expected provider:model[:tier]; known specs: %s", spec, knownModelSpecs())
	}
	entry, ok := aliases[parts[0]+":"+parts[1]]
	if !ok {
		for _, candidate := range aliases {
			if candidate.Model.Name == parts[1] || modelHasAlias(candidate, parts[1]) {
				if entry.Model.Provider != "" {
					entry = modelAlias{}
					break
				}
				entry = candidate
			}
		}
		ok = entry.Model.Provider != ""
	}
	if !ok && len(parts) == 2 {
		for _, candidate := range aliases {
			if modelHasAlias(candidate, parts[0]) {
				if entry.Model.Provider != "" {
					entry = modelAlias{}
					break
				}
				entry = candidate
			}
		}
		ok = entry.Model.Provider != ""
	}
	if !ok {
		return Model{}, fmt.Errorf("unknown model %q; known specs: %s", spec, knownModelSpecs())
	}
	m := entry.Model
	if len(parts) == 3 {
		if parts[0] == "claude" && parts[1] == "haiku" && parts[2] == "latest" {
			return m, nil
		}
		if parts[2] != "low" && parts[2] != "med" && parts[2] != "high" {
			return Model{}, fmt.Errorf("unknown model tier in requested spec %q; known specs: %s", spec, knownModelSpecs())
		}
		m.Tier = parts[2]
	}
	return m, nil
}

func normalizeTier(tier string) string {
	switch tier {
	case "lo", "lower", "l":
		return "low"
	case "medium", "m":
		return "med"
	case "hi", "higher", "h":
		return "high"
	default:
		return tier
	}
}

// resolvePiModel permits Pi model IDs to be selected directly even when they
// are not listed in Harnez's static model guide. Preserve ID case for Pi's
// case-sensitive provider/model identifiers.
func resolvePiModel(aliases map[string]modelAlias, spec string) (Model, bool) {
	parts := strings.Split(spec, ":")
	if len(parts) == 1 && strings.EqualFold(parts[0], "pi") {
		return Model{Provider: "pi", Name: "default", Tier: "low"}, true
	}
	if len(parts) < 2 || len(parts) > 3 || !strings.EqualFold(parts[0], "pi") {
		return Model{}, false
	}
	if len(parts) == 2 && strings.EqualFold(parts[1], "default") {
		return Model{Provider: "pi", Name: "default", Tier: "low"}, true
	}
	if entry, ok := aliases["pi:"+strings.ToLower(parts[1])]; ok && len(parts) == 2 {
		return entry.Model, true
	}
	name := strings.TrimSpace(parts[1])
	if name == "" {
		return Model{}, false
	}
	tier := "low"
	if len(parts) == 3 {
		if !isTier(strings.ToLower(parts[2])) {
			return Model{}, false
		}
		tier = strings.ToLower(parts[2])
	}
	return Model{Provider: "pi", Name: name, Tier: tier}, true
}

func modelHasAlias(m modelAlias, alias string) bool {
	for _, candidate := range m.Aliases {
		if candidate == alias {
			return true
		}
	}
	return false
}

func knownModelSpecs() string {
	return strings.Join(KnownModelSpecs(), ", ")
}

func (m Model) String() string { return m.Provider + ":" + m.Name + ":" + m.Tier }

// isTier reports whether s is a reasoning tier rather than a model name.
func isTier(s string) bool {
	return s == "low" || s == "med" || s == "high"
}
