package usagestore

import (
	"context"
	"testing"
	"time"
)

// TestCompactBoundaryStoresTurnZeroOnly: a compaction sample is stored with turn 0
// and never with a turn, and before/after rows still need turn 1 or more (issue
// 1303 M2).
func TestCompactBoundaryStoresTurnZeroOnly(t *testing.T) {
	s := openTurnPairStore(t)
	ctx := context.Background()
	at := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	if _, err := s.WriteTurnQuotaBoundary(ctx, TurnQuotaBoundary{SessionID: "s1", Turn: 0, Boundary: "compact", Provider: "claude", CapturedAt: at, Source: "compact-fetch", Freshness: "fresh", HasCache: true, ImportKey: "compact-record:r1"}); err != nil {
		t.Fatalf("compact sample rejected: %v", err)
	}
	if _, err := s.WriteTurnQuotaBoundary(ctx, TurnQuotaBoundary{SessionID: "s1", Turn: 1, Boundary: "compact", Provider: "claude", CapturedAt: at}); err == nil {
		t.Fatal("compact sample with a turn was accepted")
	}
	if _, err := s.WriteTurnQuotaBoundary(ctx, TurnQuotaBoundary{SessionID: "s1", Turn: 0, Boundary: "before", Provider: "claude", CapturedAt: at}); err == nil {
		t.Fatal("before sample at turn 0 was accepted")
	}
}
