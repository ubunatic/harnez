package main

import (
	"bytes"
	"testing"

	"ubunatic.com/harnez/internal/usage"
)

// TestResolveUsageHost_FlagWinsOverLocalConfig is issue 109's acceptance
// criterion 3: an explicit --host always overrides usage.default_host from
// ~/.config/harnez/local.yaml.
func TestResolveUsageHost_FlagWinsOverLocalConfig(t *testing.T) {
	cfg := &usage.LocalConfig{Usage: usage.LocalUsageConfig{DefaultHost: "fromlocal"}}
	got := resolveUsageHost("fromflag", cfg)
	if got != "fromflag" {
		t.Fatalf("resolveUsageHost() = %q, want %q", got, "fromflag")
	}
}

func TestResolveUsageHost_FallsBackToLocalConfig(t *testing.T) {
	cfg := &usage.LocalConfig{Usage: usage.LocalUsageConfig{DefaultHost: "fromlocal"}}
	got := resolveUsageHost("", cfg)
	if got != "fromlocal" {
		t.Fatalf("resolveUsageHost() = %q, want %q", got, "fromlocal")
	}
}

func TestResolveUsageHost_NilConfigNoFlag(t *testing.T) {
	got := resolveUsageHost("", nil)
	if got != "" {
		t.Fatalf("resolveUsageHost() = %q, want empty", got)
	}
}

func TestResolveUsageHost_EmptyLocalDefault(t *testing.T) {
	cfg := &usage.LocalConfig{}
	got := resolveUsageHost("", cfg)
	if got != "" {
		t.Fatalf("resolveUsageHost() = %q, want empty", got)
	}
}

// TestValidateUsageFlags_NoFlagsOK: a bare `harnez usage` (the compact
// one-shot dashboard, formerly gated behind a since-removed --summary flag)
// must not be rejected.
func TestValidateUsageFlags_NoFlagsOK(t *testing.T) {
	if err := validateUsageFlags(false, false, false, false, false); err != nil {
		t.Fatalf("expected no flags to be valid, got error: %v", err)
	}
}

// TestValidateUsageFlags_CompactAloneOK: `--compact` alone selects the
// reduced panel set on the now-default compact dashboard; it is not an error
// on its own the way it used to require --watch or --summary.
func TestValidateUsageFlags_CompactAloneOK(t *testing.T) {
	if err := validateUsageFlags(false, false, false, true, false); err != nil {
		t.Fatalf("--compact alone should be allowed, got error: %v", err)
	}
}

func TestValidateUsageFlags_CompactAllowedWithWatch(t *testing.T) {
	if err := validateUsageFlags(true, false, false, true, false); err != nil {
		t.Fatalf("--watch --compact should be allowed, got error: %v", err)
	}
}

func TestValidateUsageFlags_WatchAndJSONRejected(t *testing.T) {
	if err := validateUsageFlags(true, false, true, false, false); err == nil {
		t.Fatalf("expected --watch and --json together to be rejected")
	}
}

func TestValidateUsageFlags_WatchAndRawRejected(t *testing.T) {
	if err := validateUsageFlags(true, true, false, false, false); err == nil {
		t.Fatalf("expected --watch and --raw together to be rejected")
	}
}

func TestValidateUsageFlags_RawAndJSONRejected(t *testing.T) {
	if err := validateUsageFlags(false, true, true, false, false); err == nil {
		t.Fatalf("expected --raw and --json together to be rejected")
	}
}

func TestValidateUsageFlags_CompactWithRawRejected(t *testing.T) {
	if err := validateUsageFlags(false, true, false, true, false); err == nil {
		t.Fatalf("expected --compact and --raw together to be rejected (--raw has no panel concept)")
	}
}

func TestValidateUsageFlags_RawAlone_OK(t *testing.T) {
	if err := validateUsageFlags(false, true, false, false, false); err != nil {
		t.Fatalf("--raw alone should be allowed, got error: %v", err)
	}
}

func TestValidateUsageFlags_LoomAlone_OK(t *testing.T) {
	if err := validateUsageFlags(false, false, false, false, true); err != nil {
		t.Fatalf("--loom alone should be allowed, got error: %v", err)
	}
}

func TestValidateUsageFlags_LoomWithRawRejected(t *testing.T) {
	if err := validateUsageFlags(false, true, false, false, true); err == nil {
		t.Fatalf("expected --loom and --raw together to be rejected")
	}
}

func TestValidateUsageFlags_LoomWithJSONRejected(t *testing.T) {
	if err := validateUsageFlags(false, false, true, false, true); err == nil {
		t.Fatalf("expected --loom and --json together to be rejected")
	}
}

func TestUsageProjectFlag_CobraRegistered(t *testing.T) {
	root := newRootCmd()
	if usageCmd := root.Commands(); usageCmd != nil {
		for _, c := range usageCmd {
			if c.Name() == "usage" {
				if c.Flags().Lookup("project") == nil {
					t.Errorf("missing --project flag on usage command")
				}
				if c.Flags().Lookup("cwd") == nil {
					t.Errorf("missing --cwd flag on usage command")
				}
				if c.Flags().Lookup("loom") == nil {
					t.Errorf("missing --loom flag on usage command")
				}
			}
			if c.Name() == "assess" {
				if c.Flags().Lookup("tokens") == nil {
					t.Errorf("missing --tokens flag on assess command")
				}
				if c.Flags().Lookup("history") == nil {
					t.Errorf("missing --history flag on assess command")
				}
				if c.Flags().Lookup("ramp") == nil {
					t.Errorf("missing --ramp flag on assess command")
				}
			}
		}
	}
}

func TestUsageCmd_LoomFlagExecution(t *testing.T) {
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"usage", "--loom", "--offline"})

	if err := root.Execute(); err != nil {
		t.Fatalf("harnez usage --loom failed: %v", err)
	}

	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Agentic usage")) {
		t.Errorf("expected output to contain 'Agentic usage', got:\n%s", out)
	}
}
