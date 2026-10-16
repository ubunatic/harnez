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
	if !strings.HasPrefix(got.Hostname, "host-") || len(got.Hostname) != len("host-")+8 {
		t.Errorf("Hostname = %q, want HMAC anonymized label with prefix 'host-' and 8 hex chars", got.Hostname)
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

// TestBuildUsageExport_StableHostLabels asserts:
// - Stable across two export invocations (with the same key)
// - Case-insensitive hostnames result in the same label (e.g. "My-Host" vs "my-host")
// - Distinct hostnames produce distinct labels (e.g. "t14" vs "t14s")
// - Real hostname never appears in the output
func TestBuildUsageExport_StableHostLabels(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	entries1 := []HistoryEntry{
		{Hostname: "t14", UsageSummary: UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{AgentID: "claude"}}}},
		{Hostname: "t14s", UsageSummary: UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{AgentID: "claude"}}}},
		{Hostname: "T14", UsageSummary: UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{AgentID: "claude"}}}},
	}
	exp1 := BuildUsageExport(entries1, time.Now())
	if len(exp1.Points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(exp1.Points))
	}
	if exp1.Points[0].Hostname != exp1.Points[2].Hostname {
		t.Errorf("case-insensitive hostname mismatch: %q vs %q", exp1.Points[0].Hostname, exp1.Points[2].Hostname)
	}
	if exp1.Points[0].Hostname == exp1.Points[1].Hostname {
		t.Errorf("t14 and t14s got the same label: %q", exp1.Points[0].Hostname)
	}

	// Second export invocation should produce identical host labels
	entries2 := []HistoryEntry{
		{Hostname: "t14", UsageSummary: UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{AgentID: "claude"}}}},
	}
	exp2 := BuildUsageExport(entries2, time.Now())
	if exp2.Points[0].Hostname != exp1.Points[0].Hostname {
		t.Errorf("export label not stable across calls: %q vs %q", exp2.Points[0].Hostname, exp1.Points[0].Hostname)
	}

	data, err := json.Marshal(exp1)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "t14") || strings.Contains(string(data), "t14s") {
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

// An export without usage points writes "points": [], not null (issue 735).
func TestBuildUsageExport_NoPointsIsEmptyArray(t *testing.T) {
	data, err := json.Marshal(BuildUsageExport(nil, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"points":[]`) {
		t.Fatalf("export = %s, want \"points\":[]", data)
	}
}
