package usage

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usagestore"
)

// Collector describes one independently scheduled usage source.
type Collector struct {
	ID           string
	Provider     string
	Capabilities []string
	Cadence      time.Duration
	Timeout      time.Duration
	Collect      func(context.Context) (AgentUsage, error)
	CollectLoad  func(context.Context) (*LoadSnapshot, error)
}

// CollectorDiagnostic records one collector run, including failures and duration.
type CollectorDiagnostic struct {
	ID       string
	Started  time.Time
	Duration time.Duration
	Error    string
}

// Registry runs registered collectors independently and retains their last good values.
type Registry struct {
	collectors []Collector
	mu         sync.Mutex
	lastGood   map[string]AgentUsage
}

func NewRegistry(collectors ...Collector) (*Registry, error) {
	seen := map[string]bool{}
	for _, c := range collectors {
		if c.ID == "" || c.Provider == "" || (c.Collect == nil && c.CollectLoad == nil) || c.Cadence <= 0 || c.Timeout <= 0 {
			return nil, fmt.Errorf("invalid usage collector %q", c.ID)
		}
		if seen[c.ID] {
			return nil, fmt.Errorf("duplicate usage collector %q", c.ID)
		}
		seen[c.ID] = true
	}
	return &Registry{collectors: append([]Collector(nil), collectors...), lastGood: map[string]AgentUsage{}}, nil
}

// CollectLoad runs one registered load collector with its declared timeout.
func (r *Registry) CollectLoad(ctx context.Context, id string, diagnostics ...func(CollectorDiagnostic)) (*LoadSnapshot, error) {
	for _, collector := range r.collectors {
		if collector.ID != id || collector.CollectLoad == nil {
			continue
		}
		started := time.Now()
		cctx, cancel := context.WithTimeout(ctx, collector.Timeout)
		snapshot, err := collector.CollectLoad(cctx)
		cancel()
		d := CollectorDiagnostic{ID: collector.ID, Started: started, Duration: time.Since(started)}
		if err != nil {
			d.Error = err.Error()
		} else if snapshot != nil {
			writeCtx, writeCancel := context.WithTimeout(ctx, 100*time.Millisecond)
			writeErr := writeLoadObservation(writeCtx, collector.Provider, d.Started, snapshot)
			writeCancel()
			if writeErr != nil {
				d.Error = writeErr.Error()
			}
		}
		if len(diagnostics) > 0 && diagnostics[0] != nil {
			diagnostics[0](d)
		}
		return snapshot, err
	}
	return nil, fmt.Errorf("load collector %q is not registered", id)
}

func writeLoadObservation(ctx context.Context, provider string, observedAt time.Time, snapshot *LoadSnapshot) error {
	dbPath, err := telemetry.DefaultDBPath()
	if err != nil {
		return err
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		return err
	}
	return store.WriteLoadObservation(ctx, provider, observedAt, snapshot)
}

func (r *Registry) Collect(ctx context.Context, progress FetchProgressFunc, diagnostic func(CollectorDiagnostic)) []AgentUsage {
	if r == nil {
		return nil
	}
	type result struct {
		collector Collector
		usage     AgentUsage
		err       error
		started   time.Time
		duration  time.Duration
	}
	results := make(chan result, len(r.collectors))
	var wg sync.WaitGroup
	for _, collector := range r.collectors {
		collector := collector
		wg.Add(1)
		go func() {
			defer wg.Done()
			started := time.Now()
			if progress != nil {
				progress(collector.ID, FetchStarted)
			}
			cctx, cancel := context.WithTimeout(ctx, collector.Timeout)
			defer cancel()
			u, err := collector.Collect(cctx)
			if err == nil && u.QuotaFetchError != "" {
				err = fmt.Errorf("%s", u.QuotaFetchError)
			}
			results <- result{collector: collector, usage: u, err: err, started: started, duration: time.Since(started)}
		}()
	}
	wg.Wait()
	close(results)
	byID := make(map[string]AgentUsage, len(r.collectors))
	for item := range results {
		u := item.usage
		if u.AgentID == "" {
			u.AgentID = item.collector.Provider
		}
		if u.LastRefreshed.IsZero() {
			u.LastRefreshed = item.started.Add(item.duration)
		}
		if item.err == nil {
			r.mu.Lock()
			r.lastGood[item.collector.ID] = u
			r.mu.Unlock()
		} else {
			r.mu.Lock()
			previous, ok := r.lastGood[item.collector.ID]
			r.mu.Unlock()
			if ok && previous.hasQuotaSignal() {
				u = mergeStaticWithLastQuota(u, previous)
				u.LastRefreshed = previous.LastRefreshed
				forEachQuotaWindow(&u, func(w *QuotaWindow) { w.Name = staleName(w.Name) })
			}
			u.QuotaFetchError = item.err.Error()
		}
		if progress != nil {
			stage := FetchDone
			if item.err != nil {
				stage = FetchFailed
			}
			progress(item.collector.ID, stage)
		}
		if diagnostic != nil {
			d := CollectorDiagnostic{ID: item.collector.ID, Started: item.started, Duration: item.duration}
			if item.err != nil {
				d.Error = item.err.Error()
			}
			diagnostic(d)
		}
		byID[item.collector.ID] = u
	}
	out := make([]AgentUsage, 0, len(byID))
	byProvider := map[string]int{}
	for _, collector := range r.collectors {
		if collector.Collect == nil {
			continue
		}
		u := byID[collector.ID]
		if idx, ok := byProvider[collector.Provider]; ok {
			out[idx] = mergeCollectorUsage(out[idx], u)
			continue
		}
		byProvider[collector.Provider] = len(out)
		out = append(out, u)
	}
	return out
}

func mergeCollectorUsage(primary, additional AgentUsage) AgentUsage {
	if primary.Name == "" {
		primary = additional
	}
	primary.Sources = append(primary.Sources, additional.Sources...)
	if !primary.hasQuotaSignal() {
		primary.Tokens = additional.Tokens
		primary.Session, primary.Weekly = additional.Session, additional.Weekly
		primary.ModelGroups, primary.ExtraWindows = additional.ModelGroups, additional.ExtraWindows
		return primary
	}
	for _, group := range additional.ModelGroups {
		duplicate := false
		for _, have := range primary.ModelGroups {
			if have.Name != group.Name || len(have.Windows) != len(group.Windows) {
				continue
			}
			duplicate = true
			for i := range have.Windows {
				if have.Windows[i].Name != group.Windows[i].Name || have.Windows[i].UsedPercent != group.Windows[i].UsedPercent {
					duplicate = false
					break
				}
			}
			if duplicate {
				break
			}
		}
		if !duplicate {
			primary.ModelGroups = append(primary.ModelGroups, group)
		}
	}
	return primary
}

func mergeStaticWithLastQuota(current, previous AgentUsage) AgentUsage {
	if !current.hasQuotaSignal() {
		current.Tokens = previous.Tokens
		current.Session, current.Weekly = previous.Session, previous.Weekly
		current.ModelGroups, current.ExtraWindows = previous.ModelGroups, previous.ExtraWindows
	}
	return current
}

func forEachQuotaWindow(u *AgentUsage, fn func(*QuotaWindow)) {
	if u.Session != nil {
		fn(u.Session)
	}
	if u.Weekly != nil {
		fn(u.Weekly)
	}
	for gi := range u.ModelGroups {
		for wi := range u.ModelGroups[gi].Windows {
			fn(&u.ModelGroups[gi].Windows[wi])
		}
	}
	for key, w := range u.ExtraWindows {
		fn(&w)
		u.ExtraWindows[key] = w
	}
}

func newUsageRegistry(homeDir string, client *http.Client, live bool) (*Registry, error) {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	cadence, timeout, err := collectorPolicy()
	if err != nil {
		return nil, err
	}
	collect := func(provider string, fn func(context.Context) AgentUsage) func(context.Context) (AgentUsage, error) {
		return func(ctx context.Context) (AgentUsage, error) {
			stateDir := StateDir(homeDir)
			var cached *AgentSnapshot
			if !live {
				if snapshot, err := ReadAgentSnapshot(stateDir, provider); err == nil && snapshot != nil {
					cached = snapshot
					if snapshot.IsFresh(DefaultCacheStaleness) {
						u := snapshot.Usage
						u.LastRefreshed = snapshot.FetchedAt
						u.Sources = append(u.Sources, fmt.Sprintf("%s (cached)", snapshotPath(stateDir, provider)))
						return u, nil
					}
				}
			}
			u := fn(ctx)
			if cached != nil && cached.Usage.hasQuotaSignal() && !u.hasQuotaSignal() {
				u = mergeStaticWithLastQuota(u, cached.Usage)
				u.LastRefreshed = cached.FetchedAt
				u.Sources = append(u.Sources, fmt.Sprintf("%s (cached, stale)", snapshotPath(stateDir, provider)))
				forEachQuotaWindow(&u, func(w *QuotaWindow) { w.Name = staleName(w.Name) })
			}
			if u.LastRefreshed.IsZero() {
				u.LastRefreshed = time.Now()
			}
			return u, nil
		}
	}
	registry, err := NewRegistry(
		Collector{ID: "claude", Provider: "claude", Capabilities: []string{"quota", "tokens", "metadata"}, Cadence: cadence, Timeout: timeout, Collect: collect("claude", func(ctx context.Context) AgentUsage {
			return CollectClaude(ctx, filepath.Join(homeDir, ".claude"), client)
		})},
		Collector{ID: "agy", Provider: "agy", Capabilities: []string{"quota", "tokens", "metadata", "agy-meter"}, Cadence: cadence, Timeout: timeout, Collect: collect("agy", func(ctx context.Context) AgentUsage {
			return collectAGYWithHome(ctx, filepath.Join(homeDir, ".gemini", "antigravity-cli"), homeDir, client)
		})},
		Collector{ID: "agy-meter", Provider: "agy", Capabilities: []string{"quota", "passive", "proxy"}, Cadence: cadence, Timeout: timeout, Collect: func(context.Context) (AgentUsage, error) {
			u, ok := applyRecentAGYMeterQuota(AgentUsage{AgentID: "agy", Name: "Antigravity (AGY)"}, homeDir, time.Now())
			if !ok {
				return AgentUsage{AgentID: "agy", Name: "Antigravity (AGY)"}, nil
			}
			return u, nil
		}},
		Collector{ID: "codex", Provider: "codex", Capabilities: []string{"quota", "tokens", "metadata"}, Cadence: cadence, Timeout: timeout, Collect: collect("codex", func(ctx context.Context) AgentUsage {
			return CollectCodex(ctx, filepath.Join(homeDir, ".codex"), client)
		})},
	)
	return registry, err
}

func remoteLoadCollector(host string) (Collector, error) {
	cadence, timeout, err := collectorPolicy()
	if err != nil {
		return Collector{}, err
	}
	return Collector{ID: "remote-load", Provider: "remote-load", Capabilities: []string{"load", "remote"}, Cadence: cadence, Timeout: timeout, CollectLoad: func(ctx context.Context) (*LoadSnapshot, error) {
		return collectRemoteLoadSnapshotRaw(ctx, host)
	}}, nil
}

// CollectRegistered runs the provider registry and persists normalized readings
// when the telemetry database is available. The collected in-memory summary is
// returned unchanged if persistence fails.
func CollectRegistered(ctx context.Context, homeDir string, client *http.Client, live bool, progress FetchProgressFunc, diagnostic func(CollectorDiagnostic)) UsageSummary {
	registry, err := newUsageRegistry(homeDir, client, live)
	if err != nil {
		return UsageSummary{Timestamp: time.Now()}
	}
	agents := registry.Collect(ctx, progress, diagnostic)
	now := time.Now()
	for i := range agents {
		if agents[i].LastRefreshed.IsZero() {
			agents[i].LastRefreshed = now
		}
	}
	_ = publishAgentObservations(ctx, agents)
	return UsageSummary{Timestamp: now, Agents: agents}
}

func publishAgentObservations(ctx context.Context, agents []AgentUsage) error {
	dbPath, err := telemetry.DefaultDBPath()
	if err != nil {
		return err
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, q string) error { return store.Exec(ctx, q) }); err != nil {
		return err
	}
	if err := store.MigrateStableWindowKeys(ctx); err != nil {
		return err
	}
	for _, agent := range agents {
		if err := importAgent(store, ctx, agent.AgentID, "registry", agent.LastRefreshed, agent); err != nil {
			return err
		}
	}
	return nil
}
