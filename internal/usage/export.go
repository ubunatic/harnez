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
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ubunatic.com/harnez/internal/privacy"
	"ubunatic.com/harnez/internal/xdgpath"
)

// ExportPoint is one scrubbed (timestamp, agent) sample, safe for external
// publication. Compared to HistoryEntry/AgentUsage:
//   - Hostname is NOT the raw os.Hostname() value HistoryEntry.Hostname
//     stores verbatim, nor even sanitizeHostname's output — sanitizeHostname
//     only makes a value filesystem-safe (lowercases it and swaps
//     disallowed characters for '-'); it does not anonymize it, and a real
//     machine name routinely embeds a username (e.g.
//     "uwes-workstation.local"). Hostname here is instead a stable, opaque
//     pseudonym ("host-" + first 8 hex characters of HMAC-SHA256(canonical hostname, key)),
//     so per-machine trends in exported timeseries stay stable across exports
//     without the real machine name leaking (issue 735).
//     sanitizeHostname is used to canonicalize hostnames before hashing so that
//     case/formatting differences don't split one real machine into distinct labels.

//   - Account is re-run through MaskAccount. In practice every producer
//     (internal/usage/agy.go, codex.go) already masks Account before it
//     is ever set, but MaskAccount is idempotent on an already-masked
//     value, so re-applying it here is a cheap belt-and-suspenders
//     guarantee against a future producer that forgets to mask.
//   - Sources (file paths / API endpoints inspected) and Details
//     (free-form key/value strings) are dropped entirely at the default
//     privacy level: both are unstructured and have historically held
//     local filesystem paths, with no safe automatic way to scrub
//     arbitrary free text. At privacy.LevelInternal/LevelRaw they survive
//     (scrubbed or raw respectively) as the Sources/Details fields below —
//     see BuildUsageExportLevel's doc comment for why LevelAgentSanitized
//     falls back to the same scrubbing as LevelInternal here rather than
//     going through an LLM (there is no usage-side equivalent of
//     telemetry's free-text "note").
type ExportPoint struct {
	Timestamp          time.Time         `json:"timestamp"`
	Hostname           string            `json:"hostname"`
	AgentID            string            `json:"agent_id"`
	Name               string            `json:"name,omitempty"`
	Account            string            `json:"account,omitempty"`
	PlanTier           string            `json:"plan_tier,omitempty"`
	Installed          bool              `json:"installed"`
	Authenticated      bool              `json:"authenticated"`
	TotalTokens        int64             `json:"total_tokens,omitempty"`
	InputTokens        int64             `json:"input_tokens,omitempty"`
	OutputTokens       int64             `json:"output_tokens,omitempty"`
	CacheReadTokens    int64             `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens   int64             `json:"cache_write_tokens,omitempty"`
	CostUSD            float64           `json:"cost_usd,omitempty"`
	ModelTokens        map[string]int64  `json:"model_tokens,omitempty"`
	SessionUsedPercent float64           `json:"session_used_percent,omitempty"`
	WeeklyUsedPercent  float64           `json:"weekly_used_percent,omitempty"`
	Sources            []string          `json:"sources,omitempty"`
	Details            map[string]string `json:"details,omitempty"`
}

// UsageExport is the top-level JSON payload for `harnez usage export`'s
// usage/token-history half.
type UsageExport struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Points      []ExportPoint `json:"points"`
}

// BuildUsageExport transforms merged history entries (see ReadHistory) into
// the scrubbed UsageExport payload at privacy.LevelPublic — the original,
// pre-issue-204-v2 behavior. Existing callers/tests are unaffected by the
// addition of privacy levels. It is a pure function over its input so it
// can be unit tested without touching disk (see export_test.go).
func BuildUsageExport(entries []HistoryEntry, now time.Time) UsageExport {
	return BuildUsageExportLevel(entries, now, privacy.LevelPublic)
}

// BuildUsageExportLevel is BuildUsageExport's privacy-level-aware form.
// Sources/Details are dropped at LevelPublic, scrubbed via
// privacy.ScrubText at LevelAgentSanitized and LevelInternal (usage has no
// free-text "note" concept for an LLM sanitizer to target — see
// ExportPoint's doc comment — so LevelAgentSanitized intentionally reuses
// LevelInternal's regex scrub here rather than being a no-op), and passed
// through raw at LevelRaw.
func BuildUsageExportLevel(entries []HistoryEntry, now time.Time, level privacy.Level) UsageExport {
	out := UsageExport{GeneratedAt: now, Points: []ExportPoint{}}
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
			switch level {
			case privacy.LevelPublic:
				// Sources/Details stay nil (dropped).
			case privacy.LevelAgentSanitized, privacy.LevelInternal:
				p.Sources = privacy.ScrubStrings(a.Sources)
				p.Details = privacy.ScrubMap(a.Details)
			case privacy.LevelRaw:
				p.Sources = a.Sources
				p.Details = a.Details
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

// ExportHostKeyPath returns the absolute path to the export host HMAC key,
// defaulting to $XDG_DATA_HOME/harnez/export-host-key (issue 735).
func ExportHostKeyPath() (string, error) {
	rel, err := ExportHostKeyRelativePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(xdgpath.DataHome(), "harnez", rel), nil
}

// loadOrCreateHostKey returns the 32-byte secret HMAC key used to hash host
// labels in usage exports. If the key file doesn't exist, it is created with 0600
// permissions.
func loadOrCreateHostKey() ([]byte, error) {
	path, err := ExportHostKeyPath()
	if err != nil {
		return nil, err
	}
	if data, err := os.ReadFile(path); err == nil && len(data) == 32 {
		return data, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate export host key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create export host key dir: %w", err)
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, fmt.Errorf("write export host key: %w", err)
	}
	return key, nil
}

// HashHostLabel generates the stable "host-" + first 8 hex characters of
// HMAC-SHA256(canonical hostname, key) for a given hostname and key.
func HashHostLabel(rawHost string, key []byte) string {
	canonical := sanitizeHostname(rawHost)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(canonical))
	sum := hex.EncodeToString(mac.Sum(nil))
	return "host-" + sum[:8]
}

// anonymizeHostname maps a raw hostname to a stable, opaque pseudonym:
// "host-" + first 8 hex chars of HMAC-SHA256(sanitizeHostname(raw), key).
func anonymizeHostname(labels map[string]string, raw string) string {
	key := sanitizeHostname(raw)
	if label, ok := labels[key]; ok {
		return label
	}
	hostKey, err := loadOrCreateHostKey()
	if err != nil {
		// Fallback if key cannot be loaded
		label := fmt.Sprintf("host-%d", len(labels)+1)
		labels[key] = label
		return label
	}
	label := HashHostLabel(raw, hostKey)
	labels[key] = label
	return label
}

// ExportHistory reads *.jsonl history entries under dir (see HistoryDir)
// and returns the scrubbed UsageExport payload at privacy.LevelPublic.
// Existing callers/tests are unaffected by the addition of privacy levels.
func ExportHistory(dir string, now time.Time) (UsageExport, error) {
	return ExportHistoryLevel(dir, now, privacy.LevelPublic)
}

// ExportHistoryLevel is ExportHistory's privacy-level-aware form.
func ExportHistoryLevel(dir string, now time.Time, level privacy.Level) (UsageExport, error) {
	entries, err := ReadHistory(dir)
	if err != nil {
		return UsageExport{}, err
	}
	return BuildUsageExportLevel(entries, now, level), nil
}
