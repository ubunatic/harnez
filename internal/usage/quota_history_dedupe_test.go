package usage

import (
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usagestore"
)

// The same reading can be stored under two sources (registry and collector).
// The legacy JSONL history has no source column, so each reading must appear once.
func TestDedupeQuotaHistoryKeepsOneWindowPerReadingAcrossSources(t *testing.T) {
	at := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	windows := []usagestore.Window{
		{Provider: "claude", Key: "weekly", Name: "Weekly", Source: "registry", UsedFraction: 0.2, ObservedAt: at},
		{Provider: "claude", Key: "weekly", Name: "Weekly", Source: "collector", UsedFraction: 0.2, ObservedAt: at},
		{Provider: "claude", Key: "weekly", Name: "Weekly", Source: "collector", UsedFraction: 0.21, ObservedAt: at.Add(time.Minute)},
		{Provider: "codex", Key: "weekly", Name: "Weekly", Source: "registry", UsedFraction: 0.2, ObservedAt: at},
	}
	got := dedupeQuotaHistoryWindows(windows)
	if len(got) != 3 {
		t.Fatalf("dedupe kept %d windows; want 3 (one per provider, key and time): %+v", len(got), got)
	}
	if got[0].Source != "registry" {
		t.Fatalf("first window source = %q; want the first stored row (registry) to win", got[0].Source)
	}
	if got[1].Provider != "claude" || got[1].ObservedAt != at.Add(time.Minute) {
		t.Fatalf("second window = %+v; want the later claude reading kept", got[1])
	}
}
