package usage

import (
	"context"
	"os"

	"ubunatic.com/harnez/internal/server"
)

func inRunnerMode() bool {
	return os.Getenv("TAIA_SESSION_DIR") != ""
}

func queryRunnerLatestSummary(ctx context.Context) (UsageSummary, error) {
	cfg, err := server.LoadConfig()
	if err != nil {
		return UsageSummary{}, err
	}
	var summary UsageSummary
	if err := server.QueryRunnerUsage(ctx, cfg, server.RunnerUsageQuery{
		Action: server.RunnerUsageActionLatestSummary,
	}, &summary); err != nil {
		return UsageSummary{}, err
	}
	return summary, nil
}

func queryRunnerProviderQuota(ctx context.Context, provider string, models []string) (map[string]ProviderQuotaAvailability, error) {
	cfg, err := server.LoadConfig()
	if err != nil {
		return nil, err
	}
	var states map[string]ProviderQuotaAvailability
	if err := server.QueryRunnerUsage(ctx, cfg, server.RunnerUsageQuery{
		Action:   server.RunnerUsageActionProviderQuota,
		Provider: provider,
		Models:   append([]string(nil), models...),
	}, &states); err != nil {
		return nil, err
	}
	return states, nil
}

func queryRunnerHistoryEntries(ctx context.Context) ([]HistoryEntry, error) {
	cfg, err := server.LoadConfig()
	if err != nil {
		return nil, err
	}
	var entries []HistoryEntry
	if err := server.QueryRunnerUsage(ctx, cfg, server.RunnerUsageQuery{
		Action: server.RunnerUsageActionHistoryEntries,
	}, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func queryRunnerQuotaHistory(ctx context.Context) ([]QuotaHistoryEntry, error) {
	cfg, err := server.LoadConfig()
	if err != nil {
		return nil, err
	}
	var entries []QuotaHistoryEntry
	if err := server.QueryRunnerUsage(ctx, cfg, server.RunnerUsageQuery{
		Action: server.RunnerUsageActionQuotaHistory,
	}, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
