package agent

import (
	"time"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/modelcatalog"
)

// ModelRow is one `harnez agent models --json` element (issue 1122). Fields
// are documented in docs/AgentModelsJSON.md and validated by
// spec/schemas/agent-models-row.schema.json.
type ModelRow struct {
	Spec     string `json:"spec"`
	Provider string `json:"provider"`
	// Model is the unchanged object (Provider, Name, Tier, Effort) that
	// existing consumers read (ticket 983); ModelName is the plain name.
	Model           subagent.Model `json:"model"`
	ModelName       string         `json:"model_name"`
	Batch           bool           `json:"batch"`
	Effort          bool           `json:"effort"`
	Cost            int            `json:"cost"`
	Eff             string         `json:"eff"`
	Skills          string         `json:"skills"`
	Roles           string         `json:"roles"`
	Use             string         `json:"use"`
	Source          string         `json:"source"`
	Quota           RowQuota       `json:"quota"`
	Startable       bool           `json:"startable"`
	StartableReason string         `json:"startable_reason,omitempty"`
	Entitlement     RowEntitlement `json:"entitlement"`
	BlockedUntil    *time.Time     `json:"blocked_until,omitempty"`
	BlockReason     string         `json:"block_reason,omitempty"`
}

// RowQuota mirrors the text AVAILABILITY column: State is available,
// exhausted, stale or unknown; AgeS is the age of the newest reading.
type RowQuota struct {
	State            string     `json:"state"`
	RemainingPercent *float64   `json:"remaining_percent"`
	ResetAt          *time.Time `json:"reset_at"`
	AgeS             int64      `json:"age_s"`
	Scope            string     `json:"scope"`
}

// RowEntitlement says whether the owner may use the model, separately from
// quota. `unverified` means nothing is known beyond the spec listing.
type RowEntitlement struct {
	State     string     `json:"state"`
	Source    string     `json:"source"`
	CheckedAt *time.Time `json:"checked_at"`
}

// ModelRows builds the JSON rows from catalog entries and cached quota
// evidence. It reads no live state: availability and enabled are injected, so
// `--offline` callers never start a collector.
func ModelRows(entries []subagent.ModelEntry, availability map[string]usage.ProviderQuotaAvailability, enabled func(subagent.Model) bool, now time.Time) []ModelRow {
	rows := make([]ModelRow, 0, len(entries))
	for _, e := range entries {
		a := availability[e.Model.Provider+"\x00"+e.Model.Name]
		state := a.State
		if state == "" {
			state = "unknown"
		}
		row := ModelRow{
			Spec: e.Spec, Provider: e.Model.Provider, Model: e.Model, ModelName: e.Model.Name,
			Batch: e.Batch, Effort: e.Effort, Cost: e.Cost, Eff: e.Eff, Skills: e.Skills,
			Roles: e.Roles, Use: e.Use, Source: e.Source,
			Quota: RowQuota{State: state, RemainingPercent: remainingPercent(a), AgeS: int64(a.Age.Seconds()), Scope: quotaScope(e.Model.Provider, a)},
		}
		if state == "exhausted" {
			reset := now.Add(a.ResetIn).UTC()
			row.Quota.ResetAt = &reset
		}
		row.Entitlement = RowEntitlement{State: "unverified", Source: "spec"}
		if access, checked, err := modelcatalog.LatestModelAccess(e.Model.Provider, e.Model.Name); err == nil {
			if access == "unsupported" || access == "denied" {
				row.Entitlement = RowEntitlement{State: "denied", Source: "model-access", CheckedAt: checked}
			}
			if access == "accessible" {
				row.Entitlement = RowEntitlement{State: "verified", Source: "model-access", CheckedAt: checked}
			}
		}
		if enabled != nil && !enabled(e.Model) {
			row.Entitlement = RowEntitlement{State: "denied", Source: "model-policy"}
		}
		row.Startable = true
		switch {
		case !a.BlockedUntil.IsZero():
			until := a.BlockedUntil.UTC()
			row.BlockedUntil, row.BlockReason = &until, a.BlockReason
			row.Startable, row.StartableReason = false, "blocked until "+until.Format(time.RFC3339)
		case state == "exhausted":
			row.Startable, row.StartableReason = false, "quota exhausted"
		case row.Entitlement.Source == "model-access" && row.Entitlement.State == "denied":
			row.Startable, row.StartableReason = false, "unsupported for this account"
		case row.Entitlement.State == "denied":
			row.Startable, row.StartableReason = false, "model disabled by policy"
		}
		rows = append(rows, row)
	}
	return rows
}

func remainingPercent(a usage.ProviderQuotaAvailability) *float64 {
	if len(a.Windows) == 0 {
		return nil
	}
	min := 100.0
	for _, w := range a.Windows {
		if left := (1 - w.UsedFraction) * 100; left < min {
			min = left
		}
	}
	if min < 0 {
		min = 0
	}
	return &min
}

// quotaScope is "model" when the evidence is specific to the model (a local
// block, or AGY's per-model pools) and "provider" otherwise.
func quotaScope(provider string, a usage.ProviderQuotaAvailability) string {
	if !a.BlockedUntil.IsZero() || provider == "agy" {
		return "model"
	}
	return "provider"
}
