package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/usage"
)

// warnModelQuota reuses the cached model-availability reader. It never probes
// the provider or waits for the concurrent before-turn capture.
func warnModelQuota(cmd *cobra.Command, availability func(string, string) usage.ProviderQuotaAvailability, provider, model string, now time.Time) {
	if availability == nil {
		availability = providerAvailability
	}
	if line := usage.QuotaNotice(provider, availability(provider, model), now); line != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), line)
	}
}

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
