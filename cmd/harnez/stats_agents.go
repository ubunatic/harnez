package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usage"
)

type agentStatsOptions struct {
	Days     int
	JSON     bool
	HomeDir  string
	DBPath   string
	StoreDir string
}

type agentStatsReport struct {
	Days     int                `json:"days"`
	Since    time.Time          `json:"since"`
	Sessions []agentSessionRow  `json:"sessions"`
	Models   []agentModelTotals `json:"models"`
}

type agentSessionRow struct {
	SessionID         string    `json:"session_id"`
	Name              string    `json:"name,omitempty"`
	Source            string    `json:"source"`
	Provider          string    `json:"provider"`
	Model             string    `json:"model,omitempty"`
	Turns             int       `json:"turns"`
	TurnsKnown        bool      `json:"turns_known"`
	NewInputTokens    int64     `json:"new_input_tokens"`
	CachedInputTokens int64     `json:"cached_input_tokens"`
	OutputTokens      int64     `json:"output_tokens"`
	TokensComplete    bool      `json:"tokens_complete"`
	QuotaDrainPercent *float64  `json:"quota_drain_percent,omitempty"`
	QuotaSource       string    `json:"quota_source"`
	Unreliable        bool      `json:"unreliable,omitempty"`
	StartedAt         time.Time `json:"started_at"`
	LastActiveAt      time.Time `json:"last_active_at"`
}

type agentModelTotals struct {
	Model                      string `json:"model"`
	Sessions                   int    `json:"sessions"`
	SessionsWithCompleteTokens int    `json:"sessions_with_complete_tokens"`
	SessionsWithKnownTurns     int    `json:"sessions_with_known_turns"`
	Turns                      int    `json:"turns"`
	NewInputTokens             int64  `json:"new_input_tokens"`
	CachedInputTokens          int64  `json:"cached_input_tokens"`
	OutputTokens               int64  `json:"output_tokens"`
}

type quotaTurnPair struct {
	Before *subagent.TurnQuotaEvent
	After  *subagent.TurnQuotaEvent
}

func runAgentStats(w io.Writer, opts agentStatsOptions) error {
	if opts.Days <= 0 {
		return fmt.Errorf("stats --agents: --days must be positive")
	}
	if opts.HomeDir == "" {
		opts.HomeDir, _ = os.UserHomeDir()
	}
	if opts.StoreDir == "" {
		opts.StoreDir = filepath.Join(opts.HomeDir, ".harnez", "agents")
	}
	if opts.DBPath == "" {
		var err error
		opts.DBPath, err = telemetry.DefaultDBPath()
		if err != nil {
			return err
		}
	}
	since := time.Now().UTC().Add(-time.Duration(opts.Days) * 24 * time.Hour)
	store, err := subagent.OpenSessionStore(opts.StoreDir)
	if err != nil {
		return fmt.Errorf("stats --agents: open agent sessions: %w", err)
	}
	sessions, err := store.List("", true)
	if err != nil {
		return fmt.Errorf("stats --agents: list agent sessions: %w", err)
	}
	db, err := telemetry.OpenReadOnly(opts.DBPath)
	if err != nil {
		return fmt.Errorf("stats --agents: open telemetry database: %w", err)
	}
	defer db.Close()
	calls, err := db.Query(telemetry.Filter{Since: since})
	if err != nil {
		return fmt.Errorf("stats --agents: query telemetry: %w", err)
	}
	history := readAgentQuotaHistory(opts.HomeDir)
	quotaEvents := readTurnQuotaEvents(filepath.Join(opts.StoreDir, "quota-readings.jsonl"))
	known := make(map[string]bool, len(sessions))
	rows := make([]agentSessionRow, 0, len(sessions))
	for _, s := range sessions {
		if s.LastActiveAt.Before(since) {
			continue
		}
		known[s.ID] = true
		row := agentSessionRow{SessionID: s.ID, Name: s.Name, Source: "harnez", Provider: s.Provider, Model: s.Provider + ":" + s.Model + ":" + s.Tier, Turns: s.Turn, TurnsKnown: s.Turn > 0, NewInputTokens: int64(s.InputTokensTotal), CachedInputTokens: int64(s.CachedTokensTotal), OutputTokens: int64(s.OutputTokensTotal), TokensComplete: s.TokenTotalsKnown, StartedAt: s.CreatedAt, LastActiveAt: s.LastActiveAt, QuotaSource: "measured"}
		row.QuotaDrainPercent, row.Unreliable = measuredTurnDrain(s.ID, quotaEvents)
		if row.QuotaDrainPercent == nil {
			row.QuotaSource = "unavailable"
		}
		rows = append(rows, row)
	}
	rows = append(rows, hostSessionRows(calls, known, history)...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].LastActiveAt.After(rows[j].LastActiveAt) })
	models := modelTotals(rows)
	report := agentStatsReport{Days: opts.Days, Since: since, Sessions: rows, Models: models}
	if opts.JSON {
		return json.NewEncoder(w).Encode(report)
	}
	return renderAgentStatsTable(w, report)
}

func readTurnQuotaEvents(path string) map[string]map[int]*quotaTurnPair {
	out := map[string]map[int]*quotaTurnPair{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var event subagent.TurnQuotaEvent
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		byTurn := out[event.SessionID]
		if byTurn == nil {
			byTurn = map[int]*quotaTurnPair{}
			out[event.SessionID] = byTurn
		}
		pair := byTurn[event.Turn]
		if pair == nil {
			pair = &quotaTurnPair{}
			byTurn[event.Turn] = pair
		}
		copyEvent := event
		if event.Boundary == "before" {
			pair.Before = &copyEvent
		}
		if event.Boundary == "after" {
			pair.After = &copyEvent
		}
	}
	return out
}

func measuredTurnDrain(sessionID string, events map[string]map[int]*quotaTurnPair) (*float64, bool) {
	byTurn := events[sessionID]
	if len(byTurn) == 0 {
		return nil, true
	}
	total := 0.0
	measured, unreliable := false, false
	for _, pair := range byTurn {
		if pair.Before == nil || pair.After == nil {
			unreliable = true
			continue
		}
		before, after := pair.Before.Reading, pair.After.Reading
		if before.Error != "" || after.Error != "" || before.CacheAgeMS > 60000 || after.CacheAgeMS > 60000 || !before.HasCache || !after.HasCache {
			unreliable = true
		}
		changes, resetOrMissing := quotaWindowDelta(before.Windows, after.Windows)
		if resetOrMissing {
			unreliable = true
		}
		if changes != nil {
			total += *changes
			measured = true
		}
	}
	if !measured {
		return nil, unreliable
	}
	return &total, unreliable
}

func isFiveHourWindow(name string) bool {
	x := strings.ToLower(name)
	return strings.Contains(x, "5-hour") || strings.Contains(x, "5h") || (strings.Contains(x, "session") && !strings.Contains(x, "weekly"))
}

func quotaWindowDelta(before, after []usage.QuotaHistoryEntry) (*float64, bool) {
	type key struct{ agent, group, window string }
	prev := map[key]usage.QuotaHistoryEntry{}
	for _, entry := range before {
		if isFiveHourWindow(entry.Window) {
			prev[key{entry.Agent, entry.Group, entry.Window}] = entry
		}
	}
	total, matched, reset := 0.0, 0, false
	for _, entry := range after {
		k := key{entry.Agent, entry.Group, entry.Window}
		old, ok := prev[k]
		if !ok {
			continue
		}
		if entry.UsedPercent < old.UsedPercent {
			reset = true
			continue
		}
		total += float64(entry.UsedPercent - old.UsedPercent)
		matched++
	}
	if matched == 0 {
		return nil, true
	}
	return &total, reset
}

func hostSessionRows(calls []telemetry.ToolCall, known map[string]bool, history []usage.QuotaHistoryEntry) []agentSessionRow {
	byID := map[string]*agentSessionRow{}
	tokenFields := map[string][3]bool{}
	for _, call := range calls {
		if call.SessionID == "" || known[call.SessionID] {
			continue
		}
		row := byID[call.SessionID]
		if row == nil {
			row = &agentSessionRow{SessionID: call.SessionID, Source: "host", Provider: call.AgentID, StartedAt: call.CreatedAt, LastActiveAt: call.CreatedAt, QuotaSource: "fitted"}
			byID[call.SessionID] = row
		}
		if call.CreatedAt.Before(row.StartedAt) {
			row.StartedAt = call.CreatedAt
		}
		if call.CreatedAt.After(row.LastActiveAt) {
			row.LastActiveAt = call.CreatedAt
		}
		if call.InputTokens != nil {
			fields := tokenFields[call.SessionID]
			fields[0] = true
			tokenFields[call.SessionID] = fields
			row.NewInputTokens += *call.InputTokens
		}
		if call.CachedInputTokens != nil {
			fields := tokenFields[call.SessionID]
			fields[1] = true
			tokenFields[call.SessionID] = fields
			row.CachedInputTokens += *call.CachedInputTokens
		}
		if call.OutputTokens != nil {
			fields := tokenFields[call.SessionID]
			fields[2] = true
			tokenFields[call.SessionID] = fields
			row.OutputTokens += *call.OutputTokens
		}
	}
	rows := make([]agentSessionRow, 0, len(byID))
	for _, row := range byID {
		fields := tokenFields[row.SessionID]
		row.TokensComplete = fields[0] && fields[1] && fields[2]
		row.QuotaDrainPercent = fittedDrain(*row, history)
		rows = append(rows, *row)
	}
	return rows
}

func fittedDrain(session agentSessionRow, history []usage.QuotaHistoryEntry) *float64 {
	type key struct{ group, window string }
	type bounds struct{ before, after *usage.QuotaHistoryEntry }
	pairs := map[key]*bounds{}
	for i := range history {
		e := &history[i]
		if e.Agent != session.Provider || !isFiveHourWindow(e.Window) {
			continue
		}
		k := key{e.Group, e.Window}
		b := pairs[k]
		if b == nil {
			b = &bounds{}
			pairs[k] = b
		}
		if !e.Timestamp.After(session.StartedAt) && (b.before == nil || e.Timestamp.After(b.before.Timestamp)) {
			b.before = e
		}
		if !e.Timestamp.Before(session.LastActiveAt) && (b.after == nil || e.Timestamp.Before(b.after.Timestamp)) {
			b.after = e
		}
	}
	total, matched := 0.0, false
	for _, b := range pairs {
		if b.before == nil || b.after == nil || b.after.UsedPercent < b.before.UsedPercent {
			continue
		}
		total += float64(b.after.UsedPercent - b.before.UsedPercent)
		matched = true
	}
	if !matched {
		return nil
	}
	return &total
}

func readAgentQuotaHistory(home string) []usage.QuotaHistoryEntry {
	var all []usage.QuotaHistoryEntry
	entries, err := usage.ReadQuotaHistory(usage.HistoryDir(home))
	if err == nil {
		all = append(all, entries...)
	}
	return all
}

func modelTotals(rows []agentSessionRow) []agentModelTotals {
	byModel := map[string]*agentModelTotals{}
	for _, row := range rows {
		if row.Source != "harnez" || row.Model == "" {
			continue
		}
		t := byModel[row.Model]
		if t == nil {
			t = &agentModelTotals{Model: row.Model}
			byModel[row.Model] = t
		}
		t.Sessions++
		if row.TurnsKnown {
			t.SessionsWithKnownTurns++
			t.Turns += row.Turns
		}
		if row.TokensComplete {
			t.SessionsWithCompleteTokens++
			t.NewInputTokens += row.NewInputTokens
			t.CachedInputTokens += row.CachedInputTokens
			t.OutputTokens += row.OutputTokens
		}
	}
	out := make([]agentModelTotals, 0, len(byModel))
	for _, t := range byModel {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return out
}

func renderAgentStatsTable(w io.Writer, report agentStatsReport) error {
	fmt.Fprintf(w, "Agent sessions in last %d days (since %s)\n", report.Days, report.Since.Format("2006-01-02"))
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SESSION\tSOURCE\tPROVIDER\tMODEL\tTURNS\tNEW INPUT\tCACHED\tOUTPUT\t5H DRAIN\tQUALITY")
	for _, row := range report.Sessions {
		drain := "—"
		if row.QuotaDrainPercent != nil {
			drain = fmt.Sprintf("%.1f%%", *row.QuotaDrainPercent)
		}
		quality := row.QuotaSource
		if row.Unreliable {
			quality += " (unreliable)"
		}
		turns, input, cached, output := fmt.Sprint(row.Turns), fmt.Sprint(row.NewInputTokens), fmt.Sprint(row.CachedInputTokens), fmt.Sprint(row.OutputTokens)
		if !row.TurnsKnown {
			turns = "—"
		}
		if !row.TokensComplete {
			input, cached, output = "—", "—", "—"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", row.SessionID, row.Source, row.Provider, row.Model, turns, input, cached, output, drain, quality)
	}
	fmt.Fprintln(tw, "\nPer-model totals:")
	fmt.Fprintln(tw, "MODEL\tSESSIONS (TOKEN COMPLETE)\tTURNS (KNOWN)\tNEW INPUT\tCACHED\tOUTPUT")
	for _, model := range report.Models {
		fmt.Fprintf(tw, "%s\t%d/%d\t%d/%d\t%d\t%d\t%d\n", model.Model, model.SessionsWithCompleteTokens, model.Sessions, model.Turns, model.SessionsWithKnownTurns, model.NewInputTokens, model.CachedInputTokens, model.OutputTokens)
	}
	return tw.Flush()
}
