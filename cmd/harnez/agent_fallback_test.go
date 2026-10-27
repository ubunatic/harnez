package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/usage"
)

func TestPolicyFallbackProcess(t *testing.T) {
	if os.Getenv("TEST_983_CHILD") == "1" {
		var notice bytes.Buffer
		model, err := selectStartModel("developer", "smart", false, func(string, string) usage.ProviderQuotaAvailability {
			return usage.ProviderQuotaAvailability{State: "unknown"}
		}, &notice)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("%s %s", model.Provider, notice.String())
		return
	}
	config := t.TempDir()
	if err := os.MkdirAll(filepath.Join(config, "harnez"), 0700); err != nil {
		t.Fatal(err)
	}
	policy := "version: 1\ndefault_enabled: false\nenabled: [claude:opus:med]\nmodels: {}\n"
	if err := os.WriteFile(filepath.Join(config, "harnez", "models.yaml"), []byte(policy), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestPolicyFallbackProcess$")
	cmd.Env = append(os.Environ(), "TEST_983_CHILD=1", "XDG_CONFIG_HOME="+config, "XDG_STATE_HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "claude smart → opus:med (sol:med: disabled by user policy)") {
		t.Fatalf("output=%s error=%v", out, err)
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
			if tc.fails && (!strings.Contains(text, policyPath) || !strings.Contains(text, "reviewer/smart") || strings.Count(text, `agent spec: role "reviewer" trait "smart":`) != 1) {
				t.Fatalf("missing or repeated error context: %s", text)
			}
		})
	}
}

func TestFallbackNoticeAndQuotaOverride(t *testing.T) {
	var out bytes.Buffer
	availability := func(provider, model string) usage.ProviderQuotaAvailability {
		if provider == "codex" {
			return usage.ProviderQuotaAvailability{State: "exhausted"}
		}
		return usage.ProviderQuotaAvailability{State: "available"}
	}
	result, err := selectStartModel("developer", "smart", false, availability, &out)
	if err != nil || result.Provider != "claude" || result.Tier != "med" || !strings.Contains(out.String(), "smart → opus:med (sol:med: quota exhausted)") {
		t.Fatalf("selection=%+v err=%v notice=%q", result, err, out.String())
	}
	out.Reset()
	result, err = selectStartModel("developer", "smart", true, availability, &out)
	if err != nil || result.Provider != "codex" || out.Len() != 0 {
		t.Fatalf("override=%+v err=%v notice=%q", result, err, out.String())
	}
}
