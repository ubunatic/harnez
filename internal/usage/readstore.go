package usage

import (
	"context"
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
	records, err := store.UsageSummaries(ctx)
	if err != nil {
		return UsageSummary{}, err
	}
	if len(records) == 0 {
		return UsageSummary{}, fmt.Errorf("usage store has no collector snapshots yet")
	}
	var summary UsageSummary
	if err := json.Unmarshal(records[len(records)-1].Payload, &summary); err != nil {
		return UsageSummary{}, fmt.Errorf("decode usage snapshot: %w", err)
	}
	return summary, nil
}
