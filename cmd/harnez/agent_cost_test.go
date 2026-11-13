package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

func TestAcceptCostStartAndResume(t *testing.T) {
	now := time.Now().UTC()
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
	if _, stderr, err := runStartCase("codex:luna:low", false, freshAvail); err != nil || strings.Contains(stderr, "Warning: using costly model") {
		t.Fatalf("cheap start failed: err=%v stderr=%q", err, stderr)
	}
	if d.runs != 1 {
		t.Fatalf("runs=%d, want 1", d.runs)
	}

	// Each costly model (including aliases and official names) is refused without --accept-cost.
	for _, tc := range []struct {
		spec, wantModel, wantCost, wantQuota string
		avail                                func(string, string) usage.ProviderQuotaAvailability
	}{
		{"codex:sol:high", "codex:sol:high", "71", "5h 80%, weekly 65%", freshAvail},
		{"sol:high", "codex:sol:high", "71", "5h 80%, weekly 65%", freshAvail},
		{"agy:argon:high", "agy:argon:high", "unknown", "unknown", staleAvail},
		{"gemini-4-argon:high", "agy:argon:high", "unknown", "unknown", staleAvail},
		{"codex:astra:low", "codex:astra:low", "182", "5h 80%, weekly 65%", freshAvail},
		{"gpt-6-astra:med", "codex:astra:med", "342", "5h 80%, weekly 65%", freshAvail},
		{"claude:fable:high", "claude:fable:high", "unknown", "unknown", staleAvail},
	} {
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
	_, stderr, err := runStartCase("codex:astra:low", true, freshAvail)
	if err != nil {
		t.Fatalf("accepted costly start failed: %v", err)
	}
	if strings.Count(stderr, "Warning: using costly model") != 1 || !strings.Contains(stderr, "182") || !strings.Contains(stderr, "5h 80%, weekly 65%") {
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
	sess := &subagent.Session{ID: "sid", Name: "costly", Provider: "codex", Model: "gpt-6-astra", Tier: "low", Role: "advisor", WorkingDir: t.TempDir(), Status: "completed", HarnessType: "harnez"}
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
	if _, err := runResumeCase(false); err == nil || !strings.Contains(err.Error(), "codex:astra:low") || !strings.Contains(err.Error(), "--accept-cost") {
		t.Fatalf("resume without --accept-cost = %v, want refusal", err)
	}
	if d.resumes != 0 {
		t.Fatalf("driver resumed despite refusal: resumes=%d", d.resumes)
	}
	if stderr, err := runResumeCase(true); err != nil || !strings.Contains(stderr, "182") {
		t.Fatalf("resume with --accept-cost err=%v stderr=%q", err, stderr)
	}
	if d.resumes != 1 {
		t.Fatalf("resumes=%d, want 1", d.resumes)
	}
}

func TestAcceptCostDryRunAndPickFallback(t *testing.T) {
	if os.Getenv("TEST_1210_PICK_CHILD") == "1" {
		sel, err := subagent.ResolveModelWithOptions("advisor", "smart", subagent.ResolveOptions{AcceptCost: false})
		if err != nil {
			t.Fatal(err)
		}
		if sel.Model.Spec() != "codex:luna:low" || !strings.Contains(strings.Join(sel.Skipped, "; "), "costly model") {
			t.Fatalf("without accept-cost: %+v", sel)
		}
		sel2, err := subagent.ResolveModelWithOptions("advisor", "smart", subagent.ResolveOptions{AcceptCost: true})
		if err != nil || sel2.Model.Spec() != "codex:sol:high" {
			t.Fatalf("with accept-cost: %+v err=%v", sel2, err)
		}
		return
	}
	// --dry-run shows whether --accept-cost is required for start and resume.
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "sid", Name: "costly", Provider: "codex", Model: "gpt-6-astra", Tier: "low", Role: "advisor", WorkingDir: t.TempDir(), Status: "completed", HarnessType: "harnez"}
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
		{"codex:astra:low", "accept-cost: required (pass --accept-cost; relative cost 182 (codex:luna:low = 1), quota left: unknown)", false},
		{"codex:astra:low", "accept-cost: accepted (relative cost 182 (codex:luna:low = 1), quota left: unknown)", true},
		{"codex:luna:low", "accept-cost: not required", false},
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
	_ = os.WriteFile(filepath.Join(harnezDir, "models.yaml"), []byte("version: 1\ndefault_enabled: true\nenabled: []\nmodels: {}\nroles:\n  advisor:\n    models:\n      smart: [sol:high, luna:low]\n"), 0o600)
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	proc := exec.Command(os.Args[0], "-test.run=^TestAcceptCostDryRunAndPickFallback$")
	proc.Env = append(os.Environ(), "TEST_1210_PICK_CHILD=1")
	if out, err := proc.CombinedOutput(); err != nil {
		t.Fatalf("pick subprocess failed: %v\n%s", err, out)
	}
}
