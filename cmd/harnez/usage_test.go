package main

import "testing"

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
