// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/agy"
	"ubunatic.com/harnez/internal/codex"
	"ubunatic.com/harnez/internal/jsonc"
)

// TestApplyDiffCLI_ComponentsDocsOnlyRoundTrip is the end-to-end acceptance
// test for issue 491: it drives `apply`/`diff` through newRootCmd() (real CLI
// dispatch, not the internal claude/components APIs directly) so a selected
// apply's removal pass is exercised the same way a user's shell invocation
// would trigger it. `--components docs-only` must actively remove the
// Codex/AGY harnez hooks and the tool-feedback-protocol skill (including its
// resources), leave `diff --components docs-only` clean, and a subsequent
// full apply must reinstall everything with `diff` clean again.
func TestApplyDiffCLI_ComponentsDocsOnlyRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HARNEZ_DISABLE_RATE_FEEDBACK", "")
	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".claude")
	codexPath := filepath.Join(home, ".codex", "config.toml")
	agyPath := filepath.Join(home, ".gemini", "config", "hooks.json")
	rateSkillDir := filepath.Join(home, ".claude", "skills", "tool-feedback-protocol")
	rateSkill := filepath.Join(rateSkillDir, "SKILL.md")

	run := func(args ...string) {
		t.Helper()
		cmd := newRootCmd()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}

	run("apply", "--target", target)

	if _, err := os.Stat(rateSkill); err != nil {
		t.Fatalf("full apply: tool-feedback-protocol SKILL.md missing: %v", err)
	}
	if installed, _ := codex.Status(codexPath); !installed {
		t.Fatal("full apply: codex hook not installed")
	}
	if installed, _ := agy.Status(agyPath); !installed {
		t.Fatal("full apply: agy hook not installed")
	}

	run("apply", "--components", "docs-only", "--target", target)

	if _, err := os.Stat(rateSkill); !os.IsNotExist(err) {
		t.Errorf("docs-only: tool-feedback-protocol SKILL.md left, stat err = %v", err)
	}
	if _, err := os.Stat(rateSkillDir); !os.IsNotExist(err) {
		t.Errorf("docs-only: tool-feedback-protocol skill dir left, stat err = %v", err)
	}
	codexData, _ := os.ReadFile(codexPath)
	if strings.Contains(string(codexData), "harnez ") {
		t.Errorf("docs-only: codex harnez hook left:\n%s", codexData)
	}
	if installed, _ := agy.Status(agyPath); installed {
		t.Error("docs-only: agy harnez hook left")
	}
	doc := jsonc.Read(filepath.Join(target, "settings.json"))
	if hm, ok := doc["hooks"].(map[string]any); ok {
		for _, v := range hm {
			for _, e := range v.([]any) {
				em, _ := e.(map[string]any)
				for _, h := range em["hooks"].([]any) {
					hm2, _ := h.(map[string]any)
					if cmdStr, _ := hm2["command"].(string); strings.HasPrefix(strings.TrimSpace(cmdStr), "harnez") {
						t.Errorf("docs-only: harnez hook %q left in settings", cmdStr)
					}
				}
			}
		}
	}

	diffCmd := newRootCmd()
	var diffOut strings.Builder
	diffCmd.SetOut(&diffOut)
	diffCmd.SetArgs([]string{"diff", "--components", "docs-only", "--target", target, "--exit-code"})
	if err := diffCmd.Execute(); err != nil {
		t.Fatalf("diff --components docs-only reports drift: %v\n%s", err, diffOut.String())
	}

	run("apply", "--target", target)

	if _, err := os.Stat(rateSkill); err != nil {
		t.Fatalf("full apply (2nd): tool-feedback-protocol SKILL.md not reinstalled: %v", err)
	}
	if installed, _ := codex.Status(codexPath); !installed {
		t.Fatal("full apply (2nd): codex hook not reinstalled")
	}

	fullDiffCmd := newRootCmd()
	var fullDiffOut strings.Builder
	fullDiffCmd.SetOut(&fullDiffOut)
	fullDiffCmd.SetArgs([]string{"diff", "--target", target, "--exit-code"})
	if err := fullDiffCmd.Execute(); err != nil {
		t.Fatalf("diff after full re-apply reports drift: %v\n%s", err, fullDiffOut.String())
	}
}
