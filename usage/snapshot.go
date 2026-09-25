// Package usage provides the public, provider-neutral Harnez usage data
// contract and read-only access to persisted snapshots.
//
// The package is intentionally separate from Harnez's provider collectors.
// Reading a snapshot never starts a collector or contacts a provider.
package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SnapshotSchemaVersion is the current version of the public snapshot JSON
// contract. Fields may be added compatibly within a version; breaking changes
// require a new version.
const SnapshotSchemaVersion = 1

const maxSnapshotBytes = 4 << 20

// StateDir returns the default Harnez usage snapshot directory. home may be
// empty to use the current user's home directory. XDG_STATE_HOME, when set,
// takes precedence.
func StateDir(home string) string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "harnez", "agents", "usage")
	}
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return filepath.Join(home, ".local", "state", "harnez", "agents", "usage")
}

// ProviderID identifies a provider in persisted snapshots and API responses.
type ProviderID string

const (
	ProviderClaude ProviderID = "claude"
	ProviderAGY    ProviderID = "agy"
	ProviderCodex  ProviderID = "codex"
)

// SnapshotStatus describes how the snapshot's usage data was obtained. A
// legacy snapshot without explicit fetch metadata is StatusUnknown.
type SnapshotStatus string

const (
	StatusUnknown SnapshotStatus = "unknown"
	StatusLive    SnapshotStatus = "live"
	StatusCached  SnapshotStatus = "cached"
	StatusStale   SnapshotStatus = "stale"
	StatusSkipped SnapshotStatus = "skipped"
	StatusError   SnapshotStatus = "error"
)

// SnapshotSource identifies the kind of source that produced the snapshot.
// Values are extensible; consumers should preserve or display unknown values.
type SnapshotSource string

const (
	SourceUnknown    SnapshotSource = "unknown"
	SourceLive       SnapshotSource = "live"
	SourceCache      SnapshotSource = "cache"
	SourceStateCache SnapshotSource = "state_cache"
	SourceHistory    SnapshotSource = "history"
	SourceLegacy     SnapshotSource = "legacy"
)

// ErrorCategory classifies a refresh failure without exposing provider error
// text or response bodies. Values are extensible; consumers should preserve
// or display unknown values.
type ErrorCategory string

const (
	ErrorProviderFetch ErrorCategory = "provider_fetch"
)

// Snapshot is one provider's usage observation. FetchedAt is when the
// collector wrote this snapshot; ObservedAt is when the represented usage was
// last known to be current. A zero ObservedAt means that time is unknown.
type Snapshot struct {
	SchemaVersion int            `json:"schema_version"`
	ProviderID    ProviderID     `json:"provider_id"`
	FetchedAt     time.Time      `json:"fetched_at"`
	ObservedAt    time.Time      `json:"observed_at,omitempty"`
	Status        SnapshotStatus `json:"status"`
	Source        SnapshotSource `json:"source,omitempty"`
	Error         *FetchError    `json:"error,omitempty"`
	Usage         UsageData      `json:"usage"`
}

// FetchError contains safe, machine-readable fetch failure metadata. Provider
// error messages and response bodies are deliberately excluded.
type FetchError struct {
	Category   ErrorCategory `json:"category"`
	RetryAfter *time.Time    `json:"retry_after,omitempty"`
}

// UsageData contains normalized usage values suitable for independent
// applications. It deliberately excludes credentials, local file paths,
// provider response bodies, and renderer-specific diagnostics.
type UsageData struct {
	Name          string `json:"name,omitempty"`
	Installed     bool   `json:"installed"`
	Authenticated bool   `json:"authenticated"`
	// Account may contain a masked account identifier. Treat it as potentially
	// identifying data when displaying, exporting, or persisting snapshots.
	Account      string                 `json:"account,omitempty"`
	PlanTier     string                 `json:"plan_tier,omitempty"`
	ActiveModel  string                 `json:"active_model,omitempty"`
	Tokens       *TokenUsage            `json:"tokens,omitempty"`
	ModelTokens  map[string]int64       `json:"model_tokens,omitempty"`
	Session      *QuotaWindow           `json:"session,omitempty"`
	Weekly       *QuotaWindow           `json:"weekly,omitempty"`
	ModelGroups  []ModelGroup           `json:"model_groups,omitempty"`
	ExtraWindows map[string]QuotaWindow `json:"extra_windows,omitempty"`
}

// TokenUsage contains provider-reported token and cost totals when available.
type TokenUsage struct {
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
}

// QuotaWindow describes one time-bounded provider quota. ResetAt is an
// absolute timestamp; DurationLeftMS is a compatibility hint for sources
// that do not provide an absolute reset time.
type QuotaWindow struct {
	Name             string     `json:"name,omitempty"`
	Source           string     `json:"source,omitempty"`
	UsedPercent      float64    `json:"used_percent"`
	RemainingPercent float64    `json:"remaining_percent"`
	ResetAt          *time.Time `json:"reset_at,omitempty"`
	DurationLeftMS   int64      `json:"duration_left_ms,omitempty"`
	IsActive         bool       `json:"is_active,omitempty"`
	Severity         string     `json:"severity,omitempty"`
}

// ModelGroup groups quota windows that apply to a family of models.
type ModelGroup struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Windows     []QuotaWindow `json:"windows"`
}

// ReadSnapshot reads <provider>.json from stateDir. Missing snapshots return
// (nil, nil). Both the current public envelope and Harnez's legacy
// {fetched_at, usage} envelope are accepted. Legacy files are mapped into the
// public DTO, and fields that may contain diagnostics or local paths are
// intentionally discarded.
func ReadSnapshot(stateDir string, provider ProviderID) (*Snapshot, error) {
	if !validProviderID(provider) {
		return nil, fmt.Errorf("usage: invalid provider ID %q", provider)
	}
	if stateDir == "" {
		stateDir = StateDir("")
	}
	path := filepath.Join(stateDir, string(provider)+".json")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("usage: read %s snapshot: %w", provider, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxSnapshotBytes+1))
	if err != nil {
		return nil, fmt.Errorf("usage: read %s snapshot: %w", provider, err)
	}
	if len(data) > maxSnapshotBytes {
		return nil, fmt.Errorf("usage: %s snapshot exceeds %d-byte limit", provider, maxSnapshotBytes)
	}

	var header map[string]json.RawMessage
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("usage: parse %s snapshot: %w", provider, err)
	}
	if header == nil {
		return nil, fmt.Errorf("usage: parse %s snapshot: expected a JSON object", provider)
	}
	rawVersion, versioned := header["schema_version"]
	if versioned {
		var version int
		if err := json.Unmarshal(rawVersion, &version); err != nil {
			return nil, fmt.Errorf("usage: parse %s snapshot schema version: %w", provider, err)
		}
		if version != SnapshotSchemaVersion {
			return nil, fmt.Errorf("usage: unsupported snapshot schema version %d for %s (supported: %d)", version, provider, SnapshotSchemaVersion)
		}
	}
	rawUsage, hasUsage := header["usage"]
	if !hasUsage || !isJSONObject(rawUsage) {
		return nil, fmt.Errorf("usage: parse %s snapshot: missing or invalid usage object", provider)
	}
	if versioned {
		var snapshot Snapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return nil, fmt.Errorf("usage: decode %s snapshot: %w", provider, err)
		}
		if snapshot.ProviderID != provider {
			return nil, fmt.Errorf("usage: snapshot provider ID %q does not match requested provider %q", snapshot.ProviderID, provider)
		}
		if err := snapshot.validate(); err != nil {
			return nil, fmt.Errorf("usage: invalid %s snapshot: %w", provider, err)
		}
		return &snapshot, nil
	}
	var legacy legacySnapshot
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("usage: decode legacy %s snapshot: %w", provider, err)
	}
	if legacy.Usage.AgentID == "" {
		return nil, fmt.Errorf("usage: decode legacy %s snapshot: missing agent_id", provider)
	}
	if legacy.FetchedAt.IsZero() {
		return nil, fmt.Errorf("usage: decode legacy %s snapshot: missing fetched_at", provider)
	}
	if ProviderID(legacy.Usage.AgentID) != provider {
		return nil, fmt.Errorf("usage: legacy snapshot provider ID %q does not match requested provider %q", legacy.Usage.AgentID, provider)
	}

	status := StatusUnknown
	var fetchError *FetchError
	if legacy.Usage.QuotaFetchError != "" {
		status = StatusError
		fetchError = &FetchError{Category: ErrorProviderFetch}
	}
	return &Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		ProviderID:    provider,
		FetchedAt:     legacy.FetchedAt,
		ObservedAt:    legacy.Usage.LastRefreshed,
		Status:        status,
		Source:        SourceLegacy,
		Error:         fetchError,
		Usage:         legacy.Usage.publicUsage(),
	}, nil
}

func isJSONObject(raw json.RawMessage) bool {
	var object map[string]json.RawMessage
	return json.Unmarshal(raw, &object) == nil && object != nil
}

func (s Snapshot) validate() error {
	if !validProviderID(s.ProviderID) {
		return fmt.Errorf("invalid provider ID %q", s.ProviderID)
	}
	if s.FetchedAt.IsZero() {
		return fmt.Errorf("missing fetched_at")
	}
	switch s.Status {
	case StatusUnknown, StatusLive, StatusCached, StatusStale, StatusSkipped, StatusError:
	default:
		return fmt.Errorf("unknown status %q", s.Status)
	}
	return nil
}

func validProviderID(provider ProviderID) bool {
	value := string(provider)
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, `/\\`) {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// The legacy decoder deliberately models only persisted fields needed by the
// public contract. In particular, it does not decode Details, Sources, Error,
// QuotaFetchError text, or any potential credential-bearing fields.
type legacySnapshot struct {
	FetchedAt time.Time        `json:"fetched_at"`
	Usage     legacyAgentUsage `json:"usage"`
}

type legacyAgentUsage struct {
	AgentID         string                       `json:"agent_id"`
	Name            string                       `json:"name"`
	Installed       bool                         `json:"installed"`
	Authenticated   bool                         `json:"authenticated"`
	Account         string                       `json:"account,omitempty"`
	PlanTier        string                       `json:"plan_tier,omitempty"`
	ActiveModel     string                       `json:"active_model,omitempty"`
	Tokens          *TokenUsage                  `json:"tokens,omitempty"`
	ModelTokens     map[string]int64             `json:"model_tokens,omitempty"`
	Session         *legacyQuotaWindow           `json:"session,omitempty"`
	Weekly          *legacyQuotaWindow           `json:"weekly,omitempty"`
	ModelGroups     []legacyModelGroup           `json:"model_groups,omitempty"`
	ExtraWindows    map[string]legacyQuotaWindow `json:"extra_windows,omitempty"`
	QuotaFetchError string                       `json:"quota_fetch_error,omitempty"`
	LastRefreshed   time.Time                    `json:"last_refreshed,omitempty"`
}

type legacyQuotaWindow struct {
	Name             string     `json:"name,omitempty"`
	Source           string     `json:"source,omitempty"`
	UsedPercent      float64    `json:"used_percent"`
	RemainingPercent float64    `json:"remaining_percent"`
	ResetAt          *time.Time `json:"reset_at,omitempty"`
	DurationLeft     int64      `json:"duration_left,omitempty"`
	IsActive         bool       `json:"is_active,omitempty"`
	Severity         string     `json:"severity,omitempty"`
}

type legacyModelGroup struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Windows     []legacyQuotaWindow `json:"windows"`
}

func (u legacyAgentUsage) publicUsage() UsageData {
	result := UsageData{
		Name:          u.Name,
		Installed:     u.Installed,
		Authenticated: u.Authenticated,
		Account:       u.Account,
		PlanTier:      u.PlanTier,
		ActiveModel:   u.ActiveModel,
		Tokens:        u.Tokens,
		ModelTokens:   u.ModelTokens,
		ExtraWindows:  make(map[string]QuotaWindow, len(u.ExtraWindows)),
	}
	if u.Session != nil {
		window := u.Session.publicWindow()
		result.Session = &window
	}
	if u.Weekly != nil {
		window := u.Weekly.publicWindow()
		result.Weekly = &window
	}
	for key, window := range u.ExtraWindows {
		result.ExtraWindows[key] = window.publicWindow()
	}
	if len(result.ExtraWindows) == 0 {
		result.ExtraWindows = nil
	}
	if len(u.ModelGroups) > 0 {
		result.ModelGroups = make([]ModelGroup, len(u.ModelGroups))
		for i, group := range u.ModelGroups {
			result.ModelGroups[i] = ModelGroup{Name: group.Name, Description: group.Description}
			if len(group.Windows) > 0 {
				result.ModelGroups[i].Windows = make([]QuotaWindow, len(group.Windows))
				for j, window := range group.Windows {
					result.ModelGroups[i].Windows[j] = window.publicWindow()
				}
			}
		}
	}
	return result
}

func (w legacyQuotaWindow) publicWindow() QuotaWindow {
	durationLeftMS := w.DurationLeft / int64(time.Millisecond)
	if durationLeftMS < 0 {
		durationLeftMS = 0
	}
	return QuotaWindow{
		Name:             w.Name,
		Source:           w.Source,
		UsedPercent:      w.UsedPercent,
		RemainingPercent: w.RemainingPercent,
		ResetAt:          w.ResetAt,
		DurationLeftMS:   durationLeftMS,
		IsActive:         w.IsActive,
		Severity:         w.Severity,
	}
}
