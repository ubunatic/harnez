package usage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usagestore"
)

type PassiveRequest struct {
	Windows []usagestore.Window
	Done    chan error
}

// RunCollector runs the background usage-collector loop backing `harnez
// server: it collects Claude/Codex/AGY usage live via CollectAllLive,
// persists normalized observations and history snapshots to SQLite, and
// atomically writes one JSON snapshot per agent for existing consumers.
// `harnez usage` reads the latest persisted SQLite snapshot without fetching.
//
// It collects once immediately, then again every interval, until ctx is
// cancelled (e.g. by a signal), at which point it returns nil.
//
// offline must be true only when client is nil *because* the caller passed
// `--offline` on purpose (skipping the live quota API), not merely whenever
// no client happens to be available — see PersistAgentSnapshot and issue
// 086 for why that distinction matters for what gets persisted.
func RunCollector(ctx context.Context, homeDir string, client *http.Client, interval time.Duration, out io.Writer, offline bool) error {
	return RunCollectorWithReady(ctx, homeDir, client, interval, out, offline, nil)
}

// RunCollectorWithReady signals readiness only after initialization and the
// first persisted collection cycle complete.
func RunCollectorWithReady(ctx context.Context, homeDir string, client *http.Client, interval time.Duration, out io.Writer, offline bool, ready func()) error {
	return RunCollectorWithFetchReady(ctx, homeDir, "", interval, out, offline, func(ctx context.Context) (UsageSummary, error) {
		return CollectAllLive(ctx, homeDir, client), nil
	}, ready)
}

// RunCollectorWithFetch is the testable collector loop; fetch is called once
// immediately and then at each interval, and every result is persisted by it.
func RunCollectorWithFetch(ctx context.Context, homeDir, dbPath string, interval time.Duration, out io.Writer, offline bool, fetch func(context.Context) (UsageSummary, error)) error {
	return RunCollectorWithFetchReady(ctx, homeDir, dbPath, interval, out, offline, fetch, nil)
}

func RunCollectorWithFetchReady(ctx context.Context, homeDir, dbPath string, interval time.Duration, out io.Writer, offline bool, fetch func(context.Context) (UsageSummary, error), ready func()) error {
	return runCollectorWithRequests(ctx, homeDir, dbPath, interval, out, offline, fetch, ready, nil)
}

func RunCollectorWithRequests(ctx context.Context, homeDir string, client *http.Client, interval time.Duration, out io.Writer, offline bool, requests <-chan PassiveRequest, ready func()) error {
	return RunCollectorWithRequestsAndTracker(ctx, homeDir, client, interval, out, offline, requests, ready, nil)
}

func RunCollectorWithRequestsAndTracker(ctx context.Context, homeDir string, client *http.Client, interval time.Duration, out io.Writer, offline bool, requests <-chan PassiveRequest, ready func(), tracker *ClientTracker) error {
	return runCollectorWithRequestsAndTracker(ctx, homeDir, "", interval, out, offline, func(ctx context.Context) (UsageSummary, error) {
		if catchup, _ := ctx.Value(catchupFetchKey{}).(bool); catchup {
			_, _, threshold, err := ClientHeartbeatPolicy()
			if err != nil {
				return UsageSummary{}, err
			}
			return CollectRegisteredCatchup(ctx, homeDir, client, threshold, nil, nil), nil
		}
		return CollectAllLive(ctx, homeDir, client), nil
	}, ready, requests, tracker)
}

type catchupFetchKey struct{}

func RunCollectorWithClientTracker(ctx context.Context, homeDir, dbPath string, interval time.Duration, out io.Writer, offline bool, fetch func(context.Context) (UsageSummary, error), ready func(), requests <-chan PassiveRequest, tracker *ClientTracker) error {
	return runCollectorWithRequestsAndTracker(ctx, homeDir, dbPath, interval, out, offline, fetch, ready, requests, tracker)
}

func runCollectorWithRequests(ctx context.Context, homeDir, dbPath string, interval time.Duration, out io.Writer, offline bool, fetch func(context.Context) (UsageSummary, error), ready func(), requests <-chan PassiveRequest) error {
	return runCollectorWithRequestsAndTracker(ctx, homeDir, dbPath, interval, out, offline, fetch, ready, requests, nil)
}

func runCollectorWithRequestsAndTracker(ctx context.Context, homeDir, dbPath string, interval time.Duration, out io.Writer, offline bool, fetch func(context.Context) (UsageSummary, error), ready func(), requests <-chan PassiveRequest, tracker *ClientTracker) error {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	if interval <= 0 {
		return fmt.Errorf("collector interval must be positive")
	}
	if ctx.Err() != nil {
		return nil
	}
	if dbPath == "" {
		var err error
		dbPath, err = telemetry.DefaultDBPath()
		if err != nil {
			return fmt.Errorf("resolve usage store path: %w", err)
		}
	}
	if err := InitializeUsageStore(ctx, homeDir, dbPath); err != nil {
		return fmt.Errorf("initialize usage store: %w", err)
	}
	stateDir := StateDir(homeDir)
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return fmt.Errorf("create state dir %s: %w", stateDir, err)
	}
	fmt.Fprintf(out, "harnez server: collecting usage to %s every %s\n", stateDir, interval)

	if requests == nil {
		requests = make(chan PassiveRequest)
	}
	fetchResults := make(chan fetchResult)
	writerDone := make(chan error, 1)
	var readyOnce sync.Once
	writerReady := func() {
		if ready != nil {
			readyOnce.Do(ready)
		}
	}
	go func() {
		writerDone <- runStoreWriter(ctx, homeDir, dbPath, stateDir, out, offline, requests, fetchResults, writerReady)
	}()
	triggerCatchup := make(chan struct{}, 1)
	if tracker != nil {
		tracker.SetOnFirstClient(func() {
			select {
			case triggerCatchup <- struct{}{}:
			default:
			}
		})
	}

	go func() {
		initial := true
		catchup := false
		for {
			if initial {
				initial = false
				// If a tracker is present and has no active clients at startup,
				// skip the immediate initial fetch per issue 714 M3.
				if tracker != nil && tracker.Count() == 0 {
					timer := time.NewTimer(interval)
					select {
					case <-ctx.Done():
						timer.Stop()
						return
					case <-triggerCatchup:
						timer.Stop()
						catchup = true
					case <-timer.C:
					}
				}
			}

			fetchCtx := context.WithValue(ctx, catchupFetchKey{}, catchup)
			summary, err := fetch(fetchCtx)
			catchup = false
			select {
			case fetchResults <- fetchResult{summary: summary, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-triggerCatchup:
				timer.Stop()
				catchup = true
			case <-timer.C:
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			<-writerDone
			return nil
		case err := <-writerDone:
			if err != nil {
				return err
			}
			return nil
		}
	}
}

type fetchResult struct {
	summary UsageSummary
	err     error
}

func runStoreWriter(ctx context.Context, homeDir, dbPath, stateDir string, out io.Writer, offline bool, requests <-chan PassiveRequest, results <-chan fetchResult, ready func()) error {
	// Ready once the writer runs, not after the first fetch: without clients the
	// first fetch waits for a heartbeat, which clients send only after ready.
	ready()
	for {
		select {
		case <-ctx.Done():
			return nil
		case request := <-requests:
			dedupe, err := PassiveDedupeInterval()
			if err == nil {
				err = persistPassive(ctx, dbPath, dedupe, request.Windows)
			}
			if err != nil {
				logPassiveWriteFailure(stateDir, err)
			}
			select {
			case request.Done <- err:
			case <-ctx.Done():
			}
		case result := <-results:
			if result.err != nil {
				return result.err
			}
			if err := persistUsage(ctx, homeDir, dbPath, stateDir, out, offline, result.summary); err != nil {
				return err
			}
		}
	}
}

func logPassiveWriteFailure(stateDir string, cause error) {
	message := fmt.Sprintf("harnez server: passive usage write failed: %v\n", cause)
	_, _ = io.WriteString(os.Stderr, message)
	file, err := os.OpenFile(filepath.Join(stateDir, "collector.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		_, _ = io.WriteString(file, message)
		_ = file.Close()
	}
}

func persistPassive(ctx context.Context, dbPath string, dedupe time.Duration, windows []usagestore.Window) error {
	store, err := usagestore.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	return store.WritePassive(ctx, dedupe, windows)
}

func collectTickWithFetch(ctx context.Context, homeDir, dbPath, stateDir string, out io.Writer, offline bool, fetch func(context.Context) (UsageSummary, error)) error {
	summary, err := fetch(ctx)
	if err != nil {
		return err
	}
	return persistUsage(ctx, homeDir, dbPath, stateDir, out, offline, summary)
}

func persistUsage(ctx context.Context, homeDir, dbPath, stateDir string, out io.Writer, offline bool, summary UsageSummary) error {
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
			fmt.Fprintf(out, "harnez server: write %s snapshot: %v\n", agent.AgentID, err)
		}
	}
}
