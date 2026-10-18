package main

import (
	"context"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

// startTurnQuotaCapture overlaps the before-turn reading with the provider turn.
// The caller must defer stop, including on paths that never consume the reading:
// cancelling alone does not join the collector's cache and SQLite writes.
func startTurnQuotaCapture(ctx context.Context, capture func(context.Context, string, bool) usage.TurnQuotaReading, provider string) (<-chan usage.TurnQuotaReading, func()) {
	ctx, cancel := context.WithCancel(ctx)
	reading := make(chan usage.TurnQuotaReading, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		reading <- captureTurnQuota(ctx, capture, provider, false, time.Time{})
	}()
	return reading, func() {
		cancel()
		<-done
	}
}
