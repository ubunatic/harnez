//go:build integration

// This file is build-tagged `integration` specifically so it never runs
// as part of the default `go test ./...` suite (issue 204 requires the
// fast unit suite to stay free of network/subprocess calls and not depend
// on `claude` being installed/authenticated in CI). Run it explicitly
// with: go test -tags=integration ./internal/privacy/...
package privacy

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

// TestClaudeCLISanitizer_RealInvocation shells out to the real `claude`
// CLI for exactly one note and sanity-checks it gets back non-empty
// sanitized text. Skipped gracefully if `claude` isn't on PATH (e.g. a CI
// runner without Claude Code installed).
func TestClaudeCLISanitizer_RealInvocation(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not found on PATH, skipping integration test")
	}

	s := NewClaudeCLISanitizer()
	notes := []string{"Fixed an internal authentication bug in the client Acme Corp repository."}

	got, err := s.SanitizeBatch(context.Background(), notes)
	if err != nil {
		t.Fatalf("SanitizeBatch: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 sanitized note, got %d: %v", len(got), got)
	}
	if strings.TrimSpace(got[0]) == "" {
		t.Fatal("expected non-empty sanitized text, got empty string")
	}
	t.Logf("sanitized: %q", got[0])
}
