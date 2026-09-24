package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"ubunatic.com/harnez/internal/agymeter"
	"ubunatic.com/harnez/internal/telemetry"
)

type statsCallRow struct {
	Time            time.Time `json:"time"`
	Kind            string    `json:"kind"`
	PromptID        string    `json:"prompt_id,omitempty"`
	Model           string    `json:"model,omitempty"`
	PromptTokens    int64     `json:"prompt_tokens,omitempty"`
	CachedTokens    int64     `json:"cached_tokens,omitempty"`
	CandidateTokens int64     `json:"candidate_tokens,omitempty"`
	ThoughtTokens   int64     `json:"thought_tokens,omitempty"`
	TotalTokens     int64     `json:"total_tokens,omitempty"`
	Tool            string    `json:"tool,omitempty"`
	CallType        string    `json:"call_type,omitempty"`
	ToolTokens      *int64    `json:"tool_tokens_estimate,omitempty"`
	Sources         string    `json:"sources,omitempty"`
}

type statsPromptTotal struct {
	PromptID        string `json:"prompt_id"`
	Requests        int    `json:"requests"`
	PromptTokens    int64  `json:"prompt_tokens"`
	CachedTokens    int64  `json:"cached_tokens"`
	CandidateTokens int64  `json:"candidate_tokens"`
	ThoughtTokens   int64  `json:"thought_tokens"`
	TotalTokens     int64  `json:"total_tokens"`
	ToolCalls       int    `json:"tool_calls"`
}

type statsCallsReport struct {
	Session string             `json:"session"`
	Rows    []statsCallRow     `json:"rows"`
	Prompts []statsPromptTotal `json:"prompts"`
}

func runStatsCalls(w io.Writer, opts statsOptions) error {
	if opts.Session == "" {
		return fmt.Errorf("stats --calls requires --session <id>")
	}
	dbPath := opts.DBPath
	if dbPath == "" {
		var err error
		dbPath, err = telemetry.DefaultDBPath()
		if err != nil {
			return err
		}
	}
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return fmt.Errorf("stats --calls: open telemetry: %w", err)
	}
	defer db.Close()
	tools, err := db.Query(telemetry.Filter{SessionID: opts.Session})
	if err != nil {
		return fmt.Errorf("stats --calls: query tool calls: %w", err)
	}
	meterPath := opts.MeterPath
	if meterPath == "" {
		home, e := os.UserHomeDir()
		if e != nil {
			return e
		}
		meterPath = filepath.Join(home, ".harnez", "agymeter", "usage.jsonl")
	}
	usageRows, err := agymeter.ReadUsageRecords(meterPath, opts.Session)
	if err != nil {
		return fmt.Errorf("stats --calls: read meter rows: %w", err)
	}
	report := assembleStatsCalls(opts.Session, usageRows, tools)
	if opts.JSON {
		return json.NewEncoder(w).Encode(report)
	}
	return renderStatsCalls(w, report)
}

func assembleStatsCalls(session string, usageRows []agymeter.Record, tools []telemetry.ToolCall) statsCallsReport {
	report := statsCallsReport{Session: session}
	groups := map[string]*statsPromptTotal{}
	for _, u := range usageRows {
		row := statsCallRow{Time: u.Time, Kind: "model", PromptID: u.PromptID, Model: u.Model, PromptTokens: u.Prompt, CachedTokens: u.Cached, CandidateTokens: u.Candidates, ThoughtTokens: u.Thoughts, TotalTokens: u.Total}
		report.Rows = append(report.Rows, row)
		if u.PromptID != "" {
			g := groups[u.PromptID]
			if g == nil {
				g = &statsPromptTotal{PromptID: u.PromptID}
				groups[u.PromptID] = g
			}
			g.Requests++
			g.PromptTokens += u.Prompt
			g.CachedTokens += u.Cached
			g.CandidateTokens += u.Candidates
			g.ThoughtTokens += u.Thoughts
			g.TotalTokens += u.Total
		}
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].CreatedAt.Before(tools[j].CreatedAt) })
	paired := make(map[int]int)
	usedExec := make(map[int]bool)
	for i, hook := range tools {
		if hook.CallType != "hook:prep" || !strings.Contains(hook.Note, "agy-route=exec") {
			continue
		}
		best, distance := -1, time.Duration(1<<63-1)
		for j, execRow := range tools {
			if usedExec[j] || !isExecTelemetry(execRow) || execRow.CreatedAt.Before(hook.CreatedAt) {
				continue
			}
			d := execRow.CreatedAt.Sub(hook.CreatedAt)
			if d < distance {
				best, distance = j, d
			}
		}
		if best >= 0 {
			paired[i] = best
			usedExec[best] = true
		}
	}
	for i, tc := range tools {
		if _, duplicate := paired[i]; duplicate {
			continue
		}
		row := statsCallRow{Time: tc.CreatedAt, Kind: "tool", Tool: tc.ToolName, CallType: tc.CallType, ToolTokens: tc.ActualTokens, Sources: "telemetry"}
		if hasPairedHook(i, paired) {
			row.Sources = "hook+exec"
		}
		nearest := nearestPrompt(tc.CreatedAt, usageRows)
		if nearest != "" {
			row.PromptID = nearest
			if g := groups[nearest]; g != nil {
				g.ToolCalls++
			}
		}
		report.Rows = append(report.Rows, row)
	}
	sort.SliceStable(report.Rows, func(i, j int) bool { return report.Rows[i].Time.Before(report.Rows[j].Time) })
	for _, g := range groups {
		report.Prompts = append(report.Prompts, *g)
	}
	sort.Slice(report.Prompts, func(i, j int) bool { return report.Prompts[i].PromptID < report.Prompts[j].PromptID })
	return report
}

func hasPairedHook(execIndex int, paired map[int]int) bool {
	for _, exec := range paired {
		if exec == execIndex {
			return true
		}
	}
	return false
}
func isExecTelemetry(row telemetry.ToolCall) bool {
	return row.CallType == "shell" || row.CallType == "shell-timeout" || row.CallType == telemetry.ExpectedFailureCallType
}
func nearestPrompt(at time.Time, usages []agymeter.Record) string {
	var best string
	var delta time.Duration = 1<<63 - 1
	for _, u := range usages {
		d := at.Sub(u.Time)
		if d < 0 {
			d = -d
		}
		if d < delta {
			delta = d
			best = u.PromptID
		}
	}
	return best
}

func renderStatsCalls(w io.Writer, report statsCallsReport) error {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "TIME\tTYPE\tMODEL\tPROMPT\tCACHED\tCANDIDATES\tTHOUGHTS\tTOTAL\tTOOL\tTOOL_TOKENS\tSOURCE")
	for _, r := range report.Rows {
		at := r.Time.Local().Format("15:04:05")
		if r.Kind == "model" {
			fmt.Fprintf(tw, "%s\tmodel\t%s\t%d\t%d\t%d\t%d\t%d\t\tprovider\n", at, r.Model, r.PromptTokens, r.CachedTokens, r.CandidateTokens, r.ThoughtTokens, r.TotalTokens)
		} else {
			token := ""
			if r.ToolTokens != nil {
				token = fmt.Sprint(*r.ToolTokens)
			}
			fmt.Fprintf(tw, "%s\ttool\t\t\t\t\t\t%s\t%s\t%s (%s)\n", at, r.Tool, token, r.CallType, r.Sources)
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(w, "\nPROMPT TOTALS")
	for _, p := range report.Prompts {
		fmt.Fprintf(w, "%s  requests=%d input=%d cached=%d candidates=%d thoughts=%d total=%d tool_calls=%d\n", p.PromptID, p.Requests, p.PromptTokens, p.CachedTokens, p.CandidateTokens, p.ThoughtTokens, p.TotalTokens, p.ToolCalls)
	}
	return nil
}
