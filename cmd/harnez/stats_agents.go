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
	All      bool
}

type agentStatsReport struct {
	Days            int                `json:"days"`
	Since           time.Time          `json:"since"`
	Sessions        []agentSessionRow  `json:"sessions"`
	SessionsShown   int                `json:"sessions_shown"`
	SessionsOmitted int                `json:"sessions_omitted"`
	Models          []agentModelTotals `json:"models"`
}

type agentSessionRow struct {
	SessionID               string    `json:"session_id"`
	Name                    string    `json:"name,omitempty"`
	Source                  string    `json:"source"`
	Provider                string    `json:"provider"`
	Model                   string    `json:"model,omitempty"`
	Turns                   int       `json:"turns"`
	TurnsKnown              bool      `json:"turns_known"`
	NewInputTokens          int64     `json:"new_input_tokens"`
	CachedInputTokens       int64     `json:"cached_input_tokens"`
	OutputTokens            int64     `json:"output_tokens"`
	TokensComplete          bool      `json:"tokens_complete"`
	QuotaDrainPercent       *float64  `json:"quota_drain_percent,omitempty"`
	QuotaSource             string    `json:"quota_source"`
	Unreliable              bool      `json:"unreliable,omitempty"`
	DrainShared             bool      `json:"drain_shared,omitempty"`
	Rating                  *float64  `json:"rating,omitempty"`
	Ratings                 int       `json:"ratings"`
	MeasuredDrainPoints     float64   `json:"measured_drain_points"`
	MeasuredTurns           int       `json:"measured_turns"`
	MeasuredTurnsWithTokens int       `json:"measured_turns_with_tokens"`
	MeasuredNewInputTokens  int64     `json:"measured_new_input_tokens"`
	StartedAt               time.Time `json:"started_at"`
	LastActiveAt            time.Time `json:"last_active_at"`
}

type agentModelTotals struct {
	Model                      string   `json:"model"`
	Sessions                   int      `json:"sessions"`
	SessionsWithCompleteTokens int      `json:"sessions_with_complete_tokens"`
	SessionsWithKnownTurns     int      `json:"sessions_with_known_turns"`
	Turns                      int      `json:"turns"`
	NewInputTokens             int64    `json:"new_input_tokens"`
	CachedInputTokens          int64    `json:"cached_input_tokens"`
	OutputTokens               int64    `json:"output_tokens"`
	RatingAverage              *float64 `json:"rating_average,omitempty"`
	Ratings                    int      `json:"ratings"`
	MeasuredDrainPoints        float64  `json:"measured_drain_points"`
	MeasuredTurns              int      `json:"measured_turns"`
	MeasuredTurnsWithTokens    int      `json:"measured_turns_with_tokens"`
	MeasuredNewInputTokens     int64    `json:"measured_new_input_tokens"`
	PointsPer100KNew           *float64 `json:"points_per_100k_new,omitempty"`
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
	deleted, err := store.ListDeleted()
	if err != nil {
		return fmt.Errorf("stats --agents: list deleted sessions: %w", err)
	}
	seen := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		seen[s.ID] = true
	}
	for _, s := range deleted {
		if !seen[s.ID] {
			sessions = append(sessions, s)
			seen[s.ID] = true
		}
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
		legacyNewInput := s.InputTokensTotal - s.CachedTokensTotal
		if legacyNewInput < 0 {
			legacyNewInput = 0
		}
		row := agentSessionRow{SessionID: s.ID, Name: s.Name, Source: "harnez", Provider: s.Provider, Model: s.Provider + ":" + s.Model + ":" + s.Tier, Turns: s.Turn, TurnsKnown: s.Turn > 0, NewInputTokens: int64(legacyNewInput), CachedInputTokens: int64(s.CachedTokensTotal), OutputTokens: int64(s.OutputTokensTotal), TokensComplete: s.TokenTotalsKnown, StartedAt: s.CreatedAt, LastActiveAt: s.LastActiveAt, QuotaSource: "measured"}
		if len(s.TurnRecords) == s.Turn && len(s.TurnRecords) > 0 {
			row.NewInputTokens, row.CachedInputTokens, row.OutputTokens = 0, 0, 0
		}
		if len(s.TurnRecords) > 0 {
			var scoreTotal float64
			for _, turn := range s.TurnRecords {
				if len(s.TurnRecords) == s.Turn {
					row.NewInputTokens += int64(turn.NewInputTokens)
					row.CachedInputTokens += int64(turn.CachedInputTokens)
					row.OutputTokens += int64(turn.OutputTokens)
				}
				if turn.Rating != nil {
					scoreTotal += float64(*turn.Rating)
					row.Ratings++
				}
			}
			if row.Ratings > 0 {
				avg := scoreTotal / float64(row.Ratings)
				row.Rating = &avg
			}
		}
		row.MeasuredDrainPoints, row.MeasuredTurns, row.MeasuredTurnsWithTokens, row.MeasuredNewInputTokens = measuredTurnMetrics(s, quotaEvents)
		row.QuotaDrainPercent, row.Unreliable = measuredTurnDrain(s.ID, quotaEvents)
		if row.QuotaDrainPercent == nil {
			row.QuotaSource = "unavailable"
		}
		rows = append(rows, row)
	}
	hostRows := hostSessionRows(calls, known, history, codexRolloutModels(opts.HomeDir))
	shareFittedDrain(hostRows)
	rows = append(rows, hostRows...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].LastActiveAt.After(rows[j].LastActiveAt) })
	models := modelTotals(rows)
	shown := rows
	if !opts.All && len(shown) > 20 {
		shown = shown[:20]
	}
	report := agentStatsReport{Days: opts.Days, Since: since, Sessions: shown, SessionsShown: len(shown), SessionsOmitted: len(rows) - len(shown), Models: models}
	if opts.JSON {
		return json.NewEncoder(w).Encode(report)
	}
	return renderAgentStatsTable(w, report)
}

func measuredTurnMetrics(session *subagent.Session, events map[string]map[int]*quotaTurnPair) (float64, int, int, int64) {
	turnTokens := make(map[int]int64, len(session.TurnRecords))
	for _, turn := range session.TurnRecords {
		turnTokens[turn.Turn] = int64(turn.NewInputTokens)
	}
	var points float64
	var measuredTurns int
	var turnsWithTokens int
	var newInput int64
	for turn, pair := range events[session.ID] {
		if pair.Before == nil || pair.After == nil {
			continue
		}
		before, after := pair.Before.Reading, pair.After.Reading
		if before.Error != "" || after.Error != "" || !before.HasCache || !after.HasCache || before.CacheAgeMS > 60000 || after.CacheAgeMS > 60000 {
			continue
		}
		delta, invalid := quotaWindowDelta(before.Windows, after.Windows)
		if invalid || delta == nil {
			continue
		}
		points += *delta
		measuredTurns++
		if tokens, ok := turnTokens[turn]; ok {
			newInput += tokens
			turnsWithTokens++
		} else if pair.After.Tokens != nil {
			newInput += int64(pair.After.Tokens.NewInputTokens)
			turnsWithTokens++
		}
	}
	return points, measuredTurns, turnsWithTokens, newInput
}

// shareFittedDrain prevents the same quota-history interval being attributed
// in full to multiple overlapping host sessions from one provider.
func shareFittedDrain(rows []agentSessionRow) {
	for i := range rows {
		if rows[i].QuotaDrainPercent == nil {
			continue
		}
		weight, peers := 0.0, 0
		for j := range rows {
			if rows[j].Provider != rows[i].Provider || rows[j].SessionID == rows[i].SessionID || rows[j].StartedAt.After(rows[i].LastActiveAt) || rows[j].LastActiveAt.Before(rows[i].StartedAt) {
				continue
			}
			peers++
			w := float64(rows[j].NewInputTokens)
			if w <= 0 {
				w = 1
			}
			weight += w
		}
		if peers == 0 {
			continue
		}
		self := float64(rows[i].NewInputTokens)
		if self <= 0 {
			self = 1
		}
		v := *rows[i].QuotaDrainPercent * self / (weight + self)
		rows[i].QuotaDrainPercent = &v
		rows[i].DrainShared = true
	}
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
	return strings.Contains(x, "5-hour") || strings.Contains(x, "5h") || strings.Contains(x, "five hour") || (strings.Contains(x, "session") && !strings.Contains(x, "weekly"))
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

func hostSessionRows(calls []telemetry.ToolCall, known map[string]bool, history []usage.QuotaHistoryEntry, codexModels map[string]string) []agentSessionRow {
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
		if row.Provider == "codex" && codexModels[row.SessionID] != "" {
			row.Model = "codex:" + codexModels[row.SessionID]
		}
		row.NewInputTokens -= row.CachedInputTokens
		if row.NewInputTokens < 0 {
			row.NewInputTokens = 0
		}
		row.QuotaDrainPercent = fittedDrain(*row, history)
		rows = append(rows, *row)
	}
	return rows
}

func codexRolloutModels(home string) map[string]string {
	models := map[string]string{}
	root := filepath.Join(home, ".codex", "sessions")
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry == nil || entry.IsDir() || !strings.HasPrefix(entry.Name(), "rollout-") || !strings.HasSuffix(entry.Name(), ".jsonl") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		var sessionID, model string
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			var record struct {
				Type    string `json:"type"`
				Payload struct {
					SessionID string `json:"session_id"`
					Model     string `json:"model"`
				} `json:"payload"`
			}
			if json.Unmarshal(scanner.Bytes(), &record) != nil {
				continue
			}
			if record.Type == "session_meta" && record.Payload.SessionID != "" {
				sessionID = record.Payload.SessionID
			}
			if record.Type == "turn_context" && record.Payload.Model != "" {
				model = record.Payload.Model
			}
		}
		if sessionID != "" && model != "" {
			models[sessionID] = model
		}
		return nil
	})
	return models
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
		if row.Model == "" {
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
		if row.Rating != nil {
			t.Ratings += row.Ratings
			if t.RatingAverage == nil {
				v := 0.0
				t.RatingAverage = &v
			}
			*t.RatingAverage += *row.Rating * float64(row.Ratings)
		}
		if row.Source == "harnez" {
			t.MeasuredDrainPoints += row.MeasuredDrainPoints
			t.MeasuredTurns += row.MeasuredTurns
			t.MeasuredTurnsWithTokens += row.MeasuredTurnsWithTokens
			t.MeasuredNewInputTokens += row.MeasuredNewInputTokens
		}
	}
	out := make([]agentModelTotals, 0, len(byModel))
	for _, t := range byModel {
		if t.Ratings > 0 {
			*t.RatingAverage /= float64(t.Ratings)
		}
		if t.MeasuredTurns >= 5 && t.MeasuredTurnsWithTokens == t.MeasuredTurns && t.MeasuredNewInputTokens > 0 {
			rate := t.MeasuredDrainPoints * 100000 / float64(t.MeasuredNewInputTokens)
			t.PointsPer100KNew = &rate
		}
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return out
}

func renderAgentStatsTable(w io.Writer, report agentStatsReport) error {
	fmt.Fprintf(w, "Agent sessions in last %d days (since %s)\n", report.Days, report.Since.Format("2006-01-02"))
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SESSION\tSOURCE\tPROVIDER\tMODEL\tTURNS\tNEW INPUT\tCACHED\tOUTPUT\t5H DRAIN\tSRC\tQUALITY")
	for _, row := range report.Sessions {
		drain := "—"
		if row.QuotaDrainPercent != nil {
			drain = fmt.Sprintf("%.1f%%", *row.QuotaDrainPercent)
		}
		if row.Unreliable {
			drain += " (unreliable)"
		}
		quality := "—"
		if row.Rating != nil {
			quality = fmt.Sprintf("%.1f/5", *row.Rating)
		}
		turns, input, cached, output := fmt.Sprint(row.Turns), fmt.Sprint(row.NewInputTokens), fmt.Sprint(row.CachedInputTokens), fmt.Sprint(row.OutputTokens)
		if !row.TurnsKnown {
			turns = "—"
		}
		if !row.TokensComplete {
			input, cached, output = "—", "—", "—"
		}
		drainSource := row.QuotaSource
		if row.DrainShared {
			drainSource += "/shared"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", row.SessionID, row.Source, row.Provider, row.Model, turns, input, cached, output, drain, drainSource, quality)
	}
	if report.SessionsOmitted > 0 {
		fmt.Fprintf(tw, "... %d older sessions omitted; pass --all to list every session.\n", report.SessionsOmitted)
	}
	fmt.Fprintln(tw, "\nPer-model totals:")
	fmt.Fprintln(tw, "MODEL\tSESSIONS (TOKEN COMPLETE)\tTURNS (KNOWN)\tNEW INPUT\tCACHED\tOUTPUT\tRATING\tMEASURED TURNS\t5H DRAIN\tPTS/100K NEW")
	for _, model := range report.Models {
		rating := "—"
		if model.RatingAverage != nil {
			rating = fmt.Sprintf("%.2f/5 (%d)", *model.RatingAverage, model.Ratings)
		}
		pointsPer100K := "—"
		if model.PointsPer100KNew != nil {
			pointsPer100K = fmt.Sprintf("%.2f", *model.PointsPer100KNew)
		}
		drain := "—"
		if model.MeasuredTurns > 0 {
			drain = fmt.Sprintf("%.1f pts", model.MeasuredDrainPoints)
		}
		fmt.Fprintf(tw, "%s\t%d/%d\t%d/%d\t%d\t%d\t%d\t%s\t%d\t%s\t%s\n", model.Model, model.SessionsWithCompleteTokens, model.Sessions, model.Turns, model.SessionsWithKnownTurns, model.NewInputTokens, model.CachedInputTokens, model.OutputTokens, rating, model.MeasuredTurns, drain, pointsPer100K)
	}
	return tw.Flush()
}
