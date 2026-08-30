package main

import (
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

// TestValidateUsageFlags_CompactAllowedWithSummary is issue 102's core
// acceptance criterion: `harnez usage --summary --compact` must no longer be
// rejected, since RenderSummary/RenderSummaryRemote share the same compact
// renderer --watch --compact uses.
func TestValidateUsageFlags_CompactAllowedWithSummary(t *testing.T) {
	if err := validateUsageFlags(false, true, true); err != nil {
		t.Fatalf("--summary --compact should be allowed, got error: %v", err)
	}
}

func TestValidateUsageFlags_CompactAllowedWithWatch(t *testing.T) {
	if err := validateUsageFlags(true, false, true); err != nil {
		t.Fatalf("--watch --compact should be allowed, got error: %v", err)
	}
}

func TestValidateUsageFlags_CompactRejectedWithoutWatchOrSummary(t *testing.T) {
	if err := validateUsageFlags(false, false, true); err == nil {
		t.Fatalf("expected --compact alone (no --watch or --summary) to be rejected")
	}
}

func TestValidateUsageFlags_WatchAndSummaryRejected(t *testing.T) {
	if err := validateUsageFlags(true, true, false); err == nil {
		t.Fatalf("expected --watch and --summary together to be rejected")
	}
}

func TestValidateUsageFlags_NoFlagsOK(t *testing.T) {
	if err := validateUsageFlags(false, false, false); err != nil {
		t.Fatalf("expected no flags to be valid, got error: %v", err)
	}
}
