package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestApplyDiffRepairCycle is the hermetic form of the former
// scripts/smoke-test.sh drift run (issue 1254): a repeat apply leaves
// settings.json byte- and mtime-identical, a dropped permission makes
// `diff --exit-code` fail while plain `diff` succeeds, and apply restores it.
func TestApplyDiffRepairCycle(t *testing.T) {
	target, configPath, run := scopeCLISetup(t)
	flags := []string{"--config", configPath, "--target", target}
	settingsPath := filepath.Join(target, "settings.json")

	run(append([]string{"apply"}, flags...)...)
	before, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	if out := run(append([]string{"apply"}, flags...)...); !strings.Contains(out, "No changes.") {
		t.Errorf("second apply is not idempotent:\n%s", out)
	}
	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	afterInfo, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("second apply changed settings.json bytes")
	}
	if !afterInfo.ModTime().Equal(beforeInfo.ModTime()) {
		t.Errorf("second apply changed settings.json mtime: %v -> %v", beforeInfo.ModTime(), afterInfo.ModTime())
	}

	dropped := dropAllowedPermissions(t, settingsPath, "Bash(git")
	if len(dropped) == 0 {
		t.Fatal("baseline: apply installed no Bash(git ...) permission to drop")
	}

	if out, err := runCLIErr(t, append([]string{"diff", "--exit-code"}, flags...)...); err == nil {
		t.Errorf("diff --exit-code succeeded despite dropped permissions:\n%s", out)
	}
	if out, err := runCLIErr(t, append([]string{"diff"}, flags...)...); err != nil {
		t.Errorf("diff without --exit-code failed under drift: %v\n%s", err, out)
	}

	if out := run(append([]string{"apply"}, flags...)...); !strings.Contains(out, "permissions.allow") {
		t.Errorf("apply after drift does not report the permissions.allow repair:\n%s", out)
	}
	restored := allowedPermissions(t, settingsPath)
	for _, perm := range dropped {
		if !slices.Contains(restored, perm) {
			t.Errorf("apply did not restore %q", perm)
		}
	}
	if out, err := runCLIErr(t, append([]string{"diff", "--exit-code"}, flags...)...); err != nil {
		t.Errorf("diff --exit-code still reports drift after repair: %v\n%s", err, out)
	}
	if out := run(append([]string{"apply"}, flags...)...); !strings.Contains(out, "No changes.") {
		t.Errorf("apply after repair is not idempotent:\n%s", out)
	}
}

// dropAllowedPermissions removes every permissions.allow entry starting with
// prefix from the settings file and returns the removed entries.
func dropAllowedPermissions(t *testing.T, path, prefix string) []string {
	t.Helper()
	settings := readSettingsMap(t, path)
	perms, _ := settings["permissions"].(map[string]any)
	if perms == nil {
		t.Fatalf("%s has no permissions object", path)
	}
	allow, _ := perms["allow"].([]any)
	var kept []any
	var dropped []string
	for _, entry := range allow {
		if s, ok := entry.(string); ok && strings.HasPrefix(s, prefix) {
			dropped = append(dropped, s)
			continue
		}
		kept = append(kept, entry)
	}
	perms["allow"] = kept
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return dropped
}

func allowedPermissions(t *testing.T, path string) []string {
	t.Helper()
	perms, _ := readSettingsMap(t, path)["permissions"].(map[string]any)
	allow, _ := perms["allow"].([]any)
	var out []string
	for _, entry := range allow {
		if s, ok := entry.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func readSettingsMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return settings
}
