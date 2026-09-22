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
	Resume(context.Context, string, string) (*TurnResult, error)
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
func (d UnsupportedDriver) Resume(context.Context, string, string) (*TurnResult, error) {
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

var modelAliases map[string]Model

// KnownModels returns the configured shorthand specifications in stable order.
func KnownModels() []Model {
	_ = ensureModelAliases()
	keys := make([]string, 0, len(modelAliases))
	for key := range modelAliases {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	models := make([]Model, 0, len(keys))
	for _, key := range keys {
		models = append(models, modelAliases[key])
	}
	return models
}

func (m Model) Spec() string {
	return m.Provider + ":" + modelAliasName(m) + ":" + m.Tier
}

func modelAliasName(m Model) string {
	_ = ensureModelAliases()
	for key, candidate := range modelAliases {
		if candidate == m {
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
	clean := strings.ToLower(strings.TrimSpace(spec))
	if !strings.Contains(clean, ":") {
		var match Model
		count := 0
		for key, candidate := range modelAliases {
			parts := strings.Split(key, ":")
			if len(parts) == 2 && parts[1] == clean {
				match, count = candidate, count+1
			}
		}
		if count == 1 {
			return match, nil
		}
		return Model{}, fmt.Errorf("unknown or ambiguous model %q; known specs: %s", spec, knownModelSpecs())
	}
	parts := strings.Split(clean, ":")
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] == "" {
		return Model{}, fmt.Errorf("invalid model %q: expected provider:model[:tier]; known specs: %s", spec, knownModelSpecs())
	}
	m, ok := modelAliases[parts[0]+":"+parts[1]]
	if !ok {
		return Model{}, fmt.Errorf("unknown model %q; known specs: %s", spec, knownModelSpecs())
	}
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

func knownModelSpecs() string {
	known := make([]string, 0, len(modelAliases))
	for _, model := range KnownModels() {
		known = append(known, model.Spec())
	}
	return strings.Join(known, ", ")
}

func (m Model) String() string { return m.Provider + ":" + m.Name + ":" + m.Tier }
