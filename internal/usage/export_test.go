package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBuildUsageExport_ScrubsAccountAndHostname constructs a fixture
// HistoryEntry containing a real-looking email address and a real-looking
// hostname, runs it through BuildUsageExport, and asserts by string search
// over the serialized JSON that neither raw value appears anywhere in the
// output.
func TestBuildUsageExport_ScrubsAccountAndHostname(t *testing.T) {
	rawEmail := "someone@example.com"
	rawHost := "uwes-workstation.local"

	entries := []HistoryEntry{
		{
			Hostname: rawHost,
			UsageSummary: UsageSummary{
				Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Agents: []AgentUsage{
					{
						AgentID:       "claude",
						Name:          "Claude Code",
						Installed:     true,
						Authenticated: true,
						Account:       rawEmail,
						Tokens:        &TokenBreakdown{TotalTokens: 1000},
					},
				},
			},
		},
	}

	exp := BuildUsageExport(entries, time.Now())
	if len(exp.Points) != 1 {
		t.Fatalf("expected 1 exported point, got %d", len(exp.Points))
	}

	data, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(data)

	if strings.Contains(out, rawEmail) {
		t.Fatalf("raw email %q leaked into export JSON: %s", rawEmail, out)
	}
	if strings.Contains(out, "someone") {
		t.Fatalf("raw email local-part fragment leaked into export JSON: %s", out)
	}
	if strings.Contains(out, rawHost) {
		t.Fatalf("raw hostname %q leaked into export JSON: %s", rawHost, out)
	}

	got := exp.Points[0]
	if got.Hostname != "host-1" {
		t.Errorf("Hostname = %q, want anonymized label %q", got.Hostname, "host-1")
	}
	if got.Account == rawEmail {
		t.Errorf("Account was not masked at all: %q", got.Account)
	}
	if got.TotalTokens != 1000 {
		t.Errorf("TotalTokens = %d, want 1000", got.TotalTokens)
	}
}

// TestBuildUsageExport_DropsSourcesAndDetails asserts that a filesystem
// path stashed in AgentUsage.Sources or .Details never survives into the
// export, since both fields are unstructured free text with no safe
// automatic scrubbing.
func TestBuildUsageExport_DropsSourcesAndDetails(t *testing.T) {
	rawPath := "/home/testuser/.claude/config.json"
	entries := []HistoryEntry{
		{
			Hostname: "host1",
			UsageSummary: UsageSummary{
				Timestamp: time.Now(),
				Agents: []AgentUsage{
					{
						AgentID: "codex",
						Sources: []string{rawPath},
						Details: map[string]string{"config_path": rawPath},
					},
				},
			},
		},
	}
	exp := BuildUsageExport(entries, time.Now())
	data, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), rawPath) {
		t.Fatalf("raw path from Sources/Details leaked into export JSON: %s", data)
	}
}

// TestBuildUsageExport_StableHostLabels asserts that the same real
// hostname maps to the same anonymized label across multiple entries in
// one export, and that a different real hostname gets a distinct label,
// so per-machine trends stay distinguishable without leaking the real name.
func TestBuildUsageExport_StableHostLabels(t *testing.T) {
	entries := []HistoryEntry{
		{Hostname: "alice-laptop", UsageSummary: UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{AgentID: "claude"}}}},
		{Hostname: "bob-desktop", UsageSummary: UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{AgentID: "claude"}}}},
		{Hostname: "alice-laptop", UsageSummary: UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{AgentID: "claude"}}}},
	}
	exp := BuildUsageExport(entries, time.Now())
	if len(exp.Points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(exp.Points))
	}
	if exp.Points[0].Hostname != exp.Points[2].Hostname {
		t.Errorf("same real hostname got different labels: %q vs %q", exp.Points[0].Hostname, exp.Points[2].Hostname)
	}
	if exp.Points[0].Hostname == exp.Points[1].Hostname {
		t.Errorf("different real hostnames got the same label: %q", exp.Points[0].Hostname)
	}
	data, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "alice-laptop") || strings.Contains(string(data), "bob-desktop") {
		t.Fatalf("raw hostname leaked into export JSON: %s", data)
	}
}

func TestExportHistory(t *testing.T) {
	dir := t.TempDir()
	entry := HistoryEntry{
		Hostname: "Some.Real.Host",
		UsageSummary: UsageSummary{
			Timestamp: time.Now(),
			Agents: []AgentUsage{
				{AgentID: "claude", Account: "user@example.com", Tokens: &TokenBreakdown{TotalTokens: 42}},
			},
		},
	}
	line, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "host1.jsonl"), append(line, '\n'), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	exp, err := ExportHistory(dir, time.Now())
	if err != nil {
		t.Fatalf("ExportHistory: %v", err)
	}
	if len(exp.Points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(exp.Points))
	}
	data, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "user@example.com") {
		t.Fatalf("raw email leaked into export JSON: %s", data)
	}
	if strings.Contains(string(data), "Some.Real.Host") {
		t.Fatalf("raw hostname leaked into export JSON: %s", data)
	}
}
