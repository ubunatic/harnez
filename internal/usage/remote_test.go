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

// TestParseRemoteJSON_WithLoad covers issue 090: the remote `usage --json`
// payload can carry a compact Load snapshot (CPU + GPUs) alongside
// UsageSummary, which parseRemoteJSON must decode without a second SSH
// round trip.
func TestParseRemoteJSON_WithLoad(t *testing.T) {
	jsonData := `{
		"timestamp": "2026-08-23T10:00:00Z",
		"agents": [],
		"load": {
			"cpu": {
				"Load1": 1.5,
				"NumCPU": 8,
				"Ok": true,
				"CPUPercent": 33.5,
				"CPUPercentOk": true,
				"Memory": {"UsedMiB": 2048, "TotalMiB": 16384, "Ok": true}
			},
			"gpus": [
				{"Name": "Remote GPU", "UtilPercent": 12.5}
			]
		}
	}`

	summary, _, err := parseRemoteJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseRemoteJSON: %v", err)
	}
	if summary.Load == nil {
		t.Fatalf("expected non-nil Load snapshot")
	}
	if !summary.Load.CPU.Ok || summary.Load.CPU.NumCPU != 8 {
		t.Errorf("unexpected CPU snapshot: %+v", summary.Load.CPU)
	}
	if summary.Load.CPU.CPUPercent != 33.5 {
		t.Errorf("expected CPUPercent 33.5, got %v", summary.Load.CPU.CPUPercent)
	}
	if len(summary.Load.GPUs) != 1 || summary.Load.GPUs[0].Name != "Remote GPU" {
		t.Errorf("unexpected GPUs: %+v", summary.Load.GPUs)
	}
}

// TestUsageSummary_LoadRoundTripsThroughJSON marshals a UsageSummary with a
// Load snapshot and re-parses it through parseRemoteJSON, guarding the full
// wire path CollectRemote depends on (marshal on the remote host's `usage
// --json`, unmarshal on the local watcher).
func TestUsageSummary_LoadRoundTripsThroughJSON(t *testing.T) {
	orig := UsageSummary{
		Timestamp: time.Now().UTC(),
		Agents:    []AgentUsage{{AgentID: "claude", Name: "Claude Code", Installed: true}},
		Load: &LoadSnapshot{
			CPU:  CPULoad{NumCPU: 4, Ok: true, CPUPercent: 55, CPUPercentOk: true},
			GPUs: []GPU{{Name: "GPU0", UtilPercent: 20}},
		},
	}

	data, err := RenderJSON(orig)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}

	got, _, err := parseRemoteJSON([]byte(data))
	if err != nil {
		t.Fatalf("parseRemoteJSON: %v", err)
	}
	if got.Load == nil {
		t.Fatalf("expected Load to survive round trip")
	}
	if got.Load.CPU.NumCPU != 4 || got.Load.CPU.CPUPercent != 55 {
		t.Errorf("CPU snapshot mismatch after round trip: %+v", got.Load.CPU)
	}
	if len(got.Load.GPUs) != 1 || got.Load.GPUs[0].Name != "GPU0" {
		t.Errorf("GPU snapshot mismatch after round trip: %+v", got.Load.GPUs)
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
