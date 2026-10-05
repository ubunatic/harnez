package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagestore"
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
	cacheHome := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dataHome := filepath.Join(home, ".local", "share")
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	now := time.Now()
	reset := now.Add(time.Hour)
	cache := liveQuotaFixture{FetchedAt: now.Add(-4 * time.Minute), Payload: agyQuotaFixture{ModelGroups: []agyModelGroupFixture{{Name: "Gemini Models", Windows: []usage.QuotaWindow{{Name: "weekly", RemainingPercent: 0, ResetAt: &reset}, {Name: "5h", RemainingPercent: 0, ResetAt: &reset}}}}}}
	data, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	quotaCacheDir := filepath.Join(cacheHome, "harnez")
	if err := os.MkdirAll(quotaCacheDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(quotaCacheDir, "quota-cache-agy.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := usagestore.Open(filepath.Join(dataHome, "harnez", "telemetry.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteCurrent(ctx, cache.FetchedAt, []usagestore.Window{
		{Provider: "agy", Pool: "Gemini Models", Key: "weekly", Name: "weekly", Source: "agy-meter", Freshness: "fresh", UsedFraction: 1, ResetAt: &reset, ObservedAt: cache.FetchedAt},
		{Provider: "agy", Pool: "Gemini Models", Key: "five_hour", Name: "5h", Source: "agy-meter", Freshness: "fresh", UsedFraction: 1, ResetAt: &reset, ObservedAt: cache.FetchedAt},
	}); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"models"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "AVAILABILITY") || !strings.Contains(out.String(), "exhausted (resets ") || !strings.Contains(out.String(), "unknown") {
		t.Fatalf("models output lacks quota marker/age or unknown markers: %q", out.String())
	}
}

func TestPiProviderHasBatchDriver(t *testing.T) {
	model, err := subagent.ResolveModel("pi:openai/gpt-4o:high")
	if err != nil {
		t.Fatal(err)
	}
	driver := agentDriver(model, t.TempDir())
	if _, ok := driver.(subagent.PiDriver); !ok {
		t.Fatalf("agentDriver(%#v) = %T, want PiDriver", model, driver)
	}
	entries := subagent.ModelEntriesWithDriver([]subagent.ModelEntry{{Spec: model.Spec(), Model: model}}, func(m subagent.Model) subagent.Driver {
		return agentDriver(m, ".")
	})
	if len(entries) != 1 || !entries[0].Batch {
		t.Fatalf("Pi model batch capability = %#v", entries)
	}
}

func TestAgentModelsShowsBatchCapability(t *testing.T) {
	old := agentDriver
	agentDriver = func(m subagent.Model, _ string) subagent.Driver {
		if m.Provider == "fake" {
			return subagent.UnsupportedDriver{Provider: m.Provider}
		}
		return testBatchAgentDriver{}
	}
	defer func() { agentDriver = old }()

	entries := subagent.ModelEntriesWithDriver([]subagent.ModelEntry{
		{Spec: "fake:chat:low", Model: subagent.Model{Provider: "fake", Name: "chat"}},
		{Spec: "codex:luna:low", Model: subagent.Model{Provider: "codex", Name: "luna"}},
	}, func(m subagent.Model) subagent.Driver { return agentDriver(m, ".") })
	if entries[0].Batch || !entries[1].Batch {
		t.Fatalf("batch capability entries = %#v", entries)
	}
	var out strings.Builder
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"models"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "(interactive only)") {
		t.Fatalf("existing providers unexpectedly marked interactive-only: %q", out.String())
	}

	var jsonOut strings.Builder
	cmd = newAgentCmd()
	cmd.SetOut(&jsonOut)
	cmd.SetArgs([]string{"models", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var models []subagent.ModelEntry
	if err := json.Unmarshal([]byte(jsonOut.String()), &models); err != nil {
		t.Fatal(err)
	}
	if len(models) == 0 || !models[0].Batch {
		t.Fatalf("JSON models lack batch capability: %q", jsonOut.String())
	}
}

type testBatchAgentDriver struct{}

func (testBatchAgentDriver) Run(context.Context, subagent.RunOptions) (*subagent.TurnResult, error) {
	return nil, nil
}
func (testBatchAgentDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	return nil, nil
}
func (testBatchAgentDriver) Compact(context.Context, string) (*subagent.TurnResult, error) {
	return nil, nil
}
func (testBatchAgentDriver) Stop(context.Context, string) error   { return nil }
func (testBatchAgentDriver) Delete(context.Context, string) error { return nil }

func TestAgentModelsShowsStaleQuotaAge(t *testing.T) {
	cacheHome := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dataHome := filepath.Join(home, ".local", "share")
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	now := time.Now()
	reset := now.Add(time.Hour)
	cache := liveQuotaFixture{FetchedAt: now.Add(-28 * 24 * time.Hour), Payload: agyQuotaFixture{ModelGroups: []agyModelGroupFixture{{Name: "Gemini Models", Windows: []usage.QuotaWindow{{Name: "weekly", RemainingPercent: 0, ResetAt: &reset}}}}}}
	data, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	quotaCacheDir := filepath.Join(cacheHome, "harnez")
	if err := os.MkdirAll(quotaCacheDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(quotaCacheDir, "quota-cache-agy.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := usagestore.Open(filepath.Join(dataHome, "harnez", "telemetry.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteCurrent(ctx, cache.FetchedAt, []usagestore.Window{
		{Provider: "agy", Pool: "Gemini Models", Key: "weekly", Name: "weekly", Source: "agy-meter", Freshness: "stale", UsedFraction: 1, ResetAt: &reset, ObservedAt: cache.FetchedAt},
	}); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"models"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "exhausted (resets ") || !strings.Contains(out.String(), "unknown") {
		t.Fatalf("models output lacks stale age or unknown for missing provider data: %q", out.String())
	}
}

type liveQuotaFixture struct {
	FetchedAt time.Time       `json:"fetched_at"`
	Payload   agyQuotaFixture `json:"payload"`
}

type agyQuotaFixture struct {
	ModelGroups []agyModelGroupFixture `json:"model_groups"`
}

type agyModelGroupFixture struct {
	Name    string              `json:"name"`
	Windows []usage.QuotaWindow `json:"windows"`
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

func TestAgentStartOnlyRejectsActiveExhaustion(t *testing.T) {
	model := subagent.Model{Provider: "agy", Name: "gemini-3.8-flash"}
	active := func(string, string) usage.ProviderQuotaAvailability {
		return usage.ProviderQuotaAvailability{State: "exhausted", Exhausted: true, Age: 36 * time.Minute, ResetIn: 105 * time.Minute}
	}
	if err := rejectExhaustedQuota("agy:flash38", model, false, active); err == nil {
		t.Fatal("stale snapshot with a future reset did not block agent start")
	}
	pastReset := func(string, string) usage.ProviderQuotaAvailability {
		return usage.ProviderQuotaAvailability{State: "stale", Age: 36 * time.Minute}
	}
	if err := rejectExhaustedQuota("agy:flash38", model, false, pastReset); err != nil {
		t.Fatalf("stale snapshot with a past reset blocked agent start: %v", err)
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
