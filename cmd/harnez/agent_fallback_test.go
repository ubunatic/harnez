package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
)

// smartFallback reads the shipped developer smart picks (issue 1252: no pinned
// models). It returns the first pick from another provider than the head, that
// pick's model, and the fallback notice when every earlier pick fails for reason.
func smartFallback(t *testing.T, reason string) (string, subagent.Model, string) {
	t.Helper()
	picks, err := subagent.ModelPicksWithAvailability(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range picks {
		if p.Role != "developer" || p.Trait != "smart" {
			continue
		}
		var skipped []string
		head := subagent.Model{}
		for i, entry := range p.Chain {
			m, err := subagent.ResolveModel(entry)
			if err != nil {
				t.Fatalf("developer smart pick %q: %v", entry, err)
			}
			if i == 0 {
				head = m
			}
			if m.Provider == head.Provider {
				skipped = append(skipped, entry+": "+reason)
				continue
			}
			return entry, m, "smart → " + entry + " (" + strings.Join(skipped, "; ") + ")"
		}
	}
	t.Fatal("developer smart picks need a second provider for fallback tests")
	return "", subagent.Model{}, ""
}

// smartHeadProvider is the provider of the first developer smart pick.
func smartHeadProvider(t *testing.T) string {
	t.Helper()
	m, err := subagent.ResolveModelForRole("developer", "smart")
	if err != nil {
		t.Fatal(err)
	}
	return m.Provider
}

func TestPolicyFallbackProcess(t *testing.T) {
	if os.Getenv("TEST_983_CHILD") == "1" {
		var out bytes.Buffer
		model, err := selectStartModel("developer", "smart", false, func(string, string) usage.ProviderQuotaAvailability {
			return usage.ProviderQuotaAvailability{State: "unknown"}
		}, &out)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("%s %s", model.Provider, out.String())
		return
	}
	_, target, notice := smartFallback(t, "disabled by user policy")
	config := t.TempDir()
	if err := os.MkdirAll(filepath.Join(config, "harnez"), 0700); err != nil {
		t.Fatal(err)
	}
	policy := "version: 1\ndefault_enabled: false\nenabled: [" + target.Spec() + "]\nmodels: {}\n"
	if err := os.WriteFile(filepath.Join(config, "harnez", "models.yaml"), []byte(policy), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestPolicyFallbackProcess$")
	cmd.Env = append(os.Environ(), "TEST_983_CHILD=1", "XDG_CONFIG_HOME="+config, "XDG_STATE_HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), target.Provider+" "+notice) {
		t.Fatalf("output=%s error=%v, want %q", out, err, target.Provider+" "+notice)
	}
}

func TestInvalidRolePolicyCLI(t *testing.T) {
	if os.Getenv("TEST_983_CHILD") == "1" {
		root := newRootCmd()
		root.SetArgs(strings.Fields(os.Getenv("TEST_983_ARGS")))
		if err := root.Execute(); err != nil {
			os.Exit(1)
		}
		return
	}
	config := t.TempDir()
	policyPath := filepath.Join(config, "harnez", "models.yaml")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0700); err != nil {
		t.Fatal(err)
	}
	policy := "version: 1\ndefault_enabled: true\nenabled: []\nmodels: {}\nroles:\n  reviewer:\n    models:\n      smart: [smart]\n"
	if err := os.WriteFile(policyPath, []byte(policy), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args  string
		fails bool
	}{
		{"--help", false}, {"--version", false}, {"agent models --picks", true}, {"agent selftest --step hello", true},
	} {
		t.Run(tc.args, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestInvalidRolePolicyCLI$")
			cmd.Env = append(os.Environ(), "TEST_983_CHILD=1", "TEST_983_ARGS="+tc.args, "XDG_CONFIG_HOME="+config, "XDG_STATE_HOME="+t.TempDir())
			out, err := cmd.CombinedOutput()
			text := string(out)
			if (err != nil) != tc.fails || strings.Contains(text, "panic:") || strings.Contains(text, "goroutine") {
				t.Fatalf("output=%s error=%v", text, err)
			}
			if tc.fails && strings.Count(text, policyPath+`: role reviewer trait smart: trait cycle at "smart"`) != 1 {
				t.Fatalf("missing or repeated error context: %s", text)
			}
		})
	}
}

func TestFallbackNoticeAndQuotaOverride(t *testing.T) {
	_, target, notice := smartFallback(t, "quota exhausted")
	head := smartHeadProvider(t)
	var out bytes.Buffer
	availability := func(provider, model string) usage.ProviderQuotaAvailability {
		if provider == head {
			return usage.ProviderQuotaAvailability{State: "exhausted"}
		}
		return usage.ProviderQuotaAvailability{State: "available"}
	}
	result, err := selectStartModel("developer", "smart", false, availability, &out)
	if err != nil || result.Provider != target.Provider || result.Name != target.Name || result.Tier != target.Tier || !strings.Contains(out.String(), notice) {
		t.Fatalf("selection=%+v err=%v notice=%q, want %+v and %q", result, err, out.String(), target, notice)
	}
	out.Reset()
	result, err = selectStartModel("developer", "smart", true, availability, &out)
	if err != nil || result.Provider != head || out.Len() != 0 {
		t.Fatalf("override=%+v err=%v notice=%q", result, err, out.String())
	}
}

// otherTier returns a tier different from tier, for session-conflict checks.
func otherTier(tier string) string {
	if tier == "low" {
		return "med"
	}
	return "low"
}

func TestResumeTraitFallsBackToSessionModel(t *testing.T) {
	_, target, _ := smartFallback(t, "quota exhausted")
	head := smartHeadProvider(t)
	session := &subagent.Session{Provider: target.Provider, Model: target.Name, Tier: target.Tier}
	var notice bytes.Buffer
	availability := func(provider, model string) usage.ProviderQuotaAvailability {
		if provider == head {
			return usage.ProviderQuotaAvailability{State: "exhausted"}
		}
		return usage.ProviderQuotaAvailability{State: "available"}
	}
	if err := checkResumeModel("developer", "smart", session, false, availability, &notice); err != nil || !strings.Contains(notice.String(), ": quota exhausted") {
		t.Fatalf("notice=%q err=%v", notice.String(), err)
	}
	session.Tier = otherTier(target.Tier)
	if err := checkResumeModel("developer", "smart", session, false, availability, &notice); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("conflict=%v", err)
	}
}

func TestWorkerUsesRecordedSelection(t *testing.T) {
	s := &subagent.Session{Provider: "claude", Model: "opus", Tier: "med", ModelAlias: "smart"}
	got, err := workerSelection(s, "claude:opus:med")
	if err != nil || got.Model.Provider != "claude" || got.Model.Name != "opus" || got.Trait != "smart" {
		t.Fatalf("selection=%+v err=%v", got, err)
	}
	if _, err := workerSelection(s, "codex:sol:med"); err == nil {
		t.Fatal("worker accepted model different from parent")
	}
}

func TestTraitFailureDoesNotContradictHint(t *testing.T) {
	selection := subagent.ModelSelection{Trait: "smart"}
	err := startModelError("smart", selection, fmt.Errorf("no available model; pass --model provider:model:tier"))
	if !strings.Contains(err.Error(), "--model provider:model:tier") || strings.Contains(err.Error(), "ask for guidance") {
		t.Fatal(err)
	}
}

func TestResumeDisabledCandidatePolicyProcess(t *testing.T) {
	if os.Getenv("TEST_983_CHILD") == "1" {
		var notice bytes.Buffer
		s := &subagent.Session{Provider: os.Getenv("TEST_983_PROVIDER"), Model: os.Getenv("TEST_983_MODEL"), Tier: os.Getenv("TEST_983_TIER")}
		err := checkResumeModel("developer", "smart", s, false, func(string, string) usage.ProviderQuotaAvailability {
			return usage.ProviderQuotaAvailability{State: "unknown"}
		}, &notice)
		if err != nil || !strings.Contains(notice.String(), ": disabled by user policy") {
			t.Fatalf("notice=%q err=%v", notice.String(), err)
		}
		return
	}
	_, target, _ := smartFallback(t, "disabled by user policy")
	config := t.TempDir()
	if err := os.MkdirAll(filepath.Join(config, "harnez"), 0700); err != nil {
		t.Fatal(err)
	}
	policy := "version: 1\ndefault_enabled: false\nenabled: [" + target.Spec() + "]\nmodels: {}\n"
	if err := os.WriteFile(filepath.Join(config, "harnez", "models.yaml"), []byte(policy), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestResumeDisabledCandidatePolicyProcess$")
	cmd.Env = append(os.Environ(), "TEST_983_CHILD=1", "XDG_CONFIG_HOME="+config, "XDG_STATE_HOME="+t.TempDir(),
		"TEST_983_PROVIDER="+target.Provider, "TEST_983_MODEL="+target.Name, "TEST_983_TIER="+target.Tier)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("output=%s error=%v", out, err)
	}
}
