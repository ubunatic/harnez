package main

import (
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagestore"
)

// Ticket 1302 M1: a turn record names the model the calls reported, not the
// session alias, and the in-session share pairs across reset jitter.
func TestPopulateTurnCostRecordUsesReportedModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t0 := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	reset := t0.Add(72 * time.Hour)
	jittered := reset.Add(180 * time.Millisecond)
	before := usage.TurnQuotaReading{CapturedAt: t0, StoreWindows: []usagestore.Window{{Provider: "claude", Key: "weekly", UsedFraction: 0.20, ResetAt: &reset, ObservedAt: t0}}}
	after := usage.TurnQuotaReading{CapturedAt: t0.Add(30 * time.Second), StoreWindows: []usagestore.Window{{Provider: "claude", Key: "weekly", UsedFraction: 0.22, ResetAt: &jittered, ObservedAt: t0}}}
	sess := &subagent.Session{ID: "s1", Provider: "claude", Model: "opus", Tier: "high", Turn: 1}
	sess.TurnRecords = []subagent.TurnRecord{{Turn: 1}}
	r := &subagent.TurnResult{Model: "claude-opus-5-5"}
	rec := populateTurnCostRecord("", sess, 1, r, before, after)
	if rec == nil {
		t.Fatal("no turn record")
	}
	if rec.Model != "claude-opus-5-5" {
		t.Fatalf("turn record model = %q, want the reported id", rec.Model)
	}
	if rec.SubscriptionPriceShareMicros == nil {
		t.Fatalf("share nil for a 180ms reset jitter; the tolerance must pair it")
	}
	// Without a reported model the session alias is the fallback.
	sess.TurnRecords = []subagent.TurnRecord{{Turn: 1}}
	rec = populateTurnCostRecord("", sess, 1, &subagent.TurnResult{}, before, after)
	if rec.Model != "opus" {
		t.Fatalf("fallback model = %q, want the session alias", rec.Model)
	}
}
