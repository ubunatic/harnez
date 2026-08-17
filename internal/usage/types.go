package usage

import (
	"time"
)

// QuotaWindow represents a time-bound quota window (e.g. 5-hour session or 7-day rolling).
type QuotaWindow struct {
	Name             string        `json:"name,omitempty"`
	UsedPercent      float64       `json:"used_percent"`
	RemainingPercent float64       `json:"remaining_percent"`
	ResetAt          *time.Time    `json:"reset_at,omitempty"`
	DurationLeft     time.Duration `json:"duration_left,omitempty"`
	IsActive         bool          `json:"is_active,omitempty"`
	Severity         string        `json:"severity,omitempty"`
}

// TokenBreakdown holds detailed token metrics when available locally or remotely.
type TokenBreakdown struct {
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd"`
}

// ModelGroup represents a pool/group of models with their associated quota windows (e.g. Gemini Models vs Claude and GPT Models in AGY).
type ModelGroup struct {
	Name        string        `json:"name"`                  // e.g. "Gemini Models", "Claude and GPT Models"
	Description string        `json:"description,omitempty"`  // e.g. "Models within this group: Gemini Flash, Gemini Pro"
	Windows     []QuotaWindow `json:"windows"`                // e.g. Weekly, 5-Hour limits
}

// AgentUsage models the normalized usage, quota, and session status for a single coding assistant.
type AgentUsage struct {
	AgentID       string                 `json:"agent_id"` // "claude", "agy", "codex"
	Name          string                 `json:"name"`     // "Claude Code", "Antigravity (AGY)", "OpenAI Codex"
	Installed     bool                   `json:"installed"`
	Authenticated bool                   `json:"authenticated"`
	Account       string                 `json:"account,omitempty"`
	PlanTier      string                 `json:"plan_tier,omitempty"`
	ActiveModel   string                 `json:"active_model,omitempty"`
	Tokens        *TokenBreakdown        `json:"tokens,omitempty"`
	Session       *QuotaWindow           `json:"session,omitempty"`
	Weekly        *QuotaWindow           `json:"weekly,omitempty"`
	ModelGroups   []ModelGroup           `json:"model_groups,omitempty"`
	ExtraWindows  map[string]QuotaWindow `json:"extra_windows,omitempty"`
	Details       map[string]string      `json:"details,omitempty"`
	Sources       []string               `json:"sources,omitempty"` // file paths / API endpoints inspected
	Error         string                 `json:"error,omitempty"`
}

// UsageSummary is the top-level container for multi-agent usage queries.
type UsageSummary struct {
	Timestamp time.Time    `json:"timestamp"`
	Agents    []AgentUsage `json:"agents"`
}
