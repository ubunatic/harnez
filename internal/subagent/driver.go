package subagent

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
	Cost   int    `yaml:"cost"`   // relative cost, luna = 1, astra = 100
	Eff    string `yaml:"eff"`    // tokens per goal: + ~ - ?
	Skills string `yaml:"skills"` // e.g. "Go+ TUI~ SQL?"
	Roles  string `yaml:"roles"`
	Use    string `yaml:"use"`
	UseMed string `yaml:"use_med"` // replaces Use for the :med variant
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
	Messages         []string `json:"messages,omitempty"`
	InputTokens      int      `json:"input_tokens"`
	OutputTokens     int      `json:"output_tokens"`
	CachedTokens     int      `json:"cached_tokens"`
	TokensTurn       int      `json:"tokens_turn"`
	TokensCumulative int      `json:"tokens_cumulative"`
	DurationMS       int64    `json:"duration_ms"`
}

var modelAliases map[string]modelAlias

// KnownModels returns the configured shorthand specifications in stable order.
func KnownModels() []Model {
	if err := ensureModelAliases(); err != nil {
		panic(err)
	}
	keys := make([]string, 0, len(modelAliases))
	for key := range modelAliases {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	models := make([]Model, 0, len(keys))
	for _, key := range keys {
		models = append(models, modelAliases[key].Model)
	}
	return models
}

// KnownModelSpecs lists every selectable spec: each configured model at its
// default tier, plus a :med variant for effort-aware models.
func KnownModelSpecs() []string {
	var specs []string
	for _, e := range KnownModelEntries() {
		specs = append(specs, e.Spec)
	}
	return specs
}

// ModelEntry is one selectable spec with its listing guidance.
type ModelEntry struct {
	Spec  string
	Model Model
	// Effort reports whether the batch driver passes a tier flag.
	Effort bool
	Cost   int
	Eff    string
	Skills string
	Roles  string
	Use    string
}

// KnownModelEntries backs KnownModelSpecs and `harnez agent models`.
func KnownModelEntries() []ModelEntry {
	guides, err := modelGuidesOnce()
	if err != nil {
		panic(err)
	}
	var entries []ModelEntry
	for _, m := range KnownModels() {
		g := guides[m.Provider+":"+modelAliasName(m)]
		effort := m.SupportsEffort()
		entries = append(entries, ModelEntry{Spec: m.Spec(), Model: m, Effort: effort, Cost: g.Cost, Eff: g.Eff, Skills: g.Skills, Roles: g.Roles, Use: g.Use})
		if m.Tier != "med" && effort {
			use := g.Use
			if g.UseMed != "" {
				use = g.UseMed
			}
			entries = append(entries, ModelEntry{Spec: m.Provider + ":" + modelAliasName(m) + ":med", Model: m, Effort: effort, Cost: g.Cost, Eff: g.Eff, Skills: g.Skills, Roles: g.Roles, Use: use})
		}
	}
	return entries
}

func (m Model) Spec() string {
	return m.Provider + ":" + modelAliasName(m) + ":" + m.Tier
}

func modelAliasName(m Model) string {
	if err := ensureModelAliases(); err != nil {
		panic(err)
	}
	for key, candidate := range modelAliases {
		if candidate.Model == m {
			return strings.TrimPrefix(key, m.Provider+":")
		}
	}
	return m.Name
}

// ResolveModel expands provider:model[:tier] shorthand.
func ResolveModel(spec string) (Model, error) {
	if err := ensureModelAliases(); err != nil {
		return Model{}, err
	}
	return resolveModelIn(modelAliases, spec)
}

func resolveModelIn(aliases map[string]modelAlias, spec string) (Model, error) {
	clean := strings.ToLower(strings.TrimSpace(spec))
	if strings.Contains(clean, ":") {
		parts := strings.Split(clean, ":")
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
