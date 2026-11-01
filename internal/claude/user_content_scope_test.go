package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/agy"
	"ubunatic.com/harnez/internal/codex"
)

// userHookCommand is a user-authored hook. No selection, scoped or pruned,
// may remove or rewrite it (issue 1061 review item 1).
const userHookCommand = "/usr/local/bin/user-hook"

// seedUserContent writes one user-authored hook and one unknown top-level key
// into each hook file apply manages: Claude settings.json, Codex config.toml
// and Antigravity hooks.json.
func seedUserContent(t *testing.T, target, codexPath, agyPath string) {
	t.Helper()
	writeAdapterTestFile(t, filepath.Join(target, "settings.json"), `{
  "unknownKey": "keep",
  "hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "/usr/local/bin/user-hook"}]}]}
}
`)
	writeAdapterTestFile(t, codexPath, `keep_me = true

[[hooks.PreToolUse]]
matcher = "Bash"

[[hooks.PreToolUse.hooks]]
type = "command"
command = "/usr/local/bin/user-hook"
`)
	writeAdapterTestFile(t, agyPath, `{
  "userSetting": true,
  "userHook": {"enabled": true, "PreToolUse": [{"matcher": "*", "hooks": [{"type": "command", "command": "/usr/local/bin/user-hook"}]}]}
}
`)
}

// assertUserContent fails when a seeded user item is missing from its file.
func assertUserContent(t *testing.T, label, settingsPath, codexPath, agyPath string) {
	t.Helper()
	checks := []struct {
		path  string
		wants []string
	}{
		{settingsPath, []string{`"unknownKey"`, userHookCommand}},
		{codexPath, []string{"keep_me", userHookCommand}},
		{agyPath, []string{`"userSetting"`, `"userHook"`}},
	}
	for _, c := range checks {
		data, err := os.ReadFile(c.path)
		if err != nil {
			t.Fatalf("%s: read %s: %v", label, c.path, err)
		}
		for _, want := range c.wants {
			if !strings.Contains(string(data), want) {
				t.Errorf("%s: %s lost %s:\n%s", label, c.path, want, data)
			}
		}
	}
}

// TestUserContentSurvivesScopedAndPrunedRuns seeds user content, applies the
// full set, then runs scoped selections and --prune selections. The --prune
// runs drop telemetry, so they reach codex.Remove, agy.Remove and the settings
// hooks removal; the test checks that harnez's own hooks are gone there, so a
// passing user-content check cannot come from a path that never ran.
func TestUserContentSurvivesScopedAndPrunedRuns(t *testing.T) {
	target, cfg, _, codexPath, agyPath := reportSetup(t)
	settingsPath := filepath.Join(target, "settings.json")
	seedUserContent(t, target, codexPath, agyPath)

	fullApply := func() {
		t.Helper()
		if _, err := CaptureTestStdout(func() error {
			return ApplyAllVariant(target, cfg, ApplyOptions{})
		}); err != nil {
			t.Fatalf("full apply: %v", err)
		}
	}
	runSelection := func(names []string, prune bool) {
		t.Helper()
		set, err := Resolve(names, nil, nil, prune)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CaptureTestStdout(func() error {
			return ApplyAllVariant(target, cfg, ApplyOptions{Selection: set})
		}); err != nil {
			t.Fatalf("apply %v prune=%v: %v", names, prune, err)
		}
	}

	fullApply()
	assertUserContent(t, "full apply", settingsPath, codexPath, agyPath)

	for _, names := range [][]string{{"skills"}, {"docs"}, {"usage"}, {"telemetry"}} {
		label := "scoped " + strings.Join(names, ",")
		runSelection(names, false)
		assertUserContent(t, label, settingsPath, codexPath, agyPath)
	}

	for _, names := range [][]string{{"skills"}, {"docs"}} {
		fullApply()
		label := "prune " + strings.Join(names, ",")
		runSelection(names, true)
		if installed, _ := codex.Status(codexPath); installed {
			t.Errorf("%s: codex.Remove did not run: harnez hook still installed", label)
		}
		if installed, _ := agy.Status(agyPath); installed {
			t.Errorf("%s: agy.Remove did not run: harnez hook still installed", label)
		}
		assertUserContent(t, label, settingsPath, codexPath, agyPath)
	}
}
