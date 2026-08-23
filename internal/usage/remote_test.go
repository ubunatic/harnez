package usage

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestParseRemoteJSON_Basic(t *testing.T) {
	jsonData := `{
		"timestamp": "2026-08-23T10:00:00Z",
		"agents": [
			{
				"agent_id": "claude",
				"name": "Claude Code",
				"installed": true,
				"authenticated": true,
				"account": "user@example.com",
				"plan_tier": "Pro",
				"tokens": {
					"total_tokens": 150000
				}
			}
		]
	}`

	summary, procs, err := parseRemoteJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseRemoteJSON: %v", err)
	}

	if len(summary.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(summary.Agents))
	}
	if summary.Agents[0].AgentID != "claude" {
		t.Errorf("expected claude, got %s", summary.Agents[0].AgentID)
	}
	if summary.Agents[0].Tokens.TotalTokens != 150000 {
		t.Errorf("expected 150000 tokens, got %d", summary.Agents[0].Tokens.TotalTokens)
	}
	if procs != nil {
		t.Errorf("expected nil procs, got %v", procs)
	}
}

func TestParseRemoteJSON_WithProcesses(t *testing.T) {
	jsonData := `{
		"timestamp": "2026-08-23T10:00:00Z",
		"agents": [
			{
				"agent_id": "agy",
				"name": "Antigravity",
				"installed": true,
				"authenticated": true
			}
		],
		"processes": {
			"Claude": 2,
			"AGY": 1,
			"Codex": 0
		}
	}`

	summary, procs, err := parseRemoteJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseRemoteJSON: %v", err)
	}

	if len(summary.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(summary.Agents))
	}
	if procs == nil {
		t.Fatalf("expected procs, got nil")
	}
	if procs.Claude != 2 || procs.AGY != 1 || procs.Codex != 0 {
		t.Errorf("unexpected procs: %+v", procs)
	}
	if procs.Total() != 3 {
		t.Errorf("expected total 3, got %d", procs.Total())
	}
}

func TestParseRemoteJSON_InvalidJSON(t *testing.T) {
	_, _, err := parseRemoteJSON([]byte("not-json"))
	if err == nil {
		t.Fatalf("expected error parsing invalid JSON, got nil")
	}
}

func TestCollectRemote_Validation(t *testing.T) {
	ctx := context.Background()

	// Empty host
	summary, procs, err := CollectRemote(ctx, "", false)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty host error, got %v", err)
	}
	if summary.Agents[0].QuotaFetchError == "" {
		t.Errorf("expected QuotaFetchError to be set on summary")
	}
	if procs != nil {
		t.Errorf("expected nil procs on error")
	}

	// Flag injection
	summary, _, err = CollectRemote(ctx, "-oProxyCommand=calc", false)
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected invalid host error, got %v", err)
	}
	if summary.Agents[0].QuotaFetchError == "" {
		t.Errorf("expected QuotaFetchError to be set on summary")
	}

	// Host with invalid characters
	_, _, err = CollectRemote(ctx, "host;rm -rf /", false)
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected invalid characters error, got %v", err)
	}
}

func TestCollectRemote_NonExistentHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	summary, procs, err := CollectRemote(ctx, "nonexistent-ssh-host-123456", false)
	if err == nil {
		t.Errorf("expected error connecting to non-existent host, got nil")
	}
	if len(summary.Agents) != 1 {
		t.Fatalf("expected 1 fallback agent, got %d", len(summary.Agents))
	}
	if summary.Agents[0].QuotaFetchError == "" {
		t.Errorf("expected fallback agent to have QuotaFetchError set")
	}
	if !strings.Contains(summary.Agents[0].Name, "Remote (nonexistent-ssh-host-123456)") {
		t.Errorf("expected fallback agent name to mention remote host, got %q", summary.Agents[0].Name)
	}
	if procs != nil {
		t.Errorf("expected nil procs on connection failure")
	}
}
