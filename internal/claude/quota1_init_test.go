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

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	// 2. Run RunInitWithVariant with quota-1 = true
	err = claude.RunInitWithVariant(dir, cfg, []string{"golang"}, "", true, false, false, false, nil, false, false, "", true)
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

	if !strings.Contains(agentsContent, "<!-- harnez:begin Quota-1 Guardrails -->") {
		t.Errorf("expected harnez:begin Quota-1 Guardrails in AGENTS.md, got:\n%s", agentsContent)
	}
	if !strings.Contains(agentsContent, "<!-- harnez:end Quota-1 Guardrails -->") {
		t.Errorf("expected harnez:end Quota-1 Guardrails in AGENTS.md, got:\n%s", agentsContent)
	}
	if !strings.Contains(agentsContent, "Single-Test Boundary") {
		t.Errorf("expected Single-Test Boundary rule in AGENTS.md")
	}
	if strings.Contains(agentsContent, "Report Untested Edits") {
		t.Errorf("unexpected Report Untested Edits rule in AGENTS.md")
	}
	if !strings.Contains(agentsContent, "make test-q1") {
		t.Errorf("expected make test-q1 reference in AGENTS.md")
	}
	if !strings.Contains(agentsContent, "QUOTA_BYPASS=1") {
		t.Errorf("expected QUOTA_BYPASS=1 reference in AGENTS.md")
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
	if strings.Count(string(data2), "<!-- harnez:begin Quota-1 Guardrails -->") != 1 {
		t.Errorf("expected exactly 1 Quota-1 Guardrails block in AGENTS.md, found %d", strings.Count(string(data2), "<!-- harnez:begin Quota-1 Guardrails -->"))
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

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	err = claude.RunInitWithVariant(dir, cfg, nil, "", true, false, false, false, nil, false, false, "", true)
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
