package usagecli

import (
	"encoding/json"
	"fmt"
	"strings"

	"ubunatic.com/harnez/internal/usage"
)

func RenderUsageOutput(summary usage.UsageSummary, jsonOutput, raw bool, opts ...usage.WatchOptions) (string, error) {
	if jsonOutput {
		return usage.RenderJSON(summary)
	}
	if raw {
		return usage.RenderText(summary, opts...), nil
	}
	return "", fmt.Errorf("usage output requires --json or --raw")
}

// FilterSummaryByAgent applies the usage command's case-insensitive provider filter.
func FilterSummaryByAgent(summary usage.UsageSummary, agentID string) usage.UsageSummary {
	if agentID == "" {
		return summary
	}
	filtered := make([]usage.AgentUsage, 0, len(summary.Agents))
	for _, agent := range summary.Agents {
		if strings.EqualFold(agent.AgentID, agentID) {
			filtered = append(filtered, agent)
		}
	}
	summary.Agents = filtered
	return summary
}

// RenderProjectUsage formats a project result as JSON or the default text card.
func RenderProjectUsage(result *usage.ProjectUsageResult, jsonOutput bool) (string, error) {
	if jsonOutput {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return usage.RenderProjectUsageCard(result), nil
}
