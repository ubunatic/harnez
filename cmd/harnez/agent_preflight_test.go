package main

import (
	"context"
	"errors"
	"testing"
)

func TestPreflightCodexBypassesOtherProviders(t *testing.T) {
	calls := 0
	check := func(context.Context) error { calls++; return errors.New("unexpected probe") }
	if err := preflightCodex(context.Background(), "claude", nil, check); err != nil {
		t.Fatalf("claude preflight error = %v, want nil", err)
	}
	if err := preflightCodex(context.Background(), "agy", nil, check); err != nil {
		t.Fatalf("agy preflight error = %v, want nil", err)
	}
	if calls != 0 {
		t.Fatalf("probe calls = %d, want 0 for non-Codex providers", calls)
	}
}

func TestPreflightCodexCallsChecker(t *testing.T) {
	want := errors.New("rejected")
	calls := 0
	err := preflightCodex(context.Background(), "codex", nil, func(context.Context) error { calls++; return want })
	if !errors.Is(err, want) || calls != 1 {
		t.Fatalf("preflight = (%v, calls %d), want (%v, 1)", err, calls, want)
	}
}
