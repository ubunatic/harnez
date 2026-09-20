package subagent

import (
	"context"
	"fmt"
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
	SessionID        string `json:"session_id,omitempty"`
	Response         string `json:"response"`
	InputTokens      int    `json:"input_tokens"`
	OutputTokens     int    `json:"output_tokens"`
	CachedTokens     int    `json:"cached_tokens"`
	TokensTurn       int    `json:"tokens_turn"`
	TokensCumulative int    `json:"tokens_cumulative"`
	DurationMS       int64  `json:"duration_ms"`
}

var modelAliases = map[string]Model{
	"codex:luna": {"codex", "gpt-5.6-luna", "low"}, "codex:sol": {"codex", "gpt-5.6-sol", "low"}, "codex:astra": {"codex", "gpt-5.6-astra", "low"},
	"claude:haiku": {"claude", "claude-3-5-haiku-20241022", "low"}, "claude:sonnet": {"claude", "claude-3-7-sonnet-20250219", "low"}, "claude:opus": {"claude", "claude-3-opus-20240229", "low"},
	"agy:flash": {"agy", "gemini-3.7-flash", "low"},
}

// ResolveModel expands provider:model[:tier] shorthand.
func ResolveModel(spec string) (Model, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(spec)), ":")
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] == "" {
		return Model{}, fmt.Errorf("invalid model %q: expected provider:model[:tier]", spec)
	}
	m, ok := modelAliases[parts[0]+":"+parts[1]]
	if !ok {
		return Model{}, fmt.Errorf("unknown model %q", spec)
	}
	if len(parts) == 3 {
		if parts[2] != "low" && parts[2] != "med" && parts[2] != "high" {
			return Model{}, fmt.Errorf("unknown model tier %q", parts[2])
		}
		m.Tier = parts[2]
	}
	return m, nil
}

func (m Model) String() string { return m.Provider + ":" + m.Name + ":" + m.Tier }
