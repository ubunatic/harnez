package usage

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usagestore"
)

// PersistUsageSummary applies collector-owned schema setup, compatibility
// imports, observations and history snapshots to the canonical usage store.
func PersistUsageSummary(ctx context.Context, homeDir, dbPath string, summary UsageSummary) error {
	if dbPath == "" {
		var err error
		dbPath, err = telemetry.DefaultDBPath()
		if err != nil {
			return err
		}
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	at := summary.Timestamp
	if at.IsZero() {
		at = time.Now().UTC()
	}
	summary.Timestamp = at
	for _, agent := range summary.Agents {
		observed := agent.LastRefreshed
		if observed.IsZero() {
			observed = at
		}
		if err := importAgent(store, ctx, agent.AgentID, "collector", observed, agent); err != nil {
			return err
		}
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown-host"
	}
	if err := store.WriteLatestUsageSummary(ctx, usagestore.UsageSummaryRecord{Hostname: host, ObservedAt: at, Payload: data}); err != nil {
		return err
	}
	return recordHistorySnapshot(ctx, store, summary, at)
}

// InitializeUsageStore performs collector-owned schema setup and compatibility
// imports once before the service accepts requests or starts its first tick.
func InitializeUsageStore(ctx context.Context, homeDir, dbPath string) error {
	if dbPath == "" {
		var err error
		dbPath, err = telemetry.DefaultDBPath()
		if err != nil {
			return err
		}
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.EnsureUsageSchema(ctx); err != nil {
		return err
	}
	if err := archiveLegacySourcesBeforeImport(ctx, store); err != nil {
		return err
	}
	if err := store.MigrateStableWindowKeys(ctx); err != nil {
		return err
	}
	return importCompatibility(store, ctx, homeDir)
}
