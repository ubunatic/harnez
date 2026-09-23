package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usage"
)

func TestRunAgentStatsJoinsSessionsTokensQuotaAndHostFitted(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	for _, row := range []telemetry.ToolCall{
		{CreatedAt: now.Add(-time.Hour), SessionID: "host-1", AgentID: "codex", ToolName: "Read", InputTokens: ptr64(11), CachedInputTokens: ptr64(4), OutputTokens: ptr64(3)},
		{CreatedAt: now.Add(-30 * time.Minute), SessionID: "host-1", AgentID: "codex", ToolName: "Bash", InputTokens: ptr64(7), CachedInputTokens: ptr64(2), OutputTokens: ptr64(5)},
	} {
		if err := db.Insert(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	storeDir := filepath.Join(home, ".harnez", "agents")
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "agent-1", Name: "worker", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", HarnessType: "harnez", Status: "completed", CreatedAt: now.Add(-2 * time.Hour), LastActiveAt: now.Add(-time.Hour), Turn: 2, InputTokensTotal: 100, CachedTokensTotal: 40, OutputTokensTotal: 20, TokenTotalsKnown: true, TokensCumulative: 160}); err != nil {
		t.Fatal(err)
	}
	for turn, bounds := range [][2]float64{{10, 12}, {12, 15}} {
		for i, boundary := range []string{"before", "after"} {
			used := bounds[i]
			r := usage.TurnQuotaReading{CapturedAt: now.Add(time.Duration(turn*2+i) * time.Minute), HasCache: true, CacheAgeMS: 800, Windows: []usage.QuotaHistoryEntry{{Agent: "codex", Window: "5-hour", UsedPercent: int(used)}}}
			if err := store.RecordTurnQuota(subagent.TurnQuotaEvent{SessionID: "agent-1", Turn: turn + 1, Boundary: boundary, Provider: "codex", Reading: r}); err != nil {
				t.Fatal(err)
			}
		}
	}
	historyDir := usage.HistoryDir(home)
	if err := usage.AppendQuotaHistory(historyDir, []usage.QuotaHistoryEntry{
		{Timestamp: now.Add(-2 * time.Hour), Agent: "codex", Window: "5-hour", UsedPercent: 20},
		{Timestamp: now.Add(-20 * time.Minute), Agent: "codex", Window: "5-hour", UsedPercent: 24},
	}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runAgentStats(&out, agentStatsOptions{Days: 7, JSON: true, HomeDir: home, DBPath: dbPath}); err != nil {
		t.Fatal(err)
	}
	var report agentStatsReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, out.String())
	}
	if len(report.Sessions) != 2 {
		t.Fatalf("sessions=%+v, want agent and host rows", report.Sessions)
	}
	byID := map[string]agentSessionRow{}
	for _, row := range report.Sessions {
		byID[row.SessionID] = row
	}
	agent := byID["agent-1"]
	if agent.Turns != 2 || agent.NewInputTokens != 100 || agent.CachedInputTokens != 40 || agent.OutputTokens != 20 || agent.QuotaSource != "measured" || agent.QuotaDrainPercent == nil || *agent.QuotaDrainPercent != 5 {
		t.Fatalf("agent row=%+v", agent)
	}
	host := byID["host-1"]
	if host.QuotaSource != "fitted" || host.NewInputTokens != 18 || host.CachedInputTokens != 6 || host.OutputTokens != 8 {
		t.Fatalf("host row=%+v", host)
	}
	if len(report.Models) != 1 || report.Models[0].Model != "codex:gpt-5.6-luna:low" || report.Models[0].Turns != 2 || report.Models[0].NewInputTokens != 100 {
		t.Fatalf("model totals=%+v", report.Models)
	}

	var table bytes.Buffer
	if err := runAgentStats(&table, agentStatsOptions{Days: 7, HomeDir: home, DBPath: dbPath}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"agent-1", "host-1", "measured", "fitted", "gpt-5.6-luna"} {
		if !strings.Contains(table.String(), want) {
			t.Errorf("table missing %q:\n%s", want, table.String())
		}
	}
}

func ptr64(v int64) *int64 { return &v }
