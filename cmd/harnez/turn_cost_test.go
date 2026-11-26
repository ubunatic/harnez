package main

import (
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

// TestTurnAPICostPricesClaudePerCall checks that a Claude turn is priced per API call
// on each call's own prompt, that a call without a TTL split leaves the turn unknown,
// and that a non-Claude provider never picks up a Claude catalog entry (ticket 1301).
func TestTurnAPICostPricesClaudePerCall(t *testing.T) {
	claude := &subagent.Session{Provider: "claude", Model: "haiku"}
	// One call with a 100,001-token prompt: 1 uncached (tier 0.50) + 100,000 cache read (tier 0.05).
	ok := &subagent.TurnResult{RequestUsage: []subagent.RequestUsage{{Input: 1, CacheRead: 100_000, CacheWriteSplitKnown: true}}}
	if cost, rev, found := turnAPICost(claude, ok); !found || cost == nil || *cost != 5_001 || rev == "" {
		t.Fatalf("turnAPICost = %v, %q, %v; want 5001 micros with a revision", cost, rev, found)
	}
	unsplit := &subagent.TurnResult{RequestUsage: []subagent.RequestUsage{{Input: 1, CacheWriteTotal: 7}}}
	if cost, _, found := turnAPICost(claude, unsplit); found || cost != nil {
		t.Fatalf("call without a TTL split = %v, %v; want unknown (nil)", cost, found)
	}
	noCalls := &subagent.TurnResult{InputTokens: 10}
	if cost, _, found := turnAPICost(claude, noCalls); found || cost != nil {
		t.Fatalf("turn without per-call usage = %v, %v; want unknown (nil), not a flat estimate", cost, found)
	}
	agy := &subagent.Session{Provider: "agy", Model: "claude-sonnet-5-5"}
	if cost, _, found := turnAPICost(agy, ok); found || cost != nil {
		t.Fatalf("agy session priced with Claude rates = %v, %v; want nil", cost, found)
	}
}
