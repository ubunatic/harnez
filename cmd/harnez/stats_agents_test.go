package main

import (
	"bytes"
	"encoding/json"
	"os"
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
	if err := store.Save(&subagent.Session{ID: "agent-1", Name: "worker", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", HarnessType: "harnez", Status: "completed", CreatedAt: now.Add(-2 * time.Hour), LastActiveAt: now.Add(-time.Hour), Turn: 2, InputTokensTotal: 100, CachedTokensTotal: 40, OutputTokensTotal: 20, TokenTotalsKnown: true, TokensCumulative: 160, TurnRecords: []subagent.TurnRecord{{Turn: 1, NewInputTokens: 40, CachedInputTokens: 15, OutputTokens: 8, Rating: ptrScore(4)}, {Turn: 2, NewInputTokens: 60, CachedInputTokens: 25, OutputTokens: 12, Rating: ptrScore(2)}}}); err != nil {
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
	if host.QuotaSource != "fitted" || host.NewInputTokens != 12 || host.CachedInputTokens != 6 || host.OutputTokens != 8 {
		t.Fatalf("host row=%+v", host)
	}
	if len(report.Models) != 1 || report.Models[0].Model != "codex:gpt-5.6-luna:low" || report.Models[0].Turns != 2 || report.Models[0].NewInputTokens != 100 {
		t.Fatalf("model totals=%+v", report.Models)
	}
	if report.Models[0].MeasuredTurns != 2 || report.Models[0].MeasuredTurnsWithTokens != 2 || report.Models[0].MeasuredDrainPoints != 5 || report.Models[0].MeasuredNewInputTokens != 100 || report.Models[0].PointsPer100KNew != nil {
		t.Fatalf("measured quota totals=%+v", report.Models[0])
	}
	if agent.Rating == nil || *agent.Rating != 3 || report.Models[0].RatingAverage == nil || *report.Models[0].RatingAverage != 3 || report.Models[0].Ratings != 2 {
		t.Fatalf("rating aggregation row=%+v models=%+v", agent, report.Models)
	}

	var table bytes.Buffer
	if err := runAgentStats(&table, agentStatsOptions{Days: 7, HomeDir: home, DBPath: dbPath}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"agent-1", "host-1", "measured", "fitted", "gpt-5.6-luna", "SRC", "QUALITY", "3.0/5", "PTS/100K NEW", "MEASURED TURNS"} {
		if !strings.Contains(table.String(), want) {
			t.Errorf("table missing %q:\n%s", want, table.String())
		}
	}
}

func TestAgentStatsLegacyNewInputSubtractsCachedAndRetainsDeletedSessions(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	storeDir := filepath.Join(home, ".harnez", "agents")
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "old-session", Name: "dev519", Provider: "codex", Model: "gpt-test", Tier: "low", HarnessType: "harnez", Status: "completed", CreatedAt: time.Now().Add(-time.Hour), LastActiveAt: time.Now(), Turn: 2, InputTokensTotal: 1000, CachedTokensTotal: 800, OutputTokensTotal: 50, TokenTotalsKnown: true}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(sess.ID); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runAgentStats(&out, agentStatsOptions{Days: 7, JSON: true, HomeDir: home, DBPath: dbPath}); err != nil {
		t.Fatal(err)
	}
	var report agentStatsReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Sessions) != 1 || report.Sessions[0].NewInputTokens != 200 || report.Sessions[0].CachedInputTokens != 800 {
		t.Fatalf("deleted legacy session report = %+v", report.Sessions)
	}
}

func TestCodexRolloutModelBySession(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".codex", "sessions", "2026", "09")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	data := "{\"type\":\"session_meta\",\"payload\":{\"session_id\":\"host-codex\"}}\n{\"type\":\"turn_context\",\"payload\":{\"model\":\"gpt-6-luna\"}}\n"
	if err := os.WriteFile(filepath.Join(root, "rollout-host.jsonl"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	models := codexRolloutModels(home)
	if models["host-codex"] != "gpt-6-luna" {
		t.Fatalf("rollout model map = %#v", models)
	}
	rows := hostSessionRows([]telemetry.ToolCall{{SessionID: "host-codex", AgentID: "codex", ToolName: "Read", CreatedAt: time.Now()}}, map[string]bool{}, nil, models)
	if len(rows) != 1 || rows[0].Model != "codex:gpt-6-luna" || len(modelTotals(rows)) != 1 {
		t.Fatalf("host model attribution = rows:%+v totals:%+v", rows, modelTotals(rows))
	}
}

func ptr64(v int64) *int64 { return &v }
func ptrScore(v int) *int  { return &v }

func TestShareFittedDrainApportionsOverlappingSessionsByTokens(t *testing.T) {
	rows := []agentSessionRow{
		{SessionID: "host-a", Provider: "codex", StartedAt: time.Unix(1, 0), LastActiveAt: time.Unix(3, 0), NewInputTokens: 30, QuotaDrainPercent: ptrFloat(12), QuotaSource: "fitted"},
		{SessionID: "host-b", Provider: "codex", StartedAt: time.Unix(2, 0), LastActiveAt: time.Unix(4, 0), NewInputTokens: 10, QuotaDrainPercent: ptrFloat(8), QuotaSource: "fitted"},
	}
	shareFittedDrain(rows)
	if *rows[0].QuotaDrainPercent != 9 || *rows[1].QuotaDrainPercent != 2 || !rows[0].DrainShared || !rows[1].DrainShared || rows[0].QuotaSource != "fitted" {
		t.Fatalf("shared fitted rows = %+v", rows)
	}
}

func TestModelTotalsMeasuredDrainAndRateThreshold(t *testing.T) {
	rows := []agentSessionRow{
		{Model: "model-a", Source: "harnez", MeasuredDrainPoints: 3, MeasuredTurns: 3, MeasuredTurnsWithTokens: 3, MeasuredNewInputTokens: 200000},
		{Model: "model-a", Source: "harnez", MeasuredDrainPoints: 2, MeasuredTurns: 2, MeasuredTurnsWithTokens: 2, MeasuredNewInputTokens: 300000},
		{Model: "model-b", Source: "harnez", MeasuredDrainPoints: 1, MeasuredTurns: 4, MeasuredTurnsWithTokens: 4, MeasuredNewInputTokens: 100000},
		{Model: "model-a", Source: "host", MeasuredDrainPoints: 99, MeasuredTurns: 20, MeasuredTurnsWithTokens: 20, MeasuredNewInputTokens: 100000},
	}
	totals := modelTotals(rows)
	if len(totals) != 2 {
		t.Fatalf("model totals = %+v", totals)
	}
	if totals[0].Model != "model-a" || totals[0].MeasuredDrainPoints != 5 || totals[0].MeasuredTurns != 5 || totals[0].MeasuredNewInputTokens != 500000 || totals[0].PointsPer100KNew == nil || *totals[0].PointsPer100KNew != 1 {
		t.Fatalf("model-a measured totals = %+v", totals[0])
	}
	if totals[1].Model != "model-b" || totals[1].MeasuredTurns != 4 || totals[1].PointsPer100KNew != nil {
		t.Fatalf("model-b below-threshold totals = %+v", totals[1])
	}
}

func TestMeasuredTurnMetricsSkipsStalePairsAndKeepsMatchingTurnTokens(t *testing.T) {
	session := &subagent.Session{ID: "agent", TurnRecords: []subagent.TurnRecord{{Turn: 1, NewInputTokens: 10}, {Turn: 2, NewInputTokens: 20}}}
	reading := func(age int64, used int) usage.TurnQuotaReading {
		return usage.TurnQuotaReading{HasCache: true, CacheAgeMS: age, Windows: []usage.QuotaHistoryEntry{{Agent: "codex", Window: "5-hour", UsedPercent: used}}}
	}
	events := map[string]map[int]*quotaTurnPair{"agent": {
		1: {Before: &subagent.TurnQuotaEvent{Reading: reading(100, 10)}, After: &subagent.TurnQuotaEvent{Reading: reading(100, 11), Tokens: &subagent.TurnTokenUsage{NewInputTokens: 100}}},
		2: {Before: &subagent.TurnQuotaEvent{Reading: reading(61_000, 11)}, After: &subagent.TurnQuotaEvent{Reading: reading(100, 18), Tokens: &subagent.TurnTokenUsage{NewInputTokens: 20}}},
	}}
	points, measuredTurns, turnsWithTokens, newInput := measuredTurnMetrics(session, events)
	if points != 1 || measuredTurns != 1 || turnsWithTokens != 1 || newInput != 10 {
		t.Fatalf("measured metrics = points:%v turns:%d tokenTurns:%d new:%d", points, measuredTurns, turnsWithTokens, newInput)
	}
}

func ptrFloat(v float64) *float64 { return &v }
