package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	internalusage "ubunatic.com/harnez/internal/usage"
	sharedusage "ubunatic.com/harnez/usage"
)

func TestUsageNoSharedFlagByteIdenticalToExplicitFalse(t *testing.T) {
	summary := internalusage.UsageSummary{
		Timestamp: time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC),
		Agents:    []internalusage.AgentUsage{{AgentID: "claude", Name: "Claude Code", Installed: false}},
	}
	legacyJSON, err := internalusage.RenderJSON(summary)
	if err != nil {
		t.Fatal(err)
	}
	sharedJSON, err := renderUsageOutput(summary, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte(legacyJSON), []byte(sharedJSON)) {
		t.Fatalf("JSON renderer output changed: legacy=%s shared=%s", legacyJSON, sharedJSON)
	}
	legacyText := internalusage.RenderText(summary)
	sharedText, err := renderUsageOutput(summary, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte(legacyText), []byte(sharedText)) {
		t.Fatalf("human renderer output changed: legacy=%q shared=%q", legacyText, sharedText)
	}
}

func TestSharedUsageRejectsHostAndProject(t *testing.T) {
	for _, args := range [][]string{
		{"usage", "--shared", "--host", "example.invalid"},
		{"usage", "--shared", "--project", "."},
	} {
		root := newRootCmd()
		root.SetArgs(args)
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "--shared cannot be used") {
			t.Errorf("execute %v error = %v, want clear --shared incompatibility", args, err)
		}
	}
}

func TestUsageSnapshotAdapterPreservesRendererFieldsAndSchema(t *testing.T) {
	resetAt := time.Now().UTC().Add(time.Hour)
	agent := internalusage.AgentUsage{
		AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true,
		PlanTier: "pro", ActiveModel: "sonnet", Details: map[string]string{"total_sessions": "12"},
		Sources: []string{"~/.claude/stats-cache.json"}, QuotaFetchError: "temporary failure",
		LastRefreshed: time.Now().UTC(), Tokens: &internalusage.TokenBreakdown{InputTokens: 11, TotalTokens: 15},
		Session: &internalusage.QuotaWindow{Name: "5h", UsedPercent: 25, RemainingPercent: 75, ResetAt: &resetAt, DurationLeft: time.Minute},
	}
	public := publicSnapshot(agent)
	got := agentUsageFromSnapshot(public)
	if got.AgentID != agent.AgentID || got.Details["total_sessions"] != "12" || got.QuotaFetchError != agent.QuotaFetchError || got.Tokens.TotalTokens != 15 || got.Session.DurationLeft != time.Minute {
		t.Fatalf("mapped AgentUsage lost renderer fields: %+v", got)
	}
	encoded, err := json.Marshal(summaryFromSnapshots([]sharedusage.Snapshot{public}))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"timestamp"`, `"agents"`, `"agent_id"`, `"quota_fetch_error"`, `"details"`, `"sources"`} {
		if !bytes.Contains(encoded, []byte(field)) {
			t.Errorf("existing JSON field %s missing from shared output: %s", field, encoded)
		}
	}
	if bytes.Contains(encoded, []byte(filepath.Join(string(os.PathSeparator), "home"))) {
		t.Errorf("shared JSON unexpectedly contains an absolute local path: %s", encoded)
	}
}

func TestRealUsageCollectorUsesTemporaryHomeAndRealProvider(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	snapshot, err := (realUsageCollector{}).CollectUsage(context.Background(), sharedusage.ProviderClaude)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ProviderID != sharedusage.ProviderClaude || snapshot.Usage.Installed {
		t.Fatalf("temporary home collector result = %+v", snapshot)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("collector created provider data in temporary home: %v", err)
	}
}
