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
	QuotaFetchError string `json:"quota_fetch_error,omitempty"`
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

// hasQuotaSignal reports whether this AgentUsage carries any quota/token
// data — the fields a live recollect can lose when an agent's process isn't
// currently running, even though static fields (Authenticated, Sources, ...)
// still populate from on-disk state regardless (issue 101).
func (a AgentUsage) hasQuotaSignal() bool {
	return a.Session != nil || a.Weekly != nil || a.Tokens != nil || len(a.ModelGroups) > 0
}

// hasQuotaWindowSignal reports whether this AgentUsage carries any actual
// quota/rate-limit *window* data (Session, Weekly, or a model group's
// window) — deliberately narrower than hasQuotaSignal, which also counts
// Tokens. An `--offline` collect can produce real local token totals
// (Tokens) while having skipped the live quota API entirely, so it lacks
// Session/Weekly/ModelGroups but still satisfies hasQuotaSignal(). That
// combination is exactly the degraded snapshot issue 086 describes: a
// smoke-test `agent-collector --once --offline` run wrote a snapshot with
// token totals but no quota bars, which was then served as if it were a
// complete reading. PersistAgentSnapshot's offline guard uses this
// narrower predicate so such a snapshot is recognized as incomplete even
// though it has Tokens.
func (a AgentUsage) hasQuotaWindowSignal() bool {
	return a.Session != nil || a.Weekly != nil || len(a.ModelGroups) > 0
}

// IsStale reports whether this agent's LastRefreshed timestamp is older than
// maxAge. A zero LastRefreshed — not yet threaded through by the caller that
// built this AgentUsage — is never considered stale here; such callers rely
// on HasUsageData() alone for visibility (issue 101's 7-day auto-hide gate
// is additive to, not a replacement for, issue 083's check).
func (a AgentUsage) IsStale(maxAge time.Duration) bool {
	if a.LastRefreshed.IsZero() {
		return false
	}
	return time.Since(a.LastRefreshed) > maxAge
}

// HasUsageData reports whether a collector actually found real local or
// remote state for this agent — credentials, local token history, quota
// windows, a live quota-fetch attempt, or at least one inspected source file
// — as opposed to an agent that simply isn't installed on this machine or
// has an empty/unconfigured state directory. Installed alone is not enough:
// a bare, unconfigured config dir still leaves Installed true but carries no
// other signal. Reuses existing fields rather than adding a new one (issue
// 083: self-hiding, auto-discovery agent display).
func (a AgentUsage) HasUsageData() bool {
	if !a.Installed {
		return false
	}
	return a.Authenticated ||
		a.Tokens != nil ||
		a.Session != nil ||
		a.Weekly != nil ||
		len(a.ModelGroups) > 0 ||
		len(a.ModelTokens) > 0 ||
		len(a.Sources) > 0 ||
		a.Error != "" ||
		a.QuotaFetchError != ""
}

// UsageSummary is the top-level container for multi-agent usage queries.
type UsageSummary struct {
	Timestamp time.Time    `json:"timestamp"`
	Agents    []AgentUsage `json:"agents"`
}
