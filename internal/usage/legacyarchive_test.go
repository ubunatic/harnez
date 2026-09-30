package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usagestore"
)

func TestLegacyUsageArchiveCopiesAndImportsWithManifestParity(t *testing.T) {
	base := t.TempDir()
	paths := LegacyUsageArchivePaths{
		Home: filepath.Join(base, "home"), DataHome: filepath.Join(base, "data"),
		CacheHome: filepath.Join(base, "cache"), StateHome: filepath.Join(base, "state"),
		ArchiveBase: filepath.Join(base, "archives"),
	}
	at := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	cachePath := filepath.Join(paths.CacheHome, "harnez", "quota-cache-claude.json")
	cache := liveFetchCache[claudeQuotaPayload]{FetchedAt: at, Payload: claudeQuotaPayload{Session: &QuotaWindow{Name: "5h", UsedPercent: 37.5, RemainingPercent: 62.5}}}
	writeJSON(t, cachePath, cache)
	snapshotPath := filepath.Join(paths.StateHome, "harnez", "agents", "usage", "codex.json")
	writeJSON(t, snapshotPath, AgentSnapshot{FetchedAt: at, Usage: AgentUsage{AgentID: "codex", Weekly: &QuotaWindow{Name: "weekly", UsedPercent: 21, RemainingPercent: 79}}})
	historyPath := filepath.Join(paths.DataHome, "harnez", "usage-history", QuotaHistoryFilename)
	writeJSONLine(t, historyPath, QuotaHistoryEntry{Timestamp: at.Add(time.Minute), Agent: "agy", Window: "Weekly Limit Remaining", UsedPercent: 44, RemainingPercent: 56})
	hostHistoryPath := filepath.Join(paths.DataHome, "harnez", "usage-history", "host.jsonl")
	writeJSONLine(t, hostHistoryPath, HistoryEntry{Hostname: "host", UsageSummary: UsageSummary{Timestamp: at.Add(2 * time.Minute), Agents: []AgentUsage{{AgentID: "codex", Session: &QuotaWindow{Name: "5-hour", UsedPercent: 22, RemainingPercent: 78}}}}})
	turnPath := filepath.Join(paths.Home, ".harnez", "agents", "session-a", "quota-readings.jsonl")
	writeJSONLine(t, turnPath, map[string]any{"session_id": "session-a", "turn": 1, "boundary": "before", "provider": "claude", "reading": TurnQuotaReading{CapturedAt: at, Source: "legacy-test", HasCache: true, StoreWindows: structStoreWindowFixture("claude", "5h", at)}})

	result, err := ArchiveLegacyUsageData(paths)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 5 {
		t.Fatalf("archived %d files, want 5: %s", result.Files, result.Path)
	}
	for _, source := range []string{cachePath, snapshotPath, historyPath, hostHistoryPath, turnPath} {
		if _, err := os.Stat(source); err != nil {
			t.Fatalf("archive changed source %s: %v", source, err)
		}
	}
	manifestData, err := os.ReadFile(filepath.Join(result.Path, legacyUsageArchiveManifest))
	if err != nil {
		t.Fatal(err)
	}
	var manifest LegacyUsageArchiveManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.ImportCommand != "harnez usage archive import <archive-directory>" || len(manifest.Files) != result.Files {
		t.Fatalf("manifest = %+v", manifest)
	}
	dbPath := filepath.Join(base, "telemetry.sqlite")
	imported, err := ImportLegacyUsageArchive(context.Background(), dbPath, result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if imported.Files != result.Files {
		t.Fatalf("import result = %+v", imported)
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := usagestore.EnsureSchema(context.Background(), func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.QueryRow(context.Background(), `SELECT count(*) FROM usage_observations WHERE source IN ('provider-cache','state-snapshot','history','legacy-test')`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("imported observation count = %d, want 5", count)
	}
	if _, err := ImportLegacyUsageArchive(context.Background(), dbPath, result.Path); err != nil {
		t.Fatal(err)
	}
	if err := store.QueryRow(context.Background(), `SELECT count(*) FROM usage_observations WHERE source IN ('provider-cache','state-snapshot','history','legacy-test')`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("reimport changed observation count to %d", count)
	}
	first := manifest.Files[0]
	if err := os.WriteFile(filepath.Join(result.Path, first.ArchivePath), []byte(strings.Repeat("x", int(first.Size))), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportLegacyUsageArchive(context.Background(), filepath.Join(base, "tamper.sqlite"), result.Path); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("tampered archive import error = %v", err)
	}
}

func structStoreWindowFixture(provider, name string, at time.Time) []usagestore.Window {
	return []usagestore.Window{{Provider: provider, Key: name, Name: name, Source: "legacy-test", Freshness: "fresh", UsedFraction: .31, ObservedAt: at}}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeJSONLine(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
