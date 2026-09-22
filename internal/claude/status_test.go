// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStdoutStatus captures stdout output from f, same pattern as telemetry_hook_test.go.
func captureStdoutStatus(f func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	errVal := f()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String(), errVal
}

// TestRunStatus_ChecksAllManagedSettingsKeys verifies that status reports lines for
// every key declared in the built settings doc — not just "model". A config with only
// hooks and permissions (no model) must emit those two keys and must NOT emit model.
func TestRunStatus_ChecksAllManagedSettingsKeys(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Hooks: []Hook{{Event: "PostToolUse", Command: "echo test"}},
		Permissions: Permissions{
			Allow: []string{"Bash(git:*)"},
			Deny:  []string{},
		},
	}

	// apply so settings.json exists with hooks + permissions (but no model)
	if err := ApplyAll(dir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	out, err := captureStdoutStatus(func() error {
		return RunStatus("(test)", cfg, dir, nil)
	})
	if err != nil {
		t.Fatalf("RunStatus failed: %v", err)
	}

	// hooks and permissions are in the doc → must appear in output as ok
	if !strings.Contains(out, "settings.json [hooks]") {
		t.Errorf("expected settings.json [hooks] in output, got:\n%s", out)
	}
	if !strings.Contains(out, "settings.json [permissions]") {
		t.Errorf("expected settings.json [permissions] in output, got:\n%s", out)
	}
	// model is not declared in this config → must NOT appear
	if strings.Contains(out, "settings.json [model]") {
		t.Errorf("settings.json [model] must not appear for a config without model, got:\n%s", out)
	}
}

// TestRunStatus_ReportsRemovedManagedKey verifies that status detects drift:
// after apply, if a managed key is deleted from settings.json, status reports it missing.
func TestRunStatus_ReportsRemovedManagedKey(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Hooks: []Hook{{Event: "PostToolUse", Command: "echo test"}},
	}

	if err := ApplyAll(dir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	settingsPath := filepath.Join(dir, "settings.json")

	// confirm hooks is present after apply
	out, err := captureStdoutStatus(func() error {
		return RunStatus("(test)", cfg, dir, nil)
	})
	if err != nil {
		t.Fatalf("RunStatus (pre-removal) failed: %v", err)
	}
	if !strings.Contains(out, "settings.json [hooks]") || !strings.Contains(out, "ok") {
		t.Fatalf("expected hooks to be ok before removal, got:\n%s", out)
	}

	// manually remove "hooks" from settings.json to simulate drift
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	delete(m, "hooks")
	updated, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(settingsPath, append(updated, '\n'), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// now status must report hooks as missing
	out, err = captureStdoutStatus(func() error {
		return RunStatus("(test)", cfg, dir, nil)
	})
	if err != nil {
		t.Fatalf("RunStatus (post-removal) failed: %v", err)
	}
	if !strings.Contains(out, "settings.json [hooks]") {
		t.Fatalf("expected settings.json [hooks] in output, got:\n%s", out)
	}
	// find the hooks line and verify it says missing
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "settings.json [hooks]") {
			if !strings.Contains(line, "missing") {
				t.Errorf("expected hooks to be 'missing' after removal, got line: %q", line)
			}
			return
		}
	}
	t.Errorf("no settings.json [hooks] line found after removal, full output:\n%s", out)
}

// TestRunStatus_BashShimCodexAndAgyHooks verifies that status reports bash shim, codex hooks, and agy hooks.
func TestRunStatus_BashShimCodexAndAgyHooks(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	dir := filepath.Join(tmpHome, ".claude")
	codexTarget := filepath.Join(tmpHome, ".codex", "config.toml")
	agyTarget := filepath.Join(tmpHome, ".gemini", "config", "hooks.json")
	shimPath := filepath.Join(tmpHome, ".harnez", "shims", "bash")

	// Ensure .gemini home exists so agy hooks are checked
	if err := os.MkdirAll(filepath.Join(tmpHome, ".gemini"), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cfg := &Config{
		CodexHooksTarget: codexTarget,
		AgyHooksTarget:   agyTarget,
	}

	// Before apply -> missing
	out, err := captureStdoutStatus(func() error {
		return RunStatus("(test)", cfg, dir, nil)
	})
	if err != nil {
		t.Fatalf("RunStatus failed: %v", err)
	}
	if !strings.Contains(out, shimPath) || !strings.Contains(out, "missing") {
		t.Errorf("expected bash shim missing, got:\n%s", out)
	}
	if !strings.Contains(out, codexTarget+" [hooks.harnez]") || !strings.Contains(out, "missing") {
		t.Errorf("expected codex hooks missing, got:\n%s", out)
	}
	if !strings.Contains(out, agyTarget+" [harnez]") || !strings.Contains(out, "missing") {
		t.Errorf("expected agy hooks missing, got:\n%s", out)
	}

	// Apply
	if err := ApplyAll(dir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	// After apply -> ok
	out, err = captureStdoutStatus(func() error {
		return RunStatus("(test)", cfg, dir, nil)
	})
	if err != nil {
		t.Fatalf("RunStatus post-apply failed: %v", err)
	}
	if !strings.Contains(out, shimPath) || !strings.Contains(out, "ok") {
		t.Errorf("expected bash shim ok, got:\n%s", out)
	}
	if !strings.Contains(out, codexTarget+" [hooks.harnez]") || !strings.Contains(out, "ok") {
		t.Errorf("expected codex hooks ok, got:\n%s", out)
	}
	if !strings.Contains(out, agyTarget+" [harnez]") || !strings.Contains(out, "ok") {
		t.Errorf("expected agy hooks ok, got:\n%s", out)
	}
}
