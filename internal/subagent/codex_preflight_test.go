package subagent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCodexPreflightCachesSuccessfulProbe(t *testing.T) {
	calls := 0
	p := NewCodexPreflight(func(context.Context) error { calls++; return nil })
	if err := p.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := p.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("probe calls = %d, want 1", calls)
	}
	p.now = func() time.Time { return time.Now().Add(2 * codexPreflightTTL) }
	if err := p.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("probe calls after expiry = %d, want 2", calls)
	}
}

func TestCodexPreflightAuthRejectionIsActionableAndNotCached(t *testing.T) {
	calls := 0
	p := NewCodexPreflight(func(context.Context) error {
		calls++
		return &codexAuthRejectedError{status: "401 Unauthorized"}
	})
	for range 2 {
		err := p.Check(context.Background())
		if err == nil || !strings.Contains(err.Error(), "Codex authentication rejected (401 Unauthorized)") || !strings.Contains(err.Error(), "codex login") || !strings.Contains(err.Error(), "halt the current goal loop") {
			t.Fatalf("Check() error = %v, want actionable auth rejection", err)
		}
	}
	if calls != 2 {
		t.Fatalf("probe calls = %d, want failed probes uncached", calls)
	}
}

func TestCodexPreflightTimeoutIsActionableAndNotCached(t *testing.T) {
	calls := 0
	p := NewCodexPreflight(func(ctx context.Context) error {
		calls++
		<-ctx.Done()
		return ctx.Err()
	})
	p.now = func() time.Time { return time.Now().Add(-codexPreflightTimeout) }
	err := p.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "auth service unavailable") || !strings.Contains(err.Error(), "switch to an alternate provider") {
		t.Fatalf("Check() error = %v, want actionable timeout", err)
	}
	if calls != 1 {
		t.Fatalf("probe calls = %d, want 1", calls)
	}
}

func TestCodexPreflightWrapsProbeErrors(t *testing.T) {
	want := errors.New("network down")
	p := NewCodexPreflight(func(context.Context) error { return want })
	err := p.Check(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("Check() error = %v, want wrapped probe error", err)
	}
}
