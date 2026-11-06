package main

import (
	"encoding/json"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

// This test acts as a contract test for taia-models.
// See issue 1176 for the fields taia-models reads from harnez usage --json.
// If this test fails because a field was renamed or dropped, taia's model picker will break silently.
func TestUsageJSONContract_TaiaModelsReads(t *testing.T) {
	now := time.Now()

	// Construct the exact snapshot layout the usage JSON outputs.
	summary := usage.UsageSummary{
		Agents: []usage.AgentUsage{
			{
				AgentID:       "claude",
				Installed:     true,
				Authenticated: true,
				PlanTier:      "pro",
				Stale:         true,
				LastRefreshed: now,
				Session: &usage.QuotaWindow{
					RemainingPercent: 80.0,
					UsedPercent:      20.0,
					ResetAt:          &now,
				},
				Weekly: &usage.QuotaWindow{
					RemainingPercent: 50.0,
					UsedPercent:      50.0,
					ResetAt:          &now,
				},
			},
		},
	}

	// RenderJSON is what `harnez usage --json` prints, including staleness fill-in.
	out, err := usage.RenderJSON(summary)
	if err != nil {
		t.Fatalf("RenderJSON failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	agentsAny, ok := m["agents"]
	if !ok {
		t.Fatalf("missing top level 'agents'")
	}
	agents, ok := agentsAny.([]any)
	if !ok {
		t.Fatalf("'agents' is not an array")
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	agent, ok := agents[0].(map[string]any)
	if !ok {
		t.Fatalf("agent is not a map")
	}

	assertFieldType := func(m map[string]any, field string, expectedTypes ...string) {
		t.Helper()
		val, ok := m[field]
		if !ok {
			t.Errorf("missing field %q", field)
			return
		}

		matched := false
		var actualType string
		switch val.(type) {
		case string:
			actualType = "string"
		case bool:
			actualType = "bool"
		case float64:
			actualType = "number"
		case nil:
			actualType = "null"
		default:
			actualType = "unknown"
		}

		for _, et := range expectedTypes {
			if actualType == et {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("field %q: expected one of %v, got %T", field, expectedTypes, val)
		}
	}

	assertFieldType(agent, "agent_id", "string")
	assertFieldType(agent, "installed", "bool")
	assertFieldType(agent, "authenticated", "bool")
	assertFieldType(agent, "plan_tier", "string")
	assertFieldType(agent, "stale", "bool")
	assertFieldType(agent, "last_refreshed", "string") // RFC 3339

	for _, winField := range []string{"session", "weekly"} {
		winAny, ok := agent[winField]
		if !ok {
			t.Errorf("missing field %q", winField)
			continue
		}
		win, ok := winAny.(map[string]any)
		if !ok {
			t.Errorf("%q is not a map", winField)
			continue
		}
		assertFieldType(win, "remaining_percent", "number")
		assertFieldType(win, "used_percent", "number")
		assertFieldType(win, "reset_at", "string", "null")
	}
}
