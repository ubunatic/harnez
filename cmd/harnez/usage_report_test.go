package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

// reportFixture writes one Claude transcript call and two weekly quota samples
// under a temp HOME, relative to now, so `harnez usage history report` reads
// them. It returns nothing: the command reads HOME and XDG_DATA_HOME.
func reportFixture(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	now := time.Now().UTC()
	at := now.Add(-time.Hour).Format(time.RFC3339Nano)
	transcript := `{"type":"assistant","timestamp":"` + at + `","message":{"id":"m1","model":"claude-sonnet-5-5","usage":{"input_tokens":1000000,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}` + "\n"
	path := filepath.Join(home, ".claude", "projects", "-work-proj", "s1.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(transcript), 0o600); err != nil {
		t.Fatal(err)
	}
	reset := now.Add(6 * 24 * time.Hour)
	entries := []usage.QuotaHistoryEntry{
		{Timestamp: now.Add(-3 * time.Hour), Agent: "claude", Window: usage.ClaudeWeeklyWindowName, UsedPercent: 10, ResetAt: &reset},
		{Timestamp: now.Add(-30 * time.Minute), Agent: "claude", Window: usage.ClaudeWeeklyWindowName, UsedPercent: 11, ResetAt: &reset},
	}
	if err := usage.AppendQuotaHistory(usage.HistoryDir(home), entries); err != nil {
		t.Fatal(err)
	}
}

func runReport(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newUsageHistoryReportCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// The JSON report carries the session, the attributed share and the window
// line; --since and --session narrow the rows.
func TestUsageReportCLIJSONAndSession(t *testing.T) {
	reportFixture(t)
	out, err := runReport(t, "--since", "7d", "--json")
	if err != nil {
		t.Fatalf("report: %v\n%s", err, out)
	}
	var rep struct {
		Sessions []struct {
			Session string   `json:"session"`
			Quota   *float64 `json:"quota_pct"`
		} `json:"sessions"`
		Window *struct {
			RisePct float64 `json:"weekly_rise_pct"`
		} `json:"window"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if len(rep.Sessions) != 1 || rep.Sessions[0].Session != "s1" || rep.Sessions[0].Quota == nil {
		t.Fatalf("sessions = %+v, want s1 with a quota share", rep.Sessions)
	}
	if rep.Window == nil || rep.Window.RisePct != 1 {
		t.Fatalf("window = %+v, want the 1 pp weekly rise", rep.Window)
	}

	out, err = runReport(t, "--since", "7d", "--session", "other")
	if err != nil {
		t.Fatalf("report --session: %v\n%s", err, out)
	}
	if strings.Contains(out, "s1 ") {
		t.Fatalf("--session other still shows s1:\n%s", out)
	}
}

// --window weekly takes the newest weekly window as the range.
func TestUsageReportCLIWindowWeekly(t *testing.T) {
	reportFixture(t)
	out, err := runReport(t, "--window", "weekly")
	if err != nil {
		t.Fatalf("report --window weekly: %v\n%s", err, out)
	}
	if !strings.Contains(out, "window (all local calls") {
		t.Fatalf("no window line:\n%s", out)
	}
}

// --window and --since are exclusive.
func TestUsageReportCLIWindowAndSinceExclusive(t *testing.T) {
	reportFixture(t)
	if _, err := runReport(t, "--window", "weekly", "--since", "7d"); err == nil {
		t.Fatal("--window weekly --since 7d: no error, want the exclusive-flags error")
	}
}
