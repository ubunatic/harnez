package usage

import (
	"os"
	"testing"
	"time"
)

// renderJSONIdentitySummary is the fixed input for the byte-identity check of
// RenderJSON (issue 1177 M2). It sets every field of every type that moves to
// the quota package, so a change in JSON keys, order, or values shows up.
// Reset times are far in the future so expiry cannot depend on the clock.
func renderJSONIdentitySummary() UsageSummary {
	reset := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	return UsageSummary{
		Timestamp:      time.Date(2026, 11, 6, 9, 30, 0, 0, time.UTC),
		PassiveNoQuota: map[string]uint64{"claude": 3},
		Agents: []AgentUsage{{
			AgentID:       "claude",
			Name:          "Claude Code",
			Installed:     true,
			Authenticated: true,
			Account:       "owner",
			PlanTier:      "max",
			ActiveModel:   "claude-haiku-5-5",
			Tokens: &TokenBreakdown{
				InputTokens: 10, OutputTokens: 20, CacheReadTokens: 30,
				CacheWriteTokens: 40, TotalTokens: 100, CostUSD: 1.5,
			},
			ModelTokens: map[string]int64{"claude-haiku-5-5": 100},
			Session: &QuotaWindow{
				Name: "5h", Source: "api", UsedPercent: 40, RemainingPercent: 60,
				ResetAt: &reset, DurationLeft: time.Hour, IsActive: true, Severity: "ok",
			},
			Weekly: &QuotaWindow{
				Name: "Monthly", Source: "api", UsedPercent: 10, RemainingPercent: 90,
				ResetAt: &reset,
			},
			ModelGroups: []ModelGroup{{
				Name: "Claude", Description: "all",
				Windows: []QuotaWindow{{Name: "7d", UsedPercent: 5, RemainingPercent: 95}},
			}},
			ExtraWindows:         map[string]QuotaWindow{"opus": {Name: "opus", UsedPercent: 1, RemainingPercent: 99}},
			Details:              map[string]string{"plan": "max"},
			Sources:              []string{"~/.claude/projects"},
			QuotaFetchError:      "timeout",
			QuotaFetchCause:      "transport",
			QuotaFetchDurationMS: 12,
		}, {
			AgentID:   "codex",
			Name:      "OpenAI Codex",
			Installed: true,
			Error:     "invalid auth.json format",
			Weekly: &QuotaWindow{
				Name: "Weekly (7-day)", Source: "api", UsedPercent: 20, RemainingPercent: 80,
				ResetAt: &reset,
			},
		}},
		Load: &LoadSnapshot{
			CPU: CPULoad{
				Load1: 0.5, Load5: 0.4, Load15: 0.3, NumCPU: 8, Ok: true,
				CPUPercent: 12.5, CPUPercentOk: true,
				PerCorePercent: []float64{10, 15}, PerCoreOk: true,
				PercentHistory: []float64{11, 12},
			},
			GPUs: []GPU{{
				Name: "gpu0", PCI: "0000:03:00.0", UtilPercent: 33,
				MemUsedMiB: 100, MemTotalMiB: 1000, MemPercent: 10,
				TempC: 60, PowerW: 40, HaveMem: true, HavePower: true,
				PowerLabel: "PPT", Class: GPUClassIGPU, UtilHistory: []float64{30, 33},
			}},
			ObservedAt: time.Date(2026, 11, 6, 9, 29, 0, 0, time.UTC),
		},
	}
}

// TestRenderJSONByteIdentity compares RenderJSON output with a golden file
// captured before the quota move. It must stay byte-identical afterwards.
func TestRenderJSONByteIdentity(t *testing.T) {
	got, err := RenderJSON(renderJSONIdentitySummary())
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	want, err := os.ReadFile("testdata/render_json_identity.golden")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Fatalf("RenderJSON output changed; golden is testdata/render_json_identity.golden. got:\n%s", got)
	}
}
