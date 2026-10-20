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
//
// offline must be true only when client is nil *because* the caller passed
// `--offline` on purpose (skipping the live quota API), not merely whenever
// no client happens to be available — see PersistAgentSnapshot and issue
// 086 for why that distinction matters for what gets persisted.
func RunCollector(ctx context.Context, homeDir string, client *http.Client, interval time.Duration, out io.Writer, offline bool) error {
	return RunCollectorWithFetch(ctx, homeDir, "", interval, out, offline, func(ctx context.Context) (UsageSummary, error) {
		return CollectAllLive(ctx, homeDir, client), nil
	})
}

// RunCollectorWithFetch is the testable collector loop; fetch is called once
// immediately and then at each interval, and every result is persisted by it.
func RunCollectorWithFetch(ctx context.Context, homeDir, dbPath string, interval time.Duration, out io.Writer, offline bool, fetch func(context.Context) (UsageSummary, error)) error {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	if interval <= 0 {
		return fmt.Errorf("collector interval must be positive")
	}
	if ctx.Err() != nil {
		return nil
	}
	if err := InitializeUsageStore(ctx, homeDir, dbPath); err != nil {
		return fmt.Errorf("initialize usage store: %w", err)
	}
	stateDir := StateDir(homeDir)
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return fmt.Errorf("create state dir %s: %w", stateDir, err)
	}
	fmt.Fprintf(out, "harnez agent-collector: writing snapshots to %s every %s\n", stateDir, interval)

	if err := collectTickWithFetch(ctx, homeDir, dbPath, stateDir, out, offline, fetch); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := collectTickWithFetch(ctx, homeDir, dbPath, stateDir, out, offline, fetch); err != nil {
				return err
			}
		}
	}
}

func collectTickWithFetch(ctx context.Context, homeDir, dbPath, stateDir string, out io.Writer, offline bool, fetch func(context.Context) (UsageSummary, error)) error {
	summary, err := fetch(ctx)
	if err != nil {
		return err
	}
	if err := PersistUsageSummary(ctx, homeDir, dbPath, summary); err != nil {
		return err
	}
	for _, agent := range summary.Agents {
		if err := PersistAgentSnapshot(stateDir, agent, offline); err != nil {
			fmt.Fprintf(out, "harnez server: write %s snapshot: %v\n", agent.AgentID, err)
		}
	}
	return nil
}

// collectTick runs one live collection pass and persists each agent's
// result, logging (but not aborting the loop on) individual write failures
// — one agent's disk error shouldn't stop the others from being refreshed.
// See RunCollector's doc comment for what offline must mean.
func collectTick(ctx context.Context, homeDir string, client *http.Client, stateDir string, out io.Writer, offline bool) {
	summary := CollectAllLive(ctx, homeDir, client)
	for _, agent := range summary.Agents {
		if err := PersistAgentSnapshot(stateDir, agent, offline); err != nil {
			fmt.Fprintf(out, "harnez agent-collector: write %s snapshot: %v\n", agent.AgentID, err)
		}
	}
}
