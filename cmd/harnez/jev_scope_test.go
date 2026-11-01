// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// jevKeptText is the start of the warning a scoped run prints while jev
// compaction is off and its plugin is installed (issue 1061 owner decision).
// It must match jevKeptWarning in internal/claude/jev_compaction.go.
const jevKeptText = "experimental Jev compaction plugin was detected and not removed"

// setJevCompaction rewrites configPath from the repo config with
// jev_compaction_enabled set as requested.
func setJevCompaction(t *testing.T, configPath string, enabled bool) {
	t.Helper()
	repoDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(repoDir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	want := "false"
	if enabled {
		want = "true"
		text = strings.Replace(text, "jev_compaction_enabled: false", "jev_compaction_enabled: true", 1)
	} else {
		text = strings.Replace(text, "jev_compaction_enabled: true", "jev_compaction_enabled: false", 1)
	}
	if !strings.Contains(text, "jev_compaction_enabled: "+want) {
		t.Fatalf("repo config has no jev_compaction_enabled line to set to %s", want)
	}
	if err := os.WriteFile(configPath, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

// jevPluginPresent reports whether the jev plugin cache and its settings entry
// are both under target, which is what a successful install leaves behind.
func jevPluginPresent(t *testing.T, target string) bool {
	t.Helper()
	if _, err := os.Stat(filepath.Join(target, "plugins", "cache", "harnez-local", "jev-compaction")); err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(target, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Contains(string(data), "jev-compaction@harnez-local")
}

// runCLIErr executes one harnez command and returns its stdout and error, so a
// test can check the exit status of `diff --exit-code`.
func runCLIErr(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return captureScopeStdout(t, func() error {
		cmd := newRootCmd()
		cmd.SetArgs(args)
		return cmd.Execute()
	})
}

// installJevPlugin runs a full apply with jev compaction on, so the plugin is
// present, then switches the config to jev off for the run under test.
func installJevPlugin(t *testing.T, target, configPath string, run func(args ...string) string) {
	t.Helper()
	setJevCompaction(t, configPath, true)
	run("apply", "--config", configPath, "--target", target)
	if !jevPluginPresent(t, target) {
		t.Fatal("baseline: jev plugin not installed after apply with jev on")
	}
	setJevCompaction(t, configPath, false)
}

// TestApplyCLI_JevPluginKeptByScopedRun covers issue 1061 (owner decision): with
// jev compaction off, a scoped run leaves the installed jev plugin in place, warns
// about it, reports no change on repeat, and diff matches apply.
func TestApplyCLI_JevPluginKeptByScopedRun(t *testing.T) {
	target, configPath, run := scopeCLISetup(t)
	installJevPlugin(t, target, configPath, run)
	jevRoot := filepath.Join(target, "plugins", "cache", "harnez-local", "jev-compaction")
	scoped := []string{"--config", configPath, "--components", "docs-only", "--target", target}

	out := run(append([]string{"apply"}, scoped...)...)
	if !strings.Contains(out, jevKeptText) {
		t.Errorf("scoped apply printed no jev warning:\n%s", out)
	}
	if strings.Contains(out, jevRoot) {
		t.Errorf("scoped apply names the jev plugin path as changed:\n%s", out)
	}
	if !jevPluginPresent(t, target) {
		t.Fatal("scoped apply removed the jev plugin")
	}

	repeat := run(append([]string{"apply"}, scoped...)...)
	if !strings.Contains(repeat, "No changes.") {
		t.Errorf("repeat scoped apply does not report No changes.:\n%s", repeat)
	}
	if !strings.Contains(repeat, jevKeptText) {
		t.Errorf("repeat scoped apply dropped the jev warning:\n%s", repeat)
	}

	diffOut, err := runCLIErr(t, append([]string{"diff", "--exit-code"}, scoped...)...)
	if err != nil {
		t.Errorf("scoped diff --exit-code reports drift for the kept jev plugin: %v\n%s", err, diffOut)
	}
	if !strings.Contains(diffOut, jevKeptText) {
		t.Errorf("scoped diff does not show the jev warning that apply prints:\n%s", diffOut)
	}
	if !jevPluginPresent(t, target) {
		t.Fatal("diff removed the jev plugin")
	}
}

// TestApplyCLI_JevPluginRemovedWhenNotScoped covers issue 1061 (owner decision):
// plain apply and --prune remove the jev plugin while jev compaction is off, report
// the removal, and diff names it before apply runs.
func TestApplyCLI_JevPluginRemovedWhenNotScoped(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
	}{
		{name: "plain apply", flags: nil},
		{name: "prune", flags: []string{"--components", "docs-only", "--prune"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target, configPath, run := scopeCLISetup(t)
			installJevPlugin(t, target, configPath, run)
			jevRoot := filepath.Join(target, "plugins", "cache", "harnez-local", "jev-compaction")
			args := append([]string{"--config", configPath, "--target", target}, tc.flags...)

			diffOut, err := runCLIErr(t, append([]string{"diff", "--exit-code"}, args...)...)
			if err == nil {
				t.Errorf("diff --exit-code reports no drift before the jev removal:\n%s", diffOut)
			}
			if !strings.Contains(diffOut, "would remove "+jevRoot) {
				t.Errorf("diff does not name the jev removal apply will make:\n%s", diffOut)
			}

			out := run(append([]string{"apply"}, args...)...)
			if jevPluginPresent(t, target) {
				t.Error("jev plugin still installed after the run")
			}
			want := jevRoot + " (jev_compaction_enabled is false)"
			if !strings.Contains(out, "removed: ") || !strings.Contains(out, want) {
				t.Errorf("apply does not report the jev removal in its removed: summary; want %q:\n%s", want, out)
			}
			if strings.Contains(out, jevKeptText) {
				t.Errorf("apply printed the kept-jev warning on a run that removes jev:\n%s", out)
			}

			repeat := run(append([]string{"apply"}, args...)...)
			if !strings.Contains(repeat, "No changes.") {
				t.Errorf("repeat run after the jev removal does not report No changes.:\n%s", repeat)
			}
		})
	}
}

// TestApplyCLI_JevPluginSettingOnScopedRunUnchanged checks that the scoped run is
// unchanged while jev compaction is on: the plugin stays, and no warning is printed.
func TestApplyCLI_JevPluginSettingOnScopedRunUnchanged(t *testing.T) {
	target, configPath, run := scopeCLISetup(t)
	setJevCompaction(t, configPath, true)
	run("apply", "--config", configPath, "--target", target)

	out := run("apply", "--config", configPath, "--components", "docs-only", "--target", target)
	if strings.Contains(out, jevKeptText) {
		t.Errorf("scoped apply with jev on printed the kept-jev warning:\n%s", out)
	}
	if !jevPluginPresent(t, target) {
		t.Error("scoped apply with jev on removed the jev plugin")
	}
}
