package usage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQuotaHistorySchemaAndSerialization(t *testing.T) {
	resetAt, _ := time.Parse(time.RFC3339, "2026-09-23T09:22:53Z")
	entry := QuotaHistoryEntry{
		Timestamp:        time.Date(2026, 9, 16, 20, 37, 14, 0, time.UTC),
		Agent:            "agy",
		Group:            "Gemini Models",
		Window:           "Weekly Limit Remaining",
		UsedPercent:      17,
		RemainingPercent: 83,
		ResetAt:          &resetAt,
		DurationLeftMS:   571538071,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if raw["timestamp"] != "2026-09-16T20:37:14Z" {
		t.Errorf("timestamp = %v, want 2026-09-16T20:37:14Z", raw["timestamp"])
	}
	if raw["agent"] != "agy" {
		t.Errorf("agent = %v, want agy", raw["agent"])
	}
	if raw["group"] != "Gemini Models" {
		t.Errorf("group = %v, want Gemini Models", raw["group"])
	}
	if raw["window"] != "Weekly Limit Remaining" {
		t.Errorf("window = %v, want Weekly Limit Remaining", raw["window"])
	}
	if raw["used_percent"] != float64(17) {
		t.Errorf("used_percent = %v, want 17", raw["used_percent"])
	}
	if raw["remaining_percent"] != float64(83) {
		t.Errorf("remaining_percent = %v, want 83", raw["remaining_percent"])
	}
	if raw["reset_at"] != "2026-09-23T09:22:53Z" {
		t.Errorf("reset_at = %v, want 2026-09-23T09:22:53Z", raw["reset_at"])
	}
	if raw["duration_left_ms"] != float64(571538071) {
		t.Errorf("duration_left_ms = %v, want 571538071", raw["duration_left_ms"])
	}
}

func TestQuotaHistoryOmitEmptyGroup(t *testing.T) {
	entry := QuotaHistoryEntry{
		Timestamp:        time.Date(2026, 9, 16, 20, 0, 0, 0, time.UTC),
		Agent:            "claude",
		Window:           "Session (5-hour)",
		UsedPercent:      25,
		RemainingPercent: 75,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if _, ok := raw["group"]; ok {
		t.Errorf("group key should be omitted when empty: %s", string(data))
	}
	if _, ok := raw["reset_at"]; ok {
		t.Errorf("reset_at key should be omitted when nil: %s", string(data))
	}
	if _, ok := raw["duration_left_ms"]; ok {
		t.Errorf("duration_left_ms key should be omitted when 0: %s", string(data))
	}
}

func TestAppendQuotaHistoryDeduplicationAndThrottling(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	reset1 := t0.Add(5 * time.Hour)
	reset2 := t0.Add(6 * time.Hour)

	// 1. Initial snapshot
	entry1 := QuotaHistoryEntry{
		Timestamp:        t0,
		Agent:            "claude",
		Window:           "Session (5-hour)",
		UsedPercent:      20,
		RemainingPercent: 80,
		ResetAt:          &reset1,
		DurationLeftMS:   (5 * time.Hour).Milliseconds(),
	}
	if err := AppendQuotaHistoryWithThrottle(dir, []QuotaHistoryEntry{entry1}, 15*time.Minute, t0); err != nil {
		t.Fatalf("initial append: %v", err)
	}

	entries, err := ReadQuotaHistory(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d (err: %v)", len(entries), err)
	}

	// 2. Duplicate snapshot within throttle window (5 seconds later) -> Should be dropped
	t1 := t0.Add(5 * time.Second)
	entry2 := QuotaHistoryEntry{
		Timestamp:        t1,
		Agent:            "claude",
		Window:           "Session (5-hour)",
		UsedPercent:      20,
		RemainingPercent: 80,
		ResetAt:          &reset1,
		DurationLeftMS:   (5*time.Hour - 5*time.Second).Milliseconds(),
	}
	if err := AppendQuotaHistoryWithThrottle(dir, []QuotaHistoryEntry{entry2}, 15*time.Minute, t1); err != nil {
		t.Fatalf("duplicate append: %v", err)
	}

	entries, _ = ReadQuotaHistory(dir)
	if len(entries) != 1 {
		t.Fatalf("expected duplicate to be skipped, got %d entries", len(entries))
	}

	// 3. Percentage change within throttle window (2 minutes later) -> Should append
	t2 := t0.Add(2 * time.Minute)
	entry3 := QuotaHistoryEntry{
		Timestamp:        t2,
		Agent:            "claude",
		Window:           "Session (5-hour)",
		UsedPercent:      21,
		RemainingPercent: 79,
		ResetAt:          &reset1,
		DurationLeftMS:   (5*time.Hour - 2*time.Minute).Milliseconds(),
	}
	if err := AppendQuotaHistoryWithThrottle(dir, []QuotaHistoryEntry{entry3}, 15*time.Minute, t2); err != nil {
		t.Fatalf("percentage change append: %v", err)
	}

	entries, _ = ReadQuotaHistory(dir)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after percentage change, got %d", len(entries))
	}
	if entries[1].UsedPercent != 21 {
		t.Errorf("entry[1].UsedPercent = %d, want 21", entries[1].UsedPercent)
	}

	// 4. ResetAt change within throttle window (3 minutes later) -> Should append
	t3 := t0.Add(3 * time.Minute)
	entry4 := QuotaHistoryEntry{
		Timestamp:        t3,
		Agent:            "claude",
		Window:           "Session (5-hour)",
		UsedPercent:      21,
		RemainingPercent: 79,
		ResetAt:          &reset2,
		DurationLeftMS:   (6 * time.Hour).Milliseconds(),
	}
	if err := AppendQuotaHistoryWithThrottle(dir, []QuotaHistoryEntry{entry4}, 15*time.Minute, t3); err != nil {
		t.Fatalf("reset change append: %v", err)
	}

	entries, _ = ReadQuotaHistory(dir)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries after reset time change, got %d", len(entries))
	}

	// 5. Unchanged snapshot after throttle duration elapses (16 minutes after t3) -> Should append
	t4 := t3.Add(16 * time.Minute)
	entry5 := QuotaHistoryEntry{
		Timestamp:        t4,
		Agent:            "claude",
		Window:           "Session (5-hour)",
		UsedPercent:      21,
		RemainingPercent: 79,
		ResetAt:          &reset2,
		DurationLeftMS:   (6*time.Hour - 16*time.Minute).Milliseconds(),
	}
	if err := AppendQuotaHistoryWithThrottle(dir, []QuotaHistoryEntry{entry5}, 15*time.Minute, t4); err != nil {
		t.Fatalf("throttle elapsed append: %v", err)
	}

	entries, _ = ReadQuotaHistory(dir)
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries after throttle expiry, got %d", len(entries))
	}
}

func TestAgentUsageToQuotaHistoryEntries(t *testing.T) {
	now := time.Date(2026, 9, 16, 15, 0, 0, 0, time.UTC)
	reset := now.Add(2 * time.Hour)

	usage := AgentUsage{
		AgentID: "agy",
		ModelGroups: []ModelGroup{
			{
				Name: "Gemini Models",
				Windows: []QuotaWindow{
					{
						Name:             "Weekly Limit Remaining",
						UsedPercent:      15.0,
						RemainingPercent: 85.0,
						ResetAt:          &reset,
					},
					{
						Name:             "Five Hour Limit Remaining",
						UsedPercent:      10.0,
						RemainingPercent: 90.0,
						ResetAt:          &reset,
					},
				},
			},
		},
	}

	entries := AgentUsageToQuotaHistoryEntries(usage, now)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	if entries[0].Group != "Gemini Models" || entries[0].Window != "Weekly Limit Remaining" || entries[0].UsedPercent != 15 {
		t.Errorf("unexpected entry 0: %+v", entries[0])
	}
	if entries[1].Group != "Gemini Models" || entries[1].Window != "Five Hour Limit Remaining" || entries[1].RemainingPercent != 90 {
		t.Errorf("unexpected entry 1: %+v", entries[1])
	}
}

func TestCollectClaudeAppendsQuotaHistory(t *testing.T) {
	dir := claudeFixtureDir(t)
	now := time.Now()
	resetsAt := now.Add(4 * time.Hour).Format(time.RFC3339Nano)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ClaudeOauthUsageResponse{
			FiveHour: &struct {
				Utilization float64 `json:"utilization"`
				ResetsAt    string  `json:"resets_at"`
			}{
				Utilization: 42.0,
				ResetsAt:    resetsAt,
			},
			SevenDay: &struct {
				Utilization float64 `json:"utilization"`
				ResetsAt    string  `json:"resets_at"`
			}{
				Utilization: 10.0,
				ResetsAt:    resetsAt,
			},
		})
	}))
	defer mockServer.Close()

	usage := collectClaudeAgainstURL(t, dir, mockServer)
	if usage.QuotaFetchError != "" {
		t.Fatalf("CollectClaude error: %v", usage.QuotaFetchError)
	}

	histDir := resolveQuotaHistoryDir(dir)
	entries, err := ReadQuotaHistory(histDir)
	if err != nil {
		t.Fatalf("ReadQuotaHistory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 quota history entries, got %d", len(entries))
	}

	if entries[0].Agent != "claude" || entries[0].Window != "Session (5-hour)" || entries[0].UsedPercent != 42 {
		t.Errorf("unexpected session entry: %+v", entries[0])
	}
	if entries[1].Agent != "claude" || entries[1].Window != "Weekly (7-day)" || entries[1].UsedPercent != 10 {
		t.Errorf("unexpected weekly entry: %+v", entries[1])
	}
}

func TestReadHistoryIgnoresQuotaHistoryFile(t *testing.T) {
	dir := t.TempDir()

	// Write a standard history entry
	summary := UsageSummary{
		Timestamp: time.Now(),
		Agents: []AgentUsage{
			{
				AgentID:       "claude",
				Installed:     true,
				Authenticated: true,
				Tokens:        &TokenBreakdown{TotalTokens: 1000},
			},
		},
	}
	if err := AppendHistory(dir, summary); err != nil {
		t.Fatalf("AppendHistory: %v", err)
	}

	// Write quota-history.jsonl in the same directory
	quotaEntry := QuotaHistoryEntry{
		Timestamp:        time.Now(),
		Agent:            "claude",
		Window:           "Session (5-hour)",
		UsedPercent:      50,
		RemainingPercent: 50,
	}
	if err := AppendQuotaHistory(dir, []QuotaHistoryEntry{quotaEntry}); err != nil {
		t.Fatalf("AppendQuotaHistory: %v", err)
	}

	// ReadHistory should only return the 1 UsageSummary host entry
	histEntries, err := ReadHistory(dir)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}
	if len(histEntries) != 1 {
		t.Fatalf("ReadHistory returned %d entries, want 1", len(histEntries))
	}

	// HistorySummaryStats should only count 1 file (host jsonl, ignoring quota-history.jsonl)
	stats, err := HistorySummaryStats(dir)
	if err != nil {
		t.Fatalf("HistorySummaryStats: %v", err)
	}
	if stats.FileCount != 1 {
		t.Errorf("HistorySummaryStats FileCount = %d, want 1", stats.FileCount)
	}
	if stats.TotalEntries != 1 {
		t.Errorf("HistorySummaryStats TotalEntries = %d, want 1", stats.TotalEntries)
	}
}

func TestCollectCodexAppendsQuotaHistory(t *testing.T) {
	dir := codexFixtureDir(t)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		codexWhamOKHandler(w, r)
	}))
	defer mockServer.Close()

	usage := collectCodexAgainstURL(t, dir, mockServer)
	if usage.QuotaFetchError != "" {
		t.Fatalf("CollectCodex error: %v", usage.QuotaFetchError)
	}

	histDir := resolveQuotaHistoryDir(dir)
	entries, err := ReadQuotaHistory(histDir)
	if err != nil {
		t.Fatalf("ReadQuotaHistory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 quota history entry, got %d", len(entries))
	}
	if entries[0].Agent != "codex" || entries[0].UsedPercent != 40 {
		t.Errorf("unexpected codex entry: %+v", entries[0])
	}
}

func TestCollectAGYAppendsQuotaHistory(t *testing.T) {
	_, cleanup := agyStubUsageCmd(t, []byte(agyOKOutput), nil)
	defer cleanup()

	dir := t.TempDir()
	usage := CollectAGY(context.Background(), dir, http.DefaultClient)
	if usage.QuotaFetchError != "" {
		t.Fatalf("CollectAGY error: %v", usage.QuotaFetchError)
	}

	histDir := resolveQuotaHistoryDir(dir)
	entries, err := ReadQuotaHistory(histDir)
	if err != nil {
		t.Fatalf("ReadQuotaHistory: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 quota history entries for AGY model groups, got %d", len(entries))
	}

	for _, e := range entries {
		if e.Agent != "agy" {
			t.Errorf("entry agent = %q, want agy", e.Agent)
		}
		if e.Group == "" {
			t.Errorf("entry group empty, want model group: %+v", e)
		}
	}
}

