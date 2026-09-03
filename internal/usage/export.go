// export.go implements issue 204's sanitized JSON export of recorded
// usage-history snapshots (internal/usage/history.go's HistoryEntry) for
// external visualization tools. Every field that could carry PII is either
// scrubbed through an existing helper (MaskAccount, sanitizeHostname) or
// dropped outright.
//
// Scope note (issue 204, first pass): JSON only. A SQLite export format is
// deliberately deferred — see the ticket's "Progress / Scope Note" section.
package usage

import (
	"fmt"
	"time"
)

// ExportPoint is one scrubbed (timestamp, agent) sample, safe for external
// publication. Compared to HistoryEntry/AgentUsage:
//   - Hostname is NOT the raw os.Hostname() value HistoryEntry.Hostname
//     stores verbatim, nor even sanitizeHostname's output — sanitizeHostname
//     only makes a value filesystem-safe (lowercases it and swaps
//     disallowed characters for '-'); it does not anonymize it, and a real
//     machine name routinely embeds a username (e.g.
//     "uwes-workstation.local"). Hostname here is instead an opaque,
//     per-export-run label ("host-1", "host-2", ...) assigned in first-seen
//     order via anonymizeHostnames, so per-machine trends in the exported
//     timeseries stay distinguishable without the real name leaking.
//     sanitizeHostname is still used as the map key so that
//     case/formatting differences in how a hostname was recorded don't
//     accidentally split one real machine into two anonymous labels.
//   - Account is re-run through MaskAccount. In practice every producer
//     (internal/usage/agy.go, codex.go) already masks Account before it
//     is ever set, but MaskAccount is idempotent on an already-masked
//     value, so re-applying it here is a cheap belt-and-suspenders
//     guarantee against a future producer that forgets to mask.
//   - Sources (file paths / API endpoints inspected) and Details
//     (free-form key/value strings) are dropped entirely: both are
//     unstructured and have historically held local filesystem paths,
//     with no safe automatic way to scrub arbitrary free text.
type ExportPoint struct {
	Timestamp          time.Time        `json:"timestamp"`
	Hostname           string           `json:"hostname"`
	AgentID            string           `json:"agent_id"`
	Name               string           `json:"name,omitempty"`
	Account            string           `json:"account,omitempty"`
	PlanTier           string           `json:"plan_tier,omitempty"`
	Installed          bool             `json:"installed"`
	Authenticated      bool             `json:"authenticated"`
	TotalTokens        int64            `json:"total_tokens,omitempty"`
	InputTokens        int64            `json:"input_tokens,omitempty"`
	OutputTokens       int64            `json:"output_tokens,omitempty"`
	CacheReadTokens    int64            `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens   int64            `json:"cache_write_tokens,omitempty"`
	CostUSD            float64          `json:"cost_usd,omitempty"`
	ModelTokens        map[string]int64 `json:"model_tokens,omitempty"`
	SessionUsedPercent float64          `json:"session_used_percent,omitempty"`
	WeeklyUsedPercent  float64          `json:"weekly_used_percent,omitempty"`
}

// UsageExport is the top-level JSON payload for `harnez usage export`'s
// usage/token-history half.
type UsageExport struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Points      []ExportPoint `json:"points"`
}

// BuildUsageExport transforms merged history entries (see ReadHistory) into
// the scrubbed UsageExport payload. It is a pure function over its input so
// it can be unit tested without touching disk (see export_test.go).
func BuildUsageExport(entries []HistoryEntry, now time.Time) UsageExport {
	out := UsageExport{GeneratedAt: now}
	hostLabels := make(map[string]string)
	for _, e := range entries {
		host := anonymizeHostname(hostLabels, e.Hostname)
		for _, a := range e.Agents {
			p := ExportPoint{
				Timestamp:     e.Timestamp,
				Hostname:      host,
				AgentID:       a.AgentID,
				Name:          a.Name,
				Account:       MaskAccount(a.Account),
				PlanTier:      a.PlanTier,
				Installed:     a.Installed,
				Authenticated: a.Authenticated,
			}
			if a.Tokens != nil {
				p.TotalTokens = a.Tokens.TotalTokens
				p.InputTokens = a.Tokens.InputTokens
				p.OutputTokens = a.Tokens.OutputTokens
				p.CacheReadTokens = a.Tokens.CacheReadTokens
				p.CacheWriteTokens = a.Tokens.CacheWriteTokens
				p.CostUSD = a.Tokens.CostUSD
			}
			if len(a.ModelTokens) > 0 {
				p.ModelTokens = a.ModelTokens
			}
			if a.Session != nil {
				p.SessionUsedPercent = a.Session.UsedPercent
			}
			if a.Weekly != nil {
				p.WeeklyUsedPercent = a.Weekly.UsedPercent
			}
			out.Points = append(out.Points, p)
		}
	}
	return out
}

// anonymizeHostname maps a raw hostname to a stable, opaque per-export-run
// label ("host-1", "host-2", ...), assigned in first-seen order and cached
// in labels (keyed by sanitizeHostname's canonicalized form) so the same
// real machine gets the same label across every entry in one export call.
// See ExportPoint's Hostname doc for why sanitizeHostname alone is not
// sufficient here.
func anonymizeHostname(labels map[string]string, raw string) string {
	key := sanitizeHostname(raw)
	if label, ok := labels[key]; ok {
		return label
	}
	label := fmt.Sprintf("host-%d", len(labels)+1)
	labels[key] = label
	return label
}

// ExportHistory reads *.jsonl history entries under dir (see HistoryDir)
// and returns the scrubbed UsageExport payload.
func ExportHistory(dir string, now time.Time) (UsageExport, error) {
	entries, err := ReadHistory(dir)
	if err != nil {
		return UsageExport{}, err
	}
	return BuildUsageExport(entries, now), nil
}
