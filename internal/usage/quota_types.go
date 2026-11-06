package usage

import (
	"fmt"
	"strings"
	"time"

	"ubunatic.com/harnez/quota"
)

// The JSON types of usage --json live in ubunatic.com/harnez/quota (issue 1177
// M2). These aliases keep existing usage.X references compiling; the JSON
// output is unchanged.
type (
	QuotaWindow    = quota.QuotaWindow
	TokenBreakdown = quota.TokenBreakdown
	ModelGroup     = quota.ModelGroup
	AgentUsage     = quota.AgentUsage
	LoadSnapshot   = quota.LoadSnapshot
	UsageSummary   = quota.UsageSummary
	CPULoad        = quota.CPULoad
	GPU            = quota.GPU
	GPUClass       = quota.GPUClass
	SystemMemory   = quota.SystemMemory
)

// hasQuotaSignal reports whether this AgentUsage carries any quota/token
// data — the fields a live recollect can lose when an agent's process isn't
// currently running, even though static fields (Authenticated, Sources, ...)
// still populate from on-disk state regardless (issue 101).
func hasQuotaSignal(a AgentUsage) bool {
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
func hasQuotaWindowSignal(a AgentUsage) bool {
	return a.Session != nil || a.Weekly != nil || len(a.ModelGroups) > 0
}

// IsValueStale reports snapshot-level staleness shared by all of an agent's
// quota windows: a failed fetch or an old snapshot. Window expiry and stale
// labels are evaluated per rendered row by quotaWindowIsStale, so an unused
// leftover window cannot dim fresh siblings. It reads the spec-defined
// staleness threshold, so it lives here rather than in package quota.
func IsValueStale(a AgentUsage) bool {
	if a.QuotaFetchError != "" {
		return true
	}
	return a.IsStale(DefaultCacheStaleness())
}

func quotaWindowIsStale(window QuotaWindow, now time.Time) bool {
	return window.ExpiredAt(now) || strings.Contains(strings.ToLower(window.Name), "(stale)")
}

// StaleReasonText returns a human-readable explanation of why this agent's
// data is considered stale, or empty if it is fresh (issue 714 M1).
func StaleReasonText(a AgentUsage) string {
	if a.QuotaFetchError != "" {
		return "fetch failed: " + a.QuotaFetchError
	}
	now := time.Now()
	stalenessLimit := DefaultCacheStaleness()
	if a.IsStale(stalenessLimit) {
		age := now.Sub(a.LastRefreshed)
		if age < 0 {
			age = 0
		}
		return fmt.Sprintf("snapshot age %s exceeds stale threshold %s", FormatCompactDuration(age), FormatCompactDuration(stalenessLimit))
	}
	return ""
}

// PopulateStaleness computes and sets Stale and StaleReason on this AgentUsage.
func PopulateStaleness(a *AgentUsage) {
	if IsValueStale(*a) {
		a.Stale = true
		reason := StaleReasonText(*a)
		if reason == "" {
			reason = "snapshot stale"
		}
		a.StaleReason = reason
	} else {
		a.Stale = false
		a.StaleReason = ""
	}
}

// CollectorStatus summarizes the provenance of the current agent reading for
// compact and verbose renderers. It intentionally derives from existing
// fields so persisted JSON remains compatible.
func CollectorStatus(a AgentUsage) string {
	if a.QuotaFetchError != "" {
		return "unavailable"
	}
	for _, source := range a.Sources {
		if strings.Contains(source, "usage-history") {
			return "history"
		}
		if strings.Contains(source, "cached") || strings.Contains(source, "cache") {
			return "cache"
		}
	}
	if hasQuotaSignal(a) {
		return "live"
	}
	if len(a.Sources) > 0 || a.Error != "" {
		return "no-data"
	}
	return "unknown"
}
