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

func TestReconcileMakeTargets_Variants(t *testing.T) {
	oursTargets := `.PHONY: ⚙️ 🤖
help: ## 🤖 Print help for targets
	@echo "Available targets:"
`

	tests := []struct {
		name           string
		initial        string
		phonyFix       string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "Makefile with text glyph sentinel variant (vimconf style)",
			initial: `.PHONY: ⚙ 🤖

build:
	@echo "building"
`,
			phonyFix: "ours",
			wantContains: []string{
				"build:",
				"help: ## 🤖 Print help for targets",
			},
		},
		{
			name: "Makefile with multiple separate .PHONY lines (vkfusion style)",
			initial: `.PHONY: ⚙️ 🤖
.PHONY: all build test clean

all: build test

build:
	@echo "building"
`,
			phonyFix: "ours",
			wantContains: []string{
				".PHONY: ⚙️ 🤖",
				".PHONY: all build test clean",
				"help: ## 🤖 Print help for targets",
			},
		},
		{
			name: "Makefile with legacy marker block cleanup",
			initial: `.PHONY: all

# claudeconfig:begin targets
help:
	@echo "old help"
# claudeconfig:end targets

all:
	@echo "all"
`,
			phonyFix: "ours",
			wantContains: []string{
				"all:",
				"help: ## 🤖 Print help for targets",
			},
			wantNotContain: []string{
				"# claudeconfig:begin targets",
				"# claudeconfig:end targets",
			},
		},
		{
			name: "Makefile without any .PHONY header",
			initial: `build:
	@echo "building"
`,
			phonyFix: "ours",
			wantContains: []string{
				".PHONY: ⚙️ 🤖",
				"build:",
				"help: ## 🤖 Print help for targets",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			makefilePath := filepath.Join(dir, "Makefile")
			if err := os.WriteFile(makefilePath, []byte(tc.initial), 0o644); err != nil {
				t.Fatal(err)
			}

			cfg := claude.MakeConfig{
				PhonyFix: tc.phonyFix,
			}

			changed, err := claude.ReconcileMakeTargets(makefilePath, oursTargets, cfg, true, nil)
			if err != nil {
				t.Fatalf("ReconcileMakeTargets failed: %v", err)
			}
			if !changed {
				t.Errorf("Expected changed=true for reconciliation")
			}

			content, err := os.ReadFile(makefilePath)
			if err != nil {
				t.Fatal(err)
			}
			s := string(content)

			for _, want := range tc.wantContains {
				if !strings.Contains(s, want) {
					t.Errorf("Expected Makefile to contain %q, got:\n%s", want, s)
				}
			}
			for _, notWant := range tc.wantNotContain {
				if strings.Contains(s, notWant) {
					t.Errorf("Expected Makefile NOT to contain %q, got:\n%s", notWant, s)
				}
			}

			// Idempotency: second run should report changed=false
			changedSecond, err := claude.ReconcileMakeTargets(makefilePath, oursTargets, cfg, true, nil)
			if err != nil {
				t.Fatalf("Second ReconcileMakeTargets failed: %v", err)
			}
			if changedSecond {
				t.Errorf("Expected changed=false on second run (idempotency failure)")
			}
		})
	}
}
