package usage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"ubunatic.com/harnez/internal/usagestore"
	"ubunatic.com/harnez/internal/xdgpath"
)

// UsageStorePath resolves the canonical store path without creating it or
// migrating the legacy telemetry database.
func UsageStorePath() string {
	return filepath.Join(xdgpath.DataHome(), "harnez", "telemetry.sqlite")
}

// ReadStoredSummary reads the latest collector snapshot without permitting
// schema setup, migrations, or writes.
func ReadStoredSummary(ctx context.Context, dbPath string) (UsageSummary, error) {
	if dbPath == "" {
		dbPath = UsageStorePath()
	}
	if _, err := os.Stat(dbPath); err != nil {
		return UsageSummary{}, err
	}
	store, err := usagestore.OpenReadOnly(dbPath)
	if err != nil {
		return UsageSummary{}, err
	}
	defer store.Close()
	record, err := store.LatestUsageSummary(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return UsageSummary{}, fmt.Errorf("usage store has no collector snapshots yet")
		}
		return UsageSummary{}, err
	}
	var summary UsageSummary
	if err := json.Unmarshal(record.Payload, &summary); err != nil {
		return UsageSummary{}, fmt.Errorf("decode usage snapshot: %w", err)
	}
	summary.Timestamp = record.ObservedAt
	return summary, nil
}

func storedSummaryOrError(ctx context.Context) UsageSummary {
	summary, err := ReadStoredSummary(ctx, "")
	if err == nil {
		return summary
	}
	message := fmt.Sprintf("stored usage unavailable: %v", err)
	return UsageSummary{Agents: []AgentUsage{
		{AgentID: "claude", Name: "Claude Code", Installed: true, QuotaFetchError: message},
		{AgentID: "codex", Name: "OpenAI Codex", Installed: true, QuotaFetchError: message},
		{AgentID: "agy", Name: "Antigravity (AGY)", Installed: true, QuotaFetchError: message},
	}}
}
