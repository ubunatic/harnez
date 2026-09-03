// privacy_export_test.go covers issue 204's privacy-levels pass for
// internal/usage: LevelInternal's regex-scrub-in-place of Sources/Details
// (as opposed to LevelPublic's "drop the field" default), and LevelRaw's
// pass-through.
package usage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/privacy"
)

// TestBuildUsageExportLevel_Internal_ScrubsSourcesAndDetails asserts
// LevelInternal keeps Sources/Details present (unlike LevelPublic, which
// drops them) but with sensitive substrings redacted in place.
func TestBuildUsageExportLevel_Internal_ScrubsSourcesAndDetails(t *testing.T) {
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

	exp := BuildUsageExportLevel(entries, time.Now(), privacy.LevelInternal)
	if len(exp.Points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(exp.Points))
	}
	p := exp.Points[0]
	if len(p.Sources) != 1 || p.Sources[0] == "" {
		t.Fatalf("LevelInternal dropped Sources entirely, want it scrubbed-but-present: %v", p.Sources)
	}
	if strings.Contains(p.Sources[0], "/home/testuser") {
		t.Errorf("Sources entry not scrubbed: %q", p.Sources[0])
	}
	if got, ok := p.Details["config_path"]; !ok || strings.Contains(got, "/home/testuser") {
		t.Errorf("Details[config_path] not scrubbed: %q (present=%v)", got, ok)
	}

	data, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), rawPath) {
		t.Fatalf("raw path leaked into export JSON: %s", data)
	}
}

// TestBuildUsageExportLevel_Raw_PassesThroughUnchanged asserts LevelRaw
// exports Sources/Details verbatim.
func TestBuildUsageExportLevel_Raw_PassesThroughUnchanged(t *testing.T) {
	rawPath := "/home/testuser/.claude/config.json"
	entries := []HistoryEntry{
		{
			Hostname: "host1",
			UsageSummary: UsageSummary{
				Timestamp: time.Now(),
				Agents: []AgentUsage{
					{AgentID: "codex", Sources: []string{rawPath}, Details: map[string]string{"config_path": rawPath}},
				},
			},
		},
	}

	exp := BuildUsageExportLevel(entries, time.Now(), privacy.LevelRaw)
	p := exp.Points[0]
	if len(p.Sources) != 1 || p.Sources[0] != rawPath {
		t.Errorf("LevelRaw Sources = %v, want unchanged [%q]", p.Sources, rawPath)
	}
	if p.Details["config_path"] != rawPath {
		t.Errorf("LevelRaw Details[config_path] = %q, want unchanged %q", p.Details["config_path"], rawPath)
	}
}

// TestBuildUsageExportLevel_Public_MatchesBuildUsageExport asserts the new
// level-aware entry point behaves identically to the original
// BuildUsageExport at the default level.
func TestBuildUsageExportLevel_Public_MatchesBuildUsageExport(t *testing.T) {
	entries := []HistoryEntry{
		{Hostname: "host1", UsageSummary: UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{
			{AgentID: "codex", Sources: []string{"/home/testuser/x"}, Details: map[string]string{"k": "v"}},
		}}},
	}
	now := time.Now()
	want := BuildUsageExport(entries, now)
	got := BuildUsageExportLevel(entries, now, privacy.LevelPublic)

	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(wantJSON) != string(gotJSON) {
		t.Errorf("BuildUsageExportLevel(LevelPublic) differs from BuildUsageExport:\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}
