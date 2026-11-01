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
)

// captureScopeStdout runs fn with os.Stdout redirected to a file, because apply
// and diff print with fmt.Printf and do not write to the cobra output.
func captureScopeStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdout-*")
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = f
	runErr := fn()
	os.Stdout = old
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data), runErr
}

// scopeCLISetup gives a throwaway HOME, the repo config with jev compaction off,
// and a runner that executes harnez commands and returns their stdout.
func scopeCLISetup(t *testing.T) (target, configPath string, run func(args ...string) string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HARNEZ_DISABLE_RATE_FEEDBACK", "")
	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0755); err != nil {
		t.Fatal(err)
	}
	target = filepath.Join(home, ".claude")
	repoDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	configData, err := os.ReadFile(filepath.Join(repoDir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	configFile, err := os.CreateTemp(repoDir, ".config-test-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	configPath = configFile.Name()
	t.Cleanup(func() { _ = os.Remove(configPath) })
	if err := configFile.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(strings.Replace(string(configData), "jev_compaction_enabled: true", "jev_compaction_enabled: false", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	run = func(args ...string) string {
		t.Helper()
		out, err := captureScopeStdout(t, func() error {
			cmd := newRootCmd()
			cmd.SetArgs(args)
			return cmd.Execute()
		})
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return out
	}
	return target, configPath, run
}

// TestApplyCLI_ComponentsFlagWithoutPruneKeepsOthers is the CLI acceptance
// check for issue 1061 D1: `apply --components docs-only` (no --prune) is a
// scoped run. It leaves the Codex/AGY hooks and the requires-telemetry skill
// installed, and a following `diff --components docs-only` names no removal.
func TestApplyCLI_ComponentsFlagWithoutPruneKeepsOthers(t *testing.T) {
	target, configPath, run := scopeCLISetup(t)
	home := filepath.Dir(target)
	codexPath := filepath.Join(home, ".codex", "config.toml")
	agyPath := filepath.Join(home, ".gemini", "config", "hooks.json")
	rateSkill := filepath.Join(target, "skills", "tool-feedback-protocol", "SKILL.md")

	run("apply", "--config", configPath, "--target", target)
	if _, err := os.Stat(rateSkill); err != nil {
		t.Fatalf("full apply: tool-feedback-protocol SKILL.md missing: %v", err)
	}

	run("apply", "--config", configPath, "--components", "docs-only", "--target", target)

	if _, err := os.Stat(rateSkill); err != nil {
		t.Errorf("scoped docs-only apply removed tool-feedback-protocol SKILL.md: %v", err)
	}
	if installed, _ := codex.Status(codexPath); !installed {
		t.Error("scoped docs-only apply removed the codex hook")
	}
	if installed, _ := agy.Status(agyPath); !installed {
		t.Error("scoped docs-only apply removed the agy hook")
	}
	out := run("diff", "--config", configPath, "--components", "docs-only", "--target", target)
	if strings.Contains(out, "would remove") {
		t.Errorf("scoped diff names a removal:\n%s", out)
	}
	// Positive control: the same selection with --prune does name removals, so
	// the empty result above is not an output-capture failure.
	pruned := run("diff", "--config", configPath, "--components", "docs-only", "--prune", "--target", target)
	if !strings.Contains(pruned, "would remove") {
		t.Errorf("diff --prune names no removal; capture or prune is broken:\n%s", pruned)
	}
}

// TestApplyCLI_PruneRemovesStatusLinesWhenUsageUnselected covers issue 1061 M4:
// desired state without usage removes the managed AGY and Codex status lines,
// as the settings statusLine key does, and a scoped run keeps them.
func TestApplyCLI_PruneRemovesStatusLinesWhenUsageUnselected(t *testing.T) {
	target, configPath, run := scopeCLISetup(t)
	home := filepath.Dir(target)
	codexPath := filepath.Join(home, ".codex", "config.toml")
	agyStatusPath := agy.StatusLineSettingsPath(filepath.Join(home, ".gemini", "antigravity-cli"))

	run("apply", "--config", configPath, "--target", target)
	agyInstalled, _ := agy.StatusLineStatus(agyStatusPath)
	codexInstalled, _ := codex.StatusLineStatus(codexPath)
	if !agyInstalled || !codexInstalled {
		t.Fatalf("baseline: status lines after full apply: agy=%v codex=%v", agyInstalled, codexInstalled)
	}

	run("apply", "--config", configPath, "--components", "docs-only", "--target", target)
	if installed, _ := agy.StatusLineStatus(agyStatusPath); !installed {
		t.Error("scoped docs-only apply removed the agy status line")
	}
	if installed, _ := codex.StatusLineStatus(codexPath); !installed {
		t.Error("scoped docs-only apply removed the codex status items")
	}

	out := run("diff", "--config", configPath, "--components", "docs-only", "--prune", "--target", target)
	if !strings.Contains(out, "would remove "+agyStatusPath+" [statusLine]") {
		t.Errorf("diff --prune does not name the agy status line:\n%s", out)
	}

	run("apply", "--config", configPath, "--components", "docs-only", "--prune", "--target", target)
	if installed, _ := agy.StatusLineStatus(agyStatusPath); installed {
		t.Error("desired-state apply left the agy status line installed")
	}
	if installed, _ := codex.StatusLineStatus(codexPath); installed {
		t.Error("desired-state apply left the codex status items installed")
	}
}
