package usage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// RunCollector runs the background usage-collector loop backing `harnez
// agent-collector` (issue 082): it collects Claude/Codex/AGY usage live via
// CollectAllLive and atomically writes one JSON snapshot per agent to the
// shared state directory (see StateDir and WriteAgentSnapshot), so
// `harnez usage` / the TUI can read a warm cache (CollectAll) instead of
// paying the live-collect cost inline on every invocation.
//
// It collects once immediately, then again every interval, until ctx is
// cancelled (e.g. by a signal), at which point it returns nil.
func RunCollector(ctx context.Context, homeDir string, client *http.Client, interval time.Duration, out io.Writer) error {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	stateDir := StateDir(homeDir)
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return fmt.Errorf("create state dir %s: %w", stateDir, err)
	}
	fmt.Fprintf(out, "harnez agent-collector: writing snapshots to %s every %s\n", stateDir, interval)

	collectTick(ctx, homeDir, client, stateDir, out)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			collectTick(ctx, homeDir, client, stateDir, out)
		}
	}
}

// collectTick runs one live collection pass and persists each agent's
// result, logging (but not aborting the loop on) individual write failures
// — one agent's disk error shouldn't stop the others from being refreshed.
func collectTick(ctx context.Context, homeDir string, client *http.Client, stateDir string, out io.Writer) {
	summary := CollectAllLive(ctx, homeDir, client)
	for _, agent := range summary.Agents {
		if err := PersistAgentSnapshot(stateDir, agent); err != nil {
			fmt.Fprintf(out, "harnez agent-collector: write %s snapshot: %v\n", agent.AgentID, err)
		}
	}
}
