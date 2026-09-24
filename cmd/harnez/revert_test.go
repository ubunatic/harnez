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

func TestCleanCommandAvailableForProcessAndQuotaTargets(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	cmd := newRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"clean", "-d", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("clean command error = %v, want successful dry-run", err)
	}
	if !strings.Contains(stdout.String(), "procs: no recorded process groups") || !strings.Contains(stdout.String(), "q1: unchanged") {
		t.Fatalf("clean output = %q; want process and quota dry-run results", stdout.String())
	}
}
