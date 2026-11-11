package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
)

type helloProbeDriver struct {
	runs []subagent.RunOptions
}

func (d *helloProbeDriver) Run(ctx context.Context, opts subagent.RunOptions) (*subagent.TurnResult, error) {
	d.runs = append(d.runs, opts)
	return &subagent.TurnResult{
		OutputTokens: 10,
	}, nil
}

func (d *helloProbeDriver) Resume(ctx context.Context, id, prompt string, m subagent.Model) (*subagent.TurnResult, error) {
	return nil, nil
}

func (d *helloProbeDriver) Compact(ctx context.Context, id string, model subagent.Model) (*subagent.TurnResult, error) {
	return nil, nil
}

func (d *helloProbeDriver) Stop(ctx context.Context, id string) error {
	return nil
}

func (d *helloProbeDriver) Delete(ctx context.Context, id string) error {
	return nil
}

func TestAgentModelsSetFlagValidationAndExpiry(t *testing.T) {
	tempDir := t.TempDir()
	cleanupDir := subagent.SetTierOverridesStateDirForTest(tempDir)
	defer cleanupDir()

	// 1. Missing expiration boundary should return user-friendly error
	cmd := newAgentCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"models", "set", "--cheap=claude:haiku:low"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when expiration boundary is missing")
	}
	if !strings.Contains(err.Error(), "expiration boundary required") {
		t.Fatalf("expected error mentioning expiration boundary, got %v", err)
	}

	// 2. Both --until and --hours specified
	cmd = newAgentCmd()
	cmd.SetArgs([]string{"models", "set", "--until=23:59", "--hours=2", "--cheap=claude:haiku:low"})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error when both --until and --hours are specified")
	}

	// 3. Invalid model spec
	cmd = newAgentCmd()
	cmd.SetArgs([]string{"models", "set", "--hours=1", "--cheap=invalid:model:spec:xyz"})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid model spec")
	}

	// 4. Successful set with --hours and positional <cheap>/<lean>/<smart>
	cmd = newAgentCmd()
	out.Reset()
	errOut.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"models", "set", "--hours=1.5", "claude:haiku:low/claude:sonnet:low/claude:opus:high"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("models set failed: %v", err)
	}
	if !strings.Contains(out.String(), "Set tier model defaults") {
		t.Fatalf("expected confirmation output, got: %s", out.String())
	}

	// Verify persistence in state dir
	loaded, err := subagent.LoadTierOverrides()
	if err != nil {
		t.Fatalf("failed to load overrides: %v", err)
	}
	if loaded == nil || loaded.Tiers["cheap"] != "claude:haiku:low" || loaded.Tiers["lean"] != "claude:sonnet:low" || loaded.Tiers["smart"] != "claude:opus:high" {
		t.Fatalf("unexpected loaded tiers: %+v", loaded)
	}

	// 5. Verify top-level alias `harnez models set`
	root := newRootCmd()
	out.Reset()
	errOut.Reset()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"models", "set", "--hours=2", "--lean=codex:luna:low"})
	if err := root.Execute(); err != nil {
		t.Fatalf("harnez models set failed: %v", err)
	}

	loaded2, _ := subagent.LoadTierOverrides()
	if loaded2 == nil || loaded2.Tiers["lean"] != "codex:luna:low" {
		t.Fatalf("expected updated lean tier, got: %+v", loaded2)
	}
	// cheap and smart should still be preserved from partial update
	if loaded2.Tiers["cheap"] != "claude:haiku:low" || loaded2.Tiers["smart"] != "claude:opus:high" {
		t.Fatalf("expected preserved cheap/smart tiers, got: %+v", loaded2)
	}
}

func TestAgentModelsSetHelloProbe(t *testing.T) {
	tempDir := t.TempDir()
	cleanupDir := subagent.SetTierOverridesStateDirForTest(tempDir)
	defer cleanupDir()

	probeDriver := &helloProbeDriver{}
	oldDriver := agentDriver
	agentDriver = func(m subagent.Model, dir string) subagent.Driver {
		return probeDriver
	}
	defer func() { agentDriver = oldDriver }()

	cmd := newAgentCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"models", "set", "--hours=1", "--cheap=claude:haiku:med", "--hello"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("models set --hello failed: %v", err)
	}

	if !strings.Contains(out.String(), "Running hello probes") {
		t.Fatalf("expected hello probe output, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "ok") {
		t.Fatalf("expected probe status ok, got:\n%s", out.String())
	}

	// Verify driver invocation details: lowest effort and plain ContextMode
	if len(probeDriver.runs) != 1 {
		t.Fatalf("expected 1 probe run, got %d", len(probeDriver.runs))
	}
	run := probeDriver.runs[0]
	if run.ContextMode != "plain" {
		t.Fatalf("run.ContextMode = %q, want 'plain'", run.ContextMode)
	}
	if run.Prompt != "hello" {
		t.Fatalf("run.Prompt = %q, want 'hello'", run.Prompt)
	}
	if run.Model.Tier != "low" {
		t.Fatalf("run.Model.Tier = %q, want 'low' (lowest effort)", run.Model.Tier)
	}
}

func TestParseExpiry(t *testing.T) {
	now := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)

	// --hours
	exp, err := parseExpiry("", 2.5, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := now.Add(2*time.Hour + 30*time.Minute)
	if !exp.Equal(want) {
		t.Fatalf("got %v, want %v", exp, want)
	}

	// RFC3339 in future
	future := now.Add(4 * time.Hour).Format(time.RFC3339)
	expRFC, err := parseExpiry(future, 0, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !expRFC.Equal(now.Add(4 * time.Hour)) {
		t.Fatalf("got %v, want %v", expRFC, now.Add(4*time.Hour))
	}

	// RFC3339 in past
	past := now.Add(-1 * time.Hour).Format(time.RFC3339)
	if _, err := parseExpiry(past, 0, now); err == nil {
		t.Fatal("expected error for RFC3339 in the past")
	}

	// HH:MM today
	expHHMM, err := parseExpiry("15:30", 0, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expHHMM.Hour() != 15 || expHHMM.Minute() != 30 {
		t.Fatalf("got %v, want 15:30", expHHMM)
	}

	// HH:MM next day if time has passed today
	expNextDay, err := parseExpiry("08:00", 0, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantNextDay := now.AddDate(0, 0, 1)
	if expNextDay.Day() != wantNextDay.Day() || expNextDay.Hour() != 8 {
		t.Fatalf("got %v, want next day 08:00 (%v)", expNextDay, wantNextDay)
	}
}
