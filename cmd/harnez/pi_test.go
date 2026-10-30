// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiBackgroundUpdateCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	root := filepath.Join(home, "pi")
	t.Setenv("PI_CODING_AGENT_DIR", root)
	cmd := newPiCmd()
	cmd.SetArgs([]string{"background-tasks", "update"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, "extensions", "background-tasks", "package.json")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatal(err)
	}
	cmd = newPiCmd()
	cmd.SetArgs([]string{"background-tasks", "update", "extra"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected extra argument refusal")
	}
	cmd = newPiCmd()
	update, _, err := cmd.Find([]string{"background-tasks", "update"})
	if err != nil || !strings.Contains(update.Short, "only") || update.Flags().Lookup("config") == nil {
		t.Fatalf("update help/flags: %v", err)
	}
}
