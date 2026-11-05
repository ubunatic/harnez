package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/agent"
	"ubunatic.com/harnez/internal/modelblock"
	"ubunatic.com/harnez/internal/subagent"
)

// isolateModelState keeps tests away from the real keyring and ~/.harnez state.
func isolateModelState(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func modelsJSON(t *testing.T) []agent.ModelRow {
	t.Helper()
	cmd := newModelsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--json", "--offline"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("models --json: %v\n%s", err, out.String())
	}
	var rows []agent.ModelRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	return rows
}

func TestModelsJSONCarriesQuotaStartableAndEntitlement(t *testing.T) {
	isolateModelState(t)
	rows := modelsJSON(t)
	if len(rows) == 0 {
		t.Fatal("no rows")
	}
	for _, r := range rows {
		if r.Spec == "" || r.Provider == "" || r.ModelName == "" || r.Model.Name != r.ModelName || r.Quota.State == "" || r.Quota.Scope == "" || r.Entitlement.State == "" {
			t.Fatalf("incomplete row: %+v", r)
		}
		if r.Quota.State == "unknown" && r.Entitlement.State != "denied" && !r.Startable {
			t.Fatalf("unknown quota must stay startable: %+v", r)
		}
		if r.Entitlement.State == "unverified" && r.Entitlement.Source == "" {
			t.Fatalf("unverified entitlement needs a source: %+v", r)
		}
	}
}

// Ticket 983 promise: the `model` key stays the object consumers (horg) read.
func TestModelsJSONKeepsLegacyModelObject(t *testing.T) {
	isolateModelState(t)
	cmd := newModelsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var raw []map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil || len(raw) == 0 {
		t.Fatalf("decode: %v", err)
	}
	for _, row := range raw {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(row["model"], &obj); err != nil {
			t.Fatalf("model is not an object: %s", row["model"])
		}
		for _, k := range []string{"Provider", "Name", "Tier", "Effort"} {
			if _, ok := obj[k]; !ok {
				t.Fatalf("model object lacks %q: %s", k, row["model"])
			}
		}
		var name string
		if err := json.Unmarshal(row["model_name"], &name); err != nil || name == "" {
			t.Fatalf("model_name = %s", row["model_name"])
		}
		if _, ok := row["model_detail"]; ok {
			t.Fatal("model_detail must not exist")
		}
	}
}

func TestModelsBlockUnblockRoundTripShowsInJSON(t *testing.T) {
	isolateModelState(t)
	target := modelsJSON(t)[0]
	var out bytes.Buffer
	if err := runModelsBlock(&out, modelblock.Path(), target.Spec, "2h", "429 seen", time.Now()); err != nil {
		t.Fatal(err)
	}
	var blocked agent.ModelRow
	for _, r := range modelsJSON(t) {
		if r.Provider == target.Provider && r.ModelName == target.ModelName && r.Spec == target.Spec {
			blocked = r
		}
	}
	if blocked.Startable || blocked.Quota.State != "exhausted" || blocked.Quota.ResetAt == nil || blocked.Quota.Scope != "model" ||
		blocked.BlockedUntil == nil || blocked.BlockReason != "429 seen" || !strings.HasPrefix(blocked.StartableReason, "blocked until ") {
		t.Fatalf("blocked row = %+v", blocked)
	}
	out.Reset()
	if err := runModelsUnblock(&out, modelblock.Path(), target.Spec); err != nil || !strings.Contains(out.String(), "unblocked") {
		t.Fatalf("unblock: %v %q", err, out.String())
	}
	for _, r := range modelsJSON(t) {
		if r.BlockedUntil != nil {
			t.Fatalf("block survived unblock: %+v", r)
		}
	}
}

func TestModelsBlockRejectsBadInput(t *testing.T) {
	isolateModelState(t)
	var out bytes.Buffer
	if err := runModelsBlock(&out, modelblock.Path(), "nope:nothing", "1h", "", time.Now()); err == nil {
		t.Fatal("unknown model accepted")
	}
	target := modelsJSON(t)[0].Spec
	if err := runModelsBlock(&out, modelblock.Path(), target, "yesterday", "", time.Now()); err == nil {
		t.Fatal("bad --until accepted")
	}
	if err := runModelsBlock(&out, modelblock.Path(), target, "2020-01-01T00:00:00Z", "", time.Now()); err == nil {
		t.Fatal("past --until accepted")
	}
}

type quotaFailDriver struct {
	recordingAgentDriver
	msg string
}

func (d *quotaFailDriver) Run(context.Context, subagent.RunOptions) (*subagent.TurnResult, error) {
	return nil, errors.New(d.msg)
}

func (d *quotaFailDriver) RunStream(context.Context, subagent.RunOptions, subagent.EventFunc) (*subagent.TurnResult, error) {
	return nil, errors.New(d.msg)
}

func TestAgentStartAutoBlocksOnKnownQuotaErrorAndRefusesNextStart(t *testing.T) {
	isolateModelState(t)
	old := agentDriver
	d := &quotaFailDriver{msg: "You've hit your usage limit. Try again in 3 hours."}
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }}
	req := startRequest{Name: "q1", Role: "advisor", Prompt: "work", StoredPrompt: "work", ModelSpec: "codex:luna:low", Dir: t.TempDir(), StreamMode: streamFull}
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := runStart(t.Context(), cmd, deps, req); err == nil {
		t.Fatal("expected quota failure")
	}
	blocks := modelblock.Active(modelblock.Path(), time.Now())
	if len(blocks) != 1 || blocks[0].Provider != "codex" {
		t.Fatalf("blocks = %+v\n%s", blocks, out.String())
	}
	if left := time.Until(blocks[0].Until); left < 2*time.Hour+50*time.Minute || left > 3*time.Hour+time.Minute {
		t.Fatalf("block lasts %v, want the 3h the error named", left)
	}
	req.Name = "q2"
	err = runStart(t.Context(), cmd, deps, req)
	if err == nil || !strings.Contains(err.Error(), "quota is exhausted") {
		t.Fatalf("second start = %v, want refusal for exhausted quota", err)
	}
	req.Name, req.AllowExhaustedQuota = "q3", true
	if err := runStart(t.Context(), cmd, deps, req); err == nil || strings.Contains(err.Error(), "quota is exhausted") {
		t.Fatalf("--allow-exhausted-quota must bypass the block, got %v", err)
	}
}

func TestAgentStartIgnoresUnrelatedErrorsForBlocking(t *testing.T) {
	isolateModelState(t)
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return &quotaFailDriver{msg: "network unreachable"} }
	defer func() { agentDriver = old }()
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	store, _ := subagent.NewSessionStore(t.TempDir())
	deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	_ = runStart(t.Context(), cmd, deps, startRequest{Name: "n1", Role: "advisor", Prompt: "w", StoredPrompt: "w", ModelSpec: "codex:luna:low", Dir: t.TempDir(), StreamMode: streamFull})
	if got := modelblock.Active(modelblock.Path(), time.Now()); len(got) != 0 {
		t.Fatalf("unrelated error blocked: %+v", got)
	}
}
