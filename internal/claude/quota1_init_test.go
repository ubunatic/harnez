// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/claude"
)

func TestRunInitWithVariant_Quota1_Scaffolding(t *testing.T) {
	dir := t.TempDir()

	// 1. Create minimal Go project structure
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/q1test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := claude.LoadTestConfig(t)

	// 2. Run RunInitWithVariant with quota-1 = true
	err := claude.RunInitWithVariant(dir, cfg, []string{"golang"}, "", true, false, false, false, nil, false, false, "", true)
	if err != nil {
		t.Fatalf("RunInitWithVariant failed: %v", err)
	}

	// 3. Check AGENTS.md for Quota-1 Guardrails managed section
	agentsPath := filepath.Join(dir, "AGENTS.md")
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	agentsContent := string(data)

	if strings.Contains(agentsContent, "Quota-1 Guardrails") {
		t.Errorf("Quota-1 block should migrate out of AGENTS.md:\n%s", agentsContent)
	}
	quotaData, err := os.ReadFile(filepath.Join(dir, ".harnez", "rules", "Quota.md"))
	if err != nil {
		t.Fatal(err)
	}
	quotaContent := string(quotaData)
	for _, want := range []string{"Single-Test Boundary", "Clean Tree First", "make test-q1", "QUOTA_BYPASS=1"} {
		if !strings.Contains(quotaContent, want) {
			t.Errorf("Quota.md missing %q:\n%s", want, quotaContent)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".harnez", "rules", "Quota.md")); err != nil {
		t.Errorf("expected Quota.md for Quota-1 opt-in: %v", err)
	}

	// 4. Check Makefile for test-q1 target
	makePath := filepath.Join(dir, "Makefile")
	makeData, err := os.ReadFile(makePath)
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	makeContent := string(makeData)

	if !strings.Contains(makeContent, "test-q1:") {
		t.Errorf("expected test-q1 target in Makefile, got:\n%s", makeContent)
	}
	if !strings.Contains(makeContent, "harnez exec --quota-1 -- $(MAKE) test") {
		t.Errorf("expected harnez exec --quota-1 -- $(MAKE) test in Makefile, got:\n%s", makeContent)
	}

	// 5. Idempotency: run again, should succeed without corrupting or duplicating
	err = claude.RunInitWithVariant(dir, cfg, []string{"golang"}, "", true, false, false, false, nil, false, false, "", true)
	if err != nil {
		t.Fatalf("second RunInitWithVariant failed: %v", err)
	}

	data2, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md (second run): %v", err)
	}
	if strings.Contains(string(data2), "Quota-1 Guardrails") {
		t.Errorf("Quota-1 block reappeared in AGENTS.md:\n%s", data2)
	}
}

func TestRunInitWithVariant_Quota1_ExistingMakefile(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/q1existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	existingMakefile := `.PHONY: test
test:
	go test ./...
`
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(existingMakefile), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := claude.LoadTestConfig(t)

	err := claude.RunInitWithVariant(dir, cfg, nil, "", true, false, false, false, nil, false, false, "", true)
	if err != nil {
		t.Fatalf("RunInitWithVariant failed: %v", err)
	}

	makeData, err := os.ReadFile(filepath.Join(dir, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	makeContent := string(makeData)

	// Verify existing target is preserved
	if !strings.Contains(makeContent, "go test ./...") {
		t.Errorf("expected existing test recipe to be preserved, got:\n%s", makeContent)
	}
	// Verify test-q1 target is added
	if !strings.Contains(makeContent, "test-q1:") {
		t.Errorf("expected test-q1 target to be added to existing Makefile, got:\n%s", makeContent)
	}
}

func TestRunInitWithVariant_LiteQuota1_ContentGuidance(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/liteq1test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := claude.LoadTestConfig(t)

	if err := claude.RunInitWithVariant(dir, cfg, []string{"agentic-loop", "issue-tracking", "spec"}, "", true, false, false, false, nil, false, false, "lite", true); err != nil {
		t.Fatalf("RunInitWithVariant failed: %v", err)
	}

	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(data)
	}
	quota := read(filepath.Join(".harnez", "rules", "Quota.md"))
	loop := read("docs/AgenticLoop.md")
	spec := read("docs/Spec.md")
	issues := read("docs/IssueTracking.md")
	normalize := func(content string) string { return strings.Join(strings.Fields(content), " ") }

	if !strings.Contains(normalize(quota), "Media & Demo Verification Gate") || strings.Contains(normalize(quota), "Invariant 10") {
		t.Errorf("Quota.md must refer to the media gate by name without a stale invariant number")
	}
	if !strings.Contains(loop, "7. **Media & Demo Verification Gate**") {
		t.Errorf("lite AgenticLoop.md must include the numbered media gate")
	}
	if !strings.Contains(normalize(spec), "Harnez-specific non-UI example") || !strings.Contains(normalize(spec), "omit this example when absent") {
		t.Errorf("Spec.md must label telemetry as an optional Harnez-specific example")
	}
	if !strings.Contains(normalize(issues), "`/goal` or a clear Goal statement and acceptance criteria") || !strings.Contains(normalize(issues), "new tickets only") || !strings.Contains(normalize(issues), "no backlog migration") {
		t.Errorf("IssueTracking.md must state accepted goal formats and new-ticket-only scope")
	}
}
