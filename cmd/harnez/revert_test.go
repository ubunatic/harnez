// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRevertManagedRemovesManagedSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, ".claude")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(target, "settings.json")
	if err := os.WriteFile(settings, []byte(`{"model":"managed-model","theme":"user-theme"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"revert", "--managed", "--target", target})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("revert --managed: %v (stderr: %s)", err, stderr.String())
	}

	data, err := os.ReadFile(settings)
	if err != nil {
		t.Fatalf("read settings after revert: %v", err)
	}
	if strings.Contains(string(data), "managed-model") || !strings.Contains(string(data), "user-theme") {
		t.Fatalf("managed cleanup did not preserve only user settings: %s", data)
	}
}

func TestCleanCommandRemoved(t *testing.T) {
	cmd := newRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"clean"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("clean command error = %v, want unknown command", err)
	}
}
