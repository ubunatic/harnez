// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveDecommissionedArtifacts(t *testing.T) {
	target := t.TempDir()
	skillsRoot := t.TempDir()
	cfg := &Config{
		SkillsTarget: skillsRoot,
		Decommissioned: DecommissionedConfig{
			Commands: []string{"fresh-sprint"},
			Skills:   []string{"fresh-sprint", "fresh-sprinter"},
		},
	}

	stale := []string{
		filepath.Join(target, "commands", "fresh-sprint.md"),
		filepath.Join(skillsRoot, "fresh-sprint", "SKILL.md"),
		filepath.Join(skillsRoot, "fresh-sprinter", "SKILL.md"),
	}
	for _, path := range stale {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte("obsolete"), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	keep := filepath.Join(skillsRoot, "lean-sprint", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(keep), 0755); err != nil {
		t.Fatalf("mkdir keep: %v", err)
	}
	if err := os.WriteFile(keep, []byte("current"), 0644); err != nil {
		t.Fatalf("write keep: %v", err)
	}

	changed, err := removeDecommissionedArtifacts(target, cfg)
	if err != nil {
		t.Fatalf("removeDecommissionedArtifacts: %v", err)
	}
	if changed != len(stale) {
		t.Fatalf("changed = %d, want %d", changed, len(stale))
	}
	for _, path := range stale {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("stale artifact still exists: %s (err=%v)", path, err)
		}
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("current skill was removed: %v", err)
	}
}

func TestRemoveDecommissionedArtifactsRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "must-not-remove")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(outside) })

	cfg := &Config{
		SkillsTarget: root,
		Decommissioned: DecommissionedConfig{
			Skills: []string{"../must-not-remove"},
		},
	}
	if _, err := removeDecommissionedArtifacts(t.TempDir(), cfg); err == nil {
		t.Fatal("expected traversal-like skill name to fail")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside directory was touched: %v", err)
	}
}
