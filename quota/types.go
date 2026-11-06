package quota

import (
	"encoding/json"
	"time"
)

// QuotaWindow represents a time-bound quota window (e.g. 5-hour session or 7-day rolling).
type QuotaWindow struct {
	Name             string        `json:"name,omitempty"`
	Source           string        `json:"source,omitempty"`
	UsedPercent      float64       `json:"used_percent"`
	RemainingPercent float64       `json:"remaining_percent"`
	ResetAt          *time.Time    `json:"reset_at,omitempty"`
	DurationLeft     time.Duration `json:"duration_left,omitempty"`
	IsActive         bool          `json:"is_active,omitempty"`
	Severity         string        `json:"severity,omitempty"`
}

// RemainingAt derives the live countdown from ResetAt when available. The
// stored DurationLeft remains a compatibility fallback for older snapshots.
func (w QuotaWindow) RemainingAt(now time.Time) time.Duration {
	if w.ResetAt != nil {
		remaining := w.ResetAt.Sub(now)
		if remaining < 0 {
			return 0
		}
		return remaining
	}
	if w.DurationLeft < 0 {
		return 0
	}
	return w.DurationLeft
}

// ExpiredAt reports whether an absolute reset timestamp has passed.
func (w QuotaWindow) ExpiredAt(now time.Time) bool {
	return w.ResetAt != nil && !w.ResetAt.After(now)
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
	Description string        `json:"description,omitempty"` // e.g. "Models within this group: Gemini Flash, Gemini Pro"
	Windows     []QuotaWindow `json:"windows"`               // e.g. Weekly, 5-Hour limits
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
	ModelTokens   map[string]int64       `json:"model_tokens,omitempty"`
	Session       *QuotaWindow           `json:"session,omitempty"`
	Weekly        *QuotaWindow           `json:"weekly,omitempty"`
	ModelGroups   []ModelGroup           `json:"model_groups,omitempty"`
	ExtraWindows  map[string]QuotaWindow `json:"extra_windows,omitempty"`
	Details       map[string]string      `json:"details,omitempty"`
	Sources       []string               `json:"sources,omitempty"` // file paths / API endpoints inspected
	Error         string                 `json:"error,omitempty"`
	// QuotaFetchError is set whenever a live quota/rate-limit call was
	// attempted but failed (transport error, non-2xx, decode error), so
	// renderers can distinguish "fetch failed" from "no quota windows apply."
	QuotaFetchError      string `json:"quota_fetch_error,omitempty"`
	QuotaFetchCause      string `json:"quota_fetch_cause,omitempty"`
	QuotaFetchDurationMS int64  `json:"-"`
	// Stale reports whether this agent's currently-displayed quota values
	// should be treated as stale / not-reliably-live (issue 714 M1).
	Stale bool `json:"stale"`
	// StaleReason explains why the agent is stale when Stale is true.
	StaleReason string `json:"stale_reason,omitempty"`
	// LastRefreshed is when this AgentUsage's data was actually last known to
	// be current — the collector-daemon cache snapshot's FetchedAt time when
	// served from cache, or "now" when freshly live-collected. It is the
	// basis for both the renderers' "last updated" annotation and the
	// 7-day auto-hide gate (issue 101). Left zero by callers that construct
	// an AgentUsage directly (e.g. tests) rather than through CollectAll/
	// cacheOrLive; a zero value is treated as "unknown" rather than "stale"
	// by IsStale.
	LastRefreshed time.Time `json:"last_refreshed,omitempty"`
}

// IsStale reports whether this agent's LastRefreshed timestamp is older than
// maxAge. A zero LastRefreshed — not yet threaded through by the caller that
// built this AgentUsage — is never considered stale here; such callers rely
// internal/usage's HasUsageData alone for visibility (issue 101's 7-day auto-hide gate
// is additive to, not a replacement for, issue 083's check).
func (a AgentUsage) IsStale(maxAge time.Duration) bool {
	if a.LastRefreshed.IsZero() {
		return false
	}
	return time.Since(a.LastRefreshed) > maxAge
}

// LoadSnapshot is a compact CPU/GPU load reading carried alongside
// UsageSummary so the `[L] Load` panel can be rendered from remote data.
// `harnez load-stream` emits it with the time the remote host observed it;
// the server cache retains that time so transport retries do not make old
// readings look fresh.
type LoadSnapshot struct {
	CPU        CPULoad   `json:"cpu"`
	GPUs       []GPU     `json:"gpus,omitempty"`
	ObservedAt time.Time `json:"observed_at,omitempty"`
}

// UsageSummary is the top-level container for multi-agent usage queries.
type UsageSummary struct {
	PassiveNoQuota map[string]uint64 `json:"passive_no_quota,omitempty"`
	Timestamp      time.Time         `json:"timestamp"`
	Agents         []AgentUsage      `json:"agents"`
	Load           *LoadSnapshot     `json:"load,omitempty"`
}

// agentUsageJSON is AgentUsage without its methods, so the JSON hooks below
// can reuse the default encoding.
type agentUsageJSON AgentUsage

// MarshalJSON reports a long quota window under its real period: a window
// named "Monthly" goes to "monthly", not "weekly" (issue 1105). Internally
// the Weekly field stays the single long-window slot the renderers use.
func (u AgentUsage) MarshalJSON() ([]byte, error) {
	out := struct {
		agentUsageJSON
		Monthly *QuotaWindow `json:"monthly,omitempty"`
	}{agentUsageJSON: agentUsageJSON(u)}
	if u.Weekly != nil && u.Weekly.Name == "Monthly" {
		out.Monthly, out.Weekly = u.Weekly, nil
	}
	return json.Marshal(out)
}

// UnmarshalJSON reads "monthly" back into the long-window slot, so stored
// snapshots round-trip.
func (u *AgentUsage) UnmarshalJSON(data []byte) error {
	var in struct {
		agentUsageJSON
		Monthly *QuotaWindow `json:"monthly,omitempty"`
	}
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	*u = AgentUsage(in.agentUsageJSON)
	if u.Weekly == nil && in.Monthly != nil {
		u.Weekly = in.Monthly
	}
	return nil
}
