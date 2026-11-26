package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usagestore"
)

// Each call is priced by its own model, not by the session model (ticket 1301 M3).
func TestTurnAPICostPricesEachCallByItsOwnModel(t *testing.T) {
	sess := &subagent.Session{Provider: "claude", Model: "sonnet"}
	// 10 uncached tokens: Haiku 0.10 USD/MTok = 1 micro, Opus 4 USD/MTok = 40 micros.
	r := &subagent.TurnResult{RequestUsage: []subagent.RequestUsage{
		{Model: "claude-haiku-5-5", Input: 10, CacheWriteSplitKnown: true},
		{Model: "claude-opus-5-5", Input: 10, CacheWriteSplitKnown: true},
	}}
	if cost, rev, found := turnAPICost(sess, r); !found || cost == nil || *cost != 41 || rev == "" {
		t.Fatalf("mixed-model turn = %v, %q, %v; want 41 micros (1 Haiku + 40 Opus)", cost, rev, found)
	}
	// A dated model id from the stream prices as its catalog entry.
	dated := &subagent.TurnResult{RequestUsage: []subagent.RequestUsage{{Model: "claude-haiku-5-5-20261101", Input: 10, CacheWriteSplitKnown: true}}}
	if cost, _, found := turnAPICost(sess, dated); !found || cost == nil || *cost != 1 {
		t.Fatalf("dated haiku id = %v, %v; want 1 micro from the haiku entry", cost, found)
	}
}

// A call whose model has no sourced rate, or no model at all, makes the turn unknown.
func TestTurnAPICostUnknownCallModelMakesTurnNil(t *testing.T) {
	sess := &subagent.Session{Provider: "claude", Model: "haiku"}
	for name, model := range map[string]string{"no sourced rate": "claude-fable-5-2", "no model": ""} {
		r := &subagent.TurnResult{RequestUsage: []subagent.RequestUsage{
			{Model: "claude-haiku-5-5", Input: 10, CacheWriteSplitKnown: true},
			{Model: model, Input: 10, CacheWriteSplitKnown: true},
		}}
		if cost, _, found := turnAPICost(sess, r); found || cost != nil {
			t.Fatalf("%s: turn = %v, %v; want unknown (nil), not a partial sum", name, cost, found)
		}
	}
}

// The cumulative row stores cache reads only, like the delta row, and records
// the cache-write total so the two rows agree (ticket 1301 M3).
func TestCumulativeCacheRowExcludesCacheWrites(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	sess := &subagent.Session{
		ID: "s-cum", Provider: "claude", Model: "haiku", Turn: 1, TokenTotalsKnown: true,
		InputTokensTotal: 1, CachedTokensTotal: 130, OutputTokensTotal: 5,
		CacheWriteTokensTotal: 30, CacheWriteTokensKnown: true,
		TurnRecords: []subagent.TurnRecord{{Turn: 1}},
	}
	// CachedTokens includes the 30 cache-write tokens for Claude; the reads are 100.
	result := &subagent.TurnResult{InputTokens: 1, CachedTokens: 130, CacheWriteTokens: 30, CacheWriteTokensKnown: true, OutputTokens: 5}
	persistAgentTurnTokens(dbPath, sess, result, time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC))

	store, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rows, err := store.TurnTokenUsages(context.Background(), "s-cum")
	if err != nil {
		t.Fatal(err)
	}
	var cumulative, delta *usagestore.TurnTokenUsage
	for i := range rows {
		switch rows[i].CounterKind {
		case "cumulative":
			cumulative = &rows[i]
		case "delta":
			delta = &rows[i]
		}
	}
	if cumulative == nil || delta == nil {
		t.Fatalf("rows = %+v; want one cumulative and one delta row", rows)
	}
	if cumulative.CachedInputTokens == nil || *cumulative.CachedInputTokens != 100 {
		t.Fatalf("cumulative cached = %v; want 100 cache reads without the 30 writes", cumulative.CachedInputTokens)
	}
	if cumulative.CacheWriteTokens == nil || *cumulative.CacheWriteTokens != 30 {
		t.Fatalf("cumulative cache writes = %v; want 30 so cumulative and delta rows agree", cumulative.CacheWriteTokens)
	}
	if delta.CachedInputTokens == nil || *delta.CachedInputTokens != 100 {
		t.Fatalf("delta cached = %v; want 100", delta.CachedInputTokens)
	}
}
