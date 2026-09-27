package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	internalusage "ubunatic.com/harnez/internal/usage"
	sharedusage "ubunatic.com/harnez/usage"
)

type realUsageCollector struct{ client *http.Client }

func (c realUsageCollector) CollectUsage(ctx context.Context, provider sharedusage.ProviderID) (sharedusage.Snapshot, error) {
	home, _ := os.UserHomeDir()
	var got internalusage.AgentUsage
	switch provider {
	case sharedusage.ProviderClaude:
		got = internalusage.CollectClaude(ctx, filepath.Join(home, ".claude"), c.client)
	case sharedusage.ProviderAGY:
		got = internalusage.CollectAGY(ctx, filepath.Join(home, ".gemini", "antigravity-cli"), c.client)
	case sharedusage.ProviderCodex:
		got = internalusage.CollectCodex(ctx, filepath.Join(home, ".codex"), c.client)
	default:
		return sharedusage.Snapshot{}, fmt.Errorf("unsupported usage provider %q", provider)
	}
	return publicSnapshot(got), nil
}

func publicSnapshot(a internalusage.AgentUsage) sharedusage.Snapshot {
	u := sharedusage.UsageData{
		Name: a.Name, Installed: a.Installed, Authenticated: a.Authenticated,
		Account: a.Account, PlanTier: a.PlanTier, ActiveModel: a.ActiveModel,
		ModelTokens: a.ModelTokens, Details: a.Details, Sources: safeSources(a.Sources),
		Error: a.Error, QuotaFetchError: a.QuotaFetchError,
	}
	u.ExtraWindows = make(map[string]sharedusage.QuotaWindow, len(a.ExtraWindows))
	if a.Tokens != nil {
		u.Tokens = &sharedusage.TokenUsage{InputTokens: a.Tokens.InputTokens, OutputTokens: a.Tokens.OutputTokens, CacheReadTokens: a.Tokens.CacheReadTokens, CacheWriteTokens: a.Tokens.CacheWriteTokens, TotalTokens: a.Tokens.TotalTokens, CostUSD: a.Tokens.CostUSD}
	}
	u.Session = publicWindow(a.Session)
	u.Weekly = publicWindow(a.Weekly)
	for key, value := range a.ExtraWindows {
		u.ExtraWindows[key] = publicWindowValue(value)
	}
	for _, group := range a.ModelGroups {
		g := sharedusage.ModelGroup{Name: group.Name, Description: group.Description}
		for _, window := range group.Windows {
			g.Windows = append(g.Windows, publicWindowValue(window))
		}
		u.ModelGroups = append(u.ModelGroups, g)
	}
	status := sharedusage.StatusLive
	if a.QuotaFetchError != "" {
		status = sharedusage.StatusStale
	}
	now := time.Now().UTC()
	return sharedusage.Snapshot{SchemaVersion: sharedusage.SnapshotSchemaVersion, ProviderID: sharedusage.ProviderID(a.AgentID), FetchedAt: now, ObservedAt: now, Status: status, Source: sharedusage.SourceLive, Usage: u}
}

func publicWindow(w *internalusage.QuotaWindow) *sharedusage.QuotaWindow {
	if w == nil {
		return nil
	}
	converted := publicWindowValue(*w)
	return &converted
}

func publicWindowValue(w internalusage.QuotaWindow) sharedusage.QuotaWindow {
	return sharedusage.QuotaWindow{Name: w.Name, Source: w.Source, UsedPercent: w.UsedPercent, RemainingPercent: w.RemainingPercent, ResetAt: w.ResetAt, DurationLeftMS: w.DurationLeft.Milliseconds(), IsActive: w.IsActive, Severity: w.Severity}
}

func safeSources(sources []string) []string {
	result := make([]string, 0, len(sources))
	for _, source := range sources {
		// Collector source labels are useful to the existing renderers, but
		// never let a custom absolute path escape through the public API.
		if filepath.IsAbs(source) {
			if strings.Contains(source, " (stale)") || strings.Contains(source, " (cached") {
				result = append(result, "collector cache"+source[strings.Index(source, " ("):])
			} else {
				result = append(result, "local usage source")
			}
			continue
		}
		result = append(result, source)
	}
	return result
}

func agentUsageFromSnapshot(s sharedusage.Snapshot) internalusage.AgentUsage {
	u := s.Usage
	a := internalusage.AgentUsage{
		AgentID: string(s.ProviderID), Name: u.Name, Installed: u.Installed, Authenticated: u.Authenticated,
		Account: u.Account, PlanTier: u.PlanTier, ActiveModel: u.ActiveModel, ModelTokens: u.ModelTokens,
		Details: u.Details, Sources: u.Sources, Error: u.Error, QuotaFetchError: u.QuotaFetchError, LastRefreshed: s.ObservedAt,
	}
	a.ExtraWindows = make(map[string]internalusage.QuotaWindow, len(u.ExtraWindows))
	if u.Tokens != nil {
		a.Tokens = &internalusage.TokenBreakdown{InputTokens: u.Tokens.InputTokens, OutputTokens: u.Tokens.OutputTokens, CacheReadTokens: u.Tokens.CacheReadTokens, CacheWriteTokens: u.Tokens.CacheWriteTokens, TotalTokens: u.Tokens.TotalTokens, CostUSD: u.Tokens.CostUSD}
	}
	a.Session = internalWindow(u.Session)
	a.Weekly = internalWindow(u.Weekly)
	for key, value := range u.ExtraWindows {
		a.ExtraWindows[key] = *internalWindow(&value)
	}
	for _, group := range u.ModelGroups {
		g := internalusage.ModelGroup{Name: group.Name, Description: group.Description}
		for _, window := range group.Windows {
			g.Windows = append(g.Windows, *internalWindow(&window))
		}
		a.ModelGroups = append(a.ModelGroups, g)
	}
	if s.Status == sharedusage.StatusStale || s.Status == sharedusage.StatusError || s.Status == sharedusage.StatusThrottled {
		if a.QuotaFetchError == "" {
			a.QuotaFetchError = string(s.Status)
		}
	}
	return a
}

func internalWindow(w *sharedusage.QuotaWindow) *internalusage.QuotaWindow {
	if w == nil {
		return nil
	}
	return &internalusage.QuotaWindow{Name: w.Name, Source: w.Source, UsedPercent: w.UsedPercent, RemainingPercent: w.RemainingPercent, ResetAt: w.ResetAt, DurationLeft: time.Duration(w.DurationLeftMS) * time.Millisecond, IsActive: w.IsActive, Severity: w.Severity}
}

func summaryFromSnapshots(snapshots []sharedusage.Snapshot) internalusage.UsageSummary {
	summary := internalusage.UsageSummary{Timestamp: time.Now().UTC()}
	for _, snapshot := range snapshots {
		summary.Agents = append(summary.Agents, agentUsageFromSnapshot(snapshot))
	}
	return summary
}

func collectSharedUsage(ctx context.Context, client sharedusage.Client, refresh bool) internalusage.UsageSummary {
	var snapshots []sharedusage.Snapshot
	if refresh {
		var err error
		snapshots, err = client.Refresh(ctx)
		if err != nil {
			return internalusage.UsageSummary{Timestamp: time.Now().UTC()}
		}
	} else {
		for _, provider := range []sharedusage.ProviderID{sharedusage.ProviderClaude, sharedusage.ProviderAGY, sharedusage.ProviderCodex} {
			snapshot, err := client.Snapshot(ctx, provider)
			if err == nil && snapshot != nil {
				snapshots = append(snapshots, *snapshot)
			}
		}
	}
	return summaryFromSnapshots(snapshots)
}

func renderUsageOutput(summary internalusage.UsageSummary, jsonOutput, raw bool, opts ...internalusage.WatchOptions) (string, error) {
	if jsonOutput {
		return internalusage.RenderJSON(summary)
	}
	if raw {
		return internalusage.RenderText(summary, opts...), nil
	}
	return "", fmt.Errorf("usage output requires --json or --raw")
}

func pinSharedClient(ctx context.Context, client sharedusage.Client) (func(), error) {
	if client.ControllerInfo().State == sharedusage.ControllerUnavailable {
		return func() {}, nil
	}
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	for _, provider := range []sharedusage.ProviderID{sharedusage.ProviderClaude, sharedusage.ProviderAGY, sharedusage.ProviderCodex} {
		updates, err := client.Subscribe(ctx, provider)
		if err != nil {
			cancel()
			wg.Wait()
			return nil, fmt.Errorf("subscribe to shared %s usage: %w", provider, err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range updates {
			}
		}()
	}
	return func() {
		cancel()
		wg.Wait()
	}, nil
}
