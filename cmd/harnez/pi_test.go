// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez"
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

func TestPiBackgroundUpdateConfigTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("PI_CODING_AGENT_DIR", "")
	configDir := filepath.Join(home, "custom")
	if err := os.MkdirAll(filepath.Join(configDir, "spec"), 0700); err != nil {
		t.Fatal(err)
	}
	spec, err := harnez.DefaultFS.ReadFile("spec/pi-background-tasks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "spec/pi-background-tasks.yaml"), spec, 0600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(home, "chosen-pi")
	config := filepath.Join(configDir, "config.yaml")
	content := []byte("pi_agent_target: " + root + "\n")
	if err := os.WriteFile(config, content, 0600); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(home, "unrelated-config")
	if err := os.WriteFile(unrelated, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	before := map[string]os.FileInfo{}
	for _, file := range []string{config, unrelated, filepath.Join(configDir, "spec/pi-background-tasks.yaml")} {
		info, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		before[file] = info
	}
	cmd := newPiCmd()
	cmd.SetArgs([]string{"background-tasks", "update", "-c", config})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "extensions/background-tasks/package.json")); err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]string{config: string(content), unrelated: "keep", filepath.Join(configDir, "spec/pi-background-tasks.yaml"): string(spec)} {
		got, err := os.ReadFile(file)
		if err != nil || string(got) != want {
			t.Fatalf("configuration changed: %s: %v", file, err)
		}
		info, err := os.Stat(file)
		if err != nil || info.Mode() != before[file].Mode() || !info.ModTime().Equal(before[file].ModTime()) {
			t.Fatalf("configuration metadata changed: %s: %v", file, err)
		}
	}
}
