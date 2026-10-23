package usagecli

import (
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

func TestFilterSummaryByAgentIsCaseInsensitiveAndPreservesTimestamp(t *testing.T) {
	when := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	summary := usage.UsageSummary{
		Timestamp: when,
		Agents: []usage.AgentUsage{
			{AgentID: "claude"},
			{AgentID: "codex"},
			{AgentID: "claude"},
		},
	}
	got := FilterSummaryByAgent(summary, "CoDeX")
	if got.Timestamp != when || len(got.Agents) != 1 || got.Agents[0].AgentID != "codex" {
		t.Fatalf("filtered summary = %+v", got)
	}
	if len(summary.Agents) != 3 {
		t.Fatalf("filter mutated the input summary: %+v", summary.Agents)
	}
}
