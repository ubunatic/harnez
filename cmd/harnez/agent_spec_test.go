package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
)

func TestAgentDefaultModelIsMarkedOnce(t *testing.T) {
	var out strings.Builder
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"models"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "(default)") != 1 {
		t.Fatalf("models output=%q", out.String())
	}
}

func TestAgentModelsShowsCachedAvailabilityAndAge(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	now := time.Now()
	reset := now.Add(time.Hour)
	if err := usage.WriteAgentSnapshot(usage.StateDir(""), "agy", usage.AgentUsage{AgentID: "agy", ModelGroups: []usage.ModelGroup{{Name: "Gemini Models", Windows: []usage.QuotaWindow{{Name: "weekly", RemainingPercent: 0, ResetAt: &reset}, {Name: "5h", RemainingPercent: 0, ResetAt: &reset}}}}}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := usage.ReadAgentSnapshot(usage.StateDir(""), "agy")
	if err != nil || snapshot == nil {
		t.Fatalf("read seeded snapshot: snapshot=%v err=%v", snapshot, err)
	}
	snapshot.FetchedAt = now.Add(-4 * time.Minute)
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(usage.StateDir(""), "agy.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"models"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "AVAILABILITY") || !strings.Contains(out.String(), "exhausted (4m)") || !strings.Contains(out.String(), "unknown") {
		t.Fatalf("models output lacks quota marker/age or unknown markers: %q", out.String())
	}
}

func TestAgentStartRejectsKnownExhaustedQuotaAndNamesOverrideAndAlternatives(t *testing.T) {
	storeCalled := false
	availability := func(provider, model string) usage.ProviderQuotaAvailability {
		if provider == "agy" {
			return usage.ProviderQuotaAvailability{State: "exhausted", Age: 4 * time.Minute}
		}
		return usage.ProviderQuotaAvailability{State: "available"}
	}
	cmd := newAgentCmd()
	cmd.SetArgs([]string{"start", "--model", "agy:flash38", "task"})
	err := runStart(cmd, agentDeps{
		store:        func() (*subagent.FileSessionStore, error) { storeCalled = true; return nil, nil },
		availability: availability,
	}, startRequest{Prompt: "task", ModelSpec: "agy:flash38", Dir: ".", StreamMode: streamFull})
	if err == nil || !strings.Contains(err.Error(), "--allow-exhausted-quota") || !strings.Contains(err.Error(), "available cheaper alternatives:") {
		t.Fatalf("runStart error = %v, want override and alternatives", err)
	}
	if storeCalled {
		t.Fatal("exhausted provider was rejected only after session store access")
	}
	if err := rejectExhaustedQuota("agy:flash38", subagent.Model{Provider: "agy", Name: "gemini-3.8-flash"}, true, availability); err != nil {
		t.Fatalf("explicit quota override rejected: %v", err)
	}
	if got := quotaAlternatives(subagent.Model{Provider: "agy", Name: "gemini-3.8-flash"}, availability); len(got) == 0 || len(got) > 3 {
		t.Fatalf("available cheaper alternatives = %v", got)
	}
}

func TestAgentDefaultLiteralIsNotShadowed(t *testing.T) {
	root := filepath.Join("..", "..")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(data), "codex:luna:low") {
			t.Errorf("default literal shadowed in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAgentStartDefaultModelLine(t *testing.T) {
	d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "thread"}}, {ev: msg("CONFIRM: ready")}, {ev: msg("done")}}}
	out := runScripted(t, d, "start", "task")
	if !strings.Contains(out, "model: codex:luna:low (default)") {
		t.Fatalf("output=%q", out)
	}
	d = &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "thread"}}, {ev: msg("CONFIRM: ready")}, {ev: msg("done")}}}
	out = runScripted(t, d, "start", "--model", "codex:luna", "task")
	if strings.Contains(out, "(default)") {
		t.Fatalf("explicit output=%q", out)
	}
}
