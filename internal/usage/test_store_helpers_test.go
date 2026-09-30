package usage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usagestore"
)

func isolateUsageTestStorage(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	for key, name := range map[string]string{
		"HOME": "home", "XDG_DATA_HOME": "data", "XDG_CACHE_HOME": "cache",
		"XDG_STATE_HOME": "state", "XDG_CONFIG_HOME": "config",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(key, path)
	}
}

func seedProviderSnapshot(t *testing.T, provider string, at time.Time, usage AgentUsage) {
	t.Helper()
	dbPath, err := telemetry.DefaultDBPath()
	if err != nil {
		t.Fatal(err)
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		t.Fatal(err)
	}
	if err := store.MigrateStableWindowKeys(ctx); err != nil {
		t.Fatal(err)
	}
	usage.AgentID = provider
	if err := store.WriteCurrent(ctx, at, AgentUsageToStoreWindows(usage, at)); err != nil {
		t.Fatal(err)
	}
}
