package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagestore"
)

type stubCostDriver struct {
	runs, resumes int
}

func (d *stubCostDriver) Run(context.Context, subagent.RunOptions) (*subagent.TurnResult, error) {
	d.runs++
	return &subagent.TurnResult{Response: "ok", Messages: []string{"ok"}, InputTokens: 10, OutputTokens: 5, ContextTokens: 10}, nil
}

func (d *stubCostDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	d.resumes++
	return &subagent.TurnResult{Response: "resumed", Messages: []string{"resumed"}, InputTokens: 10, OutputTokens: 5, ContextTokens: 10}, nil
}

func (d *stubCostDriver) Stop(context.Context, string) error   { return nil }
func (d *stubCostDriver) Delete(context.Context, string) error { return nil }
func (d *stubCostDriver) Compact(context.Context, string, subagent.Model) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{}, nil
}

// costlyAndCheapEntries returns the first costly and the first cheap shipped
// model entry with a known cost, so the tests follow spec/agent.yaml (issue 1252).
func costlyAndCheapEntries(t *testing.T) (costly, cheap subagent.ModelEntry) {
	t.Helper()
	for _, e := range subagent.KnownModelEntries() {
		isCostly, err := subagent.IsCostlyModelFor(e.Model)
		if err != nil {
			t.Fatal(err)
		}
		if e.Cost <= 0 {
			continue
		}
		if isCostly && costly.Spec == "" {
			costly = e
		}
		if !isCostly && cheap.Spec == "" {
			cheap = e
		}
	}
	if costly.Spec == "" || cheap.Spec == "" {
		t.Fatalf("spec needs a costly and a cheap model with a cost: costly=%q cheap=%q", costly.Spec, cheap.Spec)
	}
	return costly, cheap
}

func TestAcceptCostStartAndResume(t *testing.T) {
	now := time.Now().UTC()
	costlyEntry, cheapEntry := costlyAndCheapEntries(t)
	freshAvail := func(string, string) usage.ProviderQuotaAvailability {
		return usage.ProviderQuotaAvailability{
			State: "available",
			Windows: []usagestore.Window{
				{Key: "five_hour", UsedFraction: 0.2, ObservedAt: now, Freshness: "fresh"},
				{Key: "weekly", UsedFraction: 0.35, ObservedAt: now, Freshness: "fresh"},
			},
		}
	}
	staleAvail := func(string, string) usage.ProviderQuotaAvailability {
		return usage.ProviderQuotaAvailability{State: "stale"}
	}

	d := &stubCostDriver{}
	prev := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	t.Cleanup(func() { agentDriver = prev })

	runStartCase := func(modelSpec string, acceptCost bool, avail func(string, string) usage.ProviderQuotaAvailability) (string, string, error) {
		store, err := subagent.NewSessionStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		deps := agentDeps{
			store:        func() (*subagent.FileSessionStore, error) { return store, nil },
			parent:       func() string { return "" },
			availability: avail,
			preflight:    func(context.Context) error { return nil },
		}
		err = runStart(context.Background(), cmd, deps, startRequest{
			Prompt: "check", Role: "advisor", ModelSpec: modelSpec, Dir: t.TempDir(),
			StreamMode: streamFull, AcceptCost: acceptCost,
		})
		return out.String(), errOut.String(), err
	}

	// Cheap model starts without --accept-cost or costly warning.
	if _, stderr, err := runStartCase(cheapEntry.Spec, false, freshAvail); err != nil || strings.Contains(stderr, "Warning: using costly model") {
		t.Fatalf("cheap start failed: err=%v stderr=%q", err, stderr)
	}
	if d.runs != 1 {
		t.Fatalf("runs=%d, want 1", d.runs)
	}

	// Each costly model (also by official name) is refused without --accept-cost,
	// naming its spec, relative cost and quota; stale quota reads as unknown.
	type costlyCase struct {
		spec, wantModel, wantCost, wantQuota string
		avail                                func(string, string) usage.ProviderQuotaAvailability
	}
	var cases []costlyCase
	for _, e := range subagent.KnownModelEntries() {
		if isCostly, err := subagent.IsCostlyModelFor(e.Model); err != nil || !isCostly {
			continue
		}
		cost := strconv.Itoa(e.Cost)
		cases = append(cases, costlyCase{e.Spec, e.Spec, cost, "5h 80%, weekly 65%", freshAvail})
		if m, err := subagent.ResolveModel(e.Model.Name + ":" + e.Model.Tier); err == nil && m.Spec() == e.Spec {
			cases = append(cases, costlyCase{e.Model.Name + ":" + e.Model.Tier, e.Spec, cost, "unknown", staleAvail})
		}
	}
	for _, tc := range cases {
		_, _, err := runStartCase(tc.spec, false, tc.avail)
		if err == nil {
			t.Fatalf("%s: expected refusal without --accept-cost", tc.spec)
		}
		msg := err.Error()
		for _, want := range []string{tc.wantModel, tc.wantCost, tc.wantQuota, "--accept-cost"} {
			if !strings.Contains(msg, want) {
				t.Fatalf("%s refusal %q missing %q", tc.spec, msg, want)
			}
		}
	}
	if d.runs != 1 {
		t.Fatalf("driver ran on refused costly starts: runs=%d", d.runs)
	}

	// Passing --accept-cost prints one stderr warning line and starts.
	_, stderr, err := runStartCase(costlyEntry.Spec, true, freshAvail)
	if err != nil {
		t.Fatalf("accepted costly start failed: %v", err)
	}
	if strings.Count(stderr, "Warning: using costly model") != 1 || !strings.Contains(stderr, strconv.Itoa(costlyEntry.Cost)) || !strings.Contains(stderr, "5h 80%, weekly 65%") {
		t.Fatalf("stderr warning = %q, want one warning line with cost and quota", stderr)
	}
	if d.runs != 2 {
		t.Fatalf("runs=%d, want 2", d.runs)
	}

	// Resume on a costly session also refuses without --accept-cost and warns with it.
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "sid", Name: "costly", Provider: costlyEntry.Model.Provider, Model: costlyEntry.Model.Name, Tier: costlyEntry.Model.Tier, Role: "advisor", WorkingDir: t.TempDir(), Status: "completed", HarnessType: "harnez"}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	runResumeCase := func(acceptCost bool) (string, error) {
		var out, errOut bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		deps := agentDeps{
			store:  func() (*subagent.FileSessionStore, error) { return store, nil },
			parent: func() string { return "" },
			find: func(_ *cobra.Command, s *subagent.FileSessionStore, _ string) (*subagent.Session, error) {
				return s.Get("sid")
			},
			availability: freshAvail,
		}
		err := runResume(context.Background(), cmd, deps, resumeRequest{Name: "costly", Prompt: "next", StreamMode: streamFull, AcceptCost: acceptCost})
		return errOut.String(), err
	}
	if _, err := runResumeCase(false); err == nil || !strings.Contains(err.Error(), costlyEntry.Spec) || !strings.Contains(err.Error(), "--accept-cost") {
		t.Fatalf("resume without --accept-cost = %v, want refusal", err)
	}
	if d.resumes != 0 {
		t.Fatalf("driver resumed despite refusal: resumes=%d", d.resumes)
	}
	if stderr, err := runResumeCase(true); err != nil || !strings.Contains(stderr, strconv.Itoa(costlyEntry.Cost)) {
		t.Fatalf("resume with --accept-cost err=%v stderr=%q", err, stderr)
	}
	if d.resumes != 1 {
		t.Fatalf("resumes=%d, want 1", d.resumes)
	}
}

func TestAcceptCostDryRunAndPickFallback(t *testing.T) {
	costlyEntry, cheapEntry := costlyAndCheapEntries(t)
	if os.Getenv("TEST_1210_PICK_CHILD") == "1" {
		sel, err := subagent.ResolveModelWithOptions("advisor", "smart", subagent.ResolveOptions{AcceptCost: false})
		if err != nil {
			t.Fatal(err)
		}
		if sel.Model.Spec() != cheapEntry.Spec || !strings.Contains(strings.Join(sel.Skipped, "; "), "costly model") {
			t.Fatalf("without accept-cost: %+v", sel)
		}
		sel2, err := subagent.ResolveModelWithOptions("advisor", "smart", subagent.ResolveOptions{AcceptCost: true})
		if err != nil || sel2.Model.Spec() != costlyEntry.Spec {
			t.Fatalf("with accept-cost: %+v err=%v", sel2, err)
		}
		return
	}
	// --dry-run shows whether --accept-cost is required for start and resume.
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "sid", Name: "costly", Provider: costlyEntry.Model.Provider, Model: costlyEntry.Model.Name, Tier: costlyEntry.Model.Tier, Role: "advisor", WorkingDir: t.TempDir(), Status: "completed", HarnessType: "harnez"}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	deps := agentDeps{
		store:  func() (*subagent.FileSessionStore, error) { return store, nil },
		parent: func() string { return "" },
		find: func(_ *cobra.Command, s *subagent.FileSessionStore, _ string) (*subagent.Session, error) {
			return s.Get("sid")
		},
		availability: func(string, string) usage.ProviderQuotaAvailability {
			return usage.ProviderQuotaAvailability{State: "stale"}
		},
	}
	for _, tc := range []struct {
		model, want string
		accept      bool
	}{
		{costlyEntry.Spec, "accept-cost: required (pass --accept-cost; relative cost " + strconv.Itoa(costlyEntry.Cost) + " (", false},
		{costlyEntry.Spec, "accept-cost: accepted (relative cost " + strconv.Itoa(costlyEntry.Cost) + " (", true},
		{cheapEntry.Spec, "accept-cost: not required", false},
	} {
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		err := runStart(context.Background(), cmd, deps, startRequest{DryRun: true, Prompt: "hi", PromptSource: "-p", Name: "w", Role: "advisor", ModelSpec: tc.model, Dir: t.TempDir(), StreamMode: streamFull, AcceptCost: tc.accept})
		if err != nil || !strings.Contains(out.String(), tc.want) {
			t.Fatalf("start dry-run (%s, %v) err=%v out=%q want=%q", tc.model, tc.accept, err, out.String(), tc.want)
		}
	}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := runResume(context.Background(), cmd, deps, resumeRequest{DryRun: true, Name: "costly", Prompt: "hi", PromptSource: "-p", StreamMode: streamFull}); err != nil || !strings.Contains(out.String(), "accept-cost: required") {
		t.Fatalf("resume dry-run err=%v out=%q", err, out.String())
	}

	// Pick fallback chain via ResolveModelWithOptions never silently picks a costly candidate without --accept-cost.
	cfgDir := t.TempDir()
	harnezDir := filepath.Join(cfgDir, "harnez")
	if err := os.MkdirAll(harnezDir, 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(harnezDir, "models.yaml"), []byte("version: 1\ndefault_enabled: true\nenabled: []\nmodels: {}\nroles:\n  advisor:\n    models:\n      smart: ["+costlyEntry.Spec+", "+cheapEntry.Spec+"]\n"), 0o600)
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	proc := exec.Command(os.Args[0], "-test.run=^TestAcceptCostDryRunAndPickFallback$")
	proc.Env = append(os.Environ(), "TEST_1210_PICK_CHILD=1")
	if out, err := proc.CombinedOutput(); err != nil {
		t.Fatalf("pick subprocess failed: %v\n%s", err, out)
	}
}
