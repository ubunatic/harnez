package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

func TestRenderUsageTimelineIncludesQuotaHistory(t *testing.T) {
	home, data := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", data)
	dir := filepath.Join(data, "harnez", "usage-history")
	when := time.Date(2026, 9, 27, 10, 30, 0, 0, time.UTC)
	if err := usage.AppendQuotaHistory(dir, []usage.QuotaHistoryEntry{{
		Timestamp: when, Agent: "codex", Window: "5h", UsedPercent: 42, RemainingPercent: 58,
	}}); err != nil {
		t.Fatal(err)
	}
	text, err := renderUsageTimeline(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Quota Window History", "codex", "5h", "42% used"} {
		if !strings.Contains(text, want) {
			t.Errorf("text timeline missing %q: %s", want, text)
		}
	}
	encoded, err := renderUsageTimeline(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	var result usageTimelineJSON
	if err := json.Unmarshal([]byte(encoded), &result); err != nil {
		t.Fatalf("decode timeline JSON: %v", err)
	}
	if len(result.QuotaWindow) != 1 || result.QuotaWindow[0].UsedPercent != 42 {
		t.Fatalf("quota_windows = %+v, want one 42%% entry", result.QuotaWindow)
	}
	if _, err := os.Stat(usage.QuotaHistoryPath(dir)); err != nil {
		t.Fatalf("test quota fixture missing: %v", err)
	}
}
