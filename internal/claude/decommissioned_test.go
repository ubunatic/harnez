// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"os"
	"path/filepath"
	"testing"

	"ubunatic.com/harnez/internal/skillreg"
)

func TestRemoveDecommissionedArtifacts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PI_CODING_AGENT_DIR", "")
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

func TestRemoveDecommissionedSkillOwnership(t *testing.T) {
	for _, tc := range []struct {
		name   string
		marker string
		keep   bool
	}{
		{name: "bundled", marker: "bundled: true\nurl: https://example.com/skills\n"},
		{name: "registry", marker: "url: https://example.com/skills\n", keep: true},
		{name: "registry-not-bundled", marker: "bundled: false\nurl: https://example.com/skills\n", keep: true},
		{name: "unmarked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("PI_CODING_AGENT_DIR", "")
			cfg := &Config{
				SkillsTarget:       "~/.gemini/skills",
				CodexSkillsTarget:  "~/.codex/skills",
				ClaudeSkillsTarget: "~/.claude/skills",
				PrimeAgentTarget:   "~/.prime/agent",
				PiAgentTarget:      "~/.pi/agent",
				Decommissioned:     DecommissionedConfig{Skills: []string{"domain-modeling"}},
			}
			// Enumerate roots independently so missing cleanup targets fail the test.
			roots := []string{".claude/skills", ".codex/skills", ".gemini/skills", ".prime/agent/skills", ".pi/agent/skills"}
			for _, root := range roots {
				dir := filepath.Join(home, root, "domain-modeling")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("obsolete"), 0o644); err != nil {
					t.Fatal(err)
				}
				if tc.marker != "" {
					if err := os.WriteFile(filepath.Join(dir, skillreg.MarkerFile), []byte(tc.marker), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			changed, err := removeDecommissionedArtifacts(filepath.Join(home, ".claude"), cfg)
			if err != nil {
				t.Fatal(err)
			}
			wantChanged := len(roots)
			if tc.keep {
				wantChanged = 0
			}
			if changed != wantChanged {
				t.Errorf("changed = %d, want %d", changed, wantChanged)
			}
			for _, root := range roots {
				t.Run(root, func(t *testing.T) {
					dir := filepath.Join(home, root, "domain-modeling")
					if !tc.keep {
						if _, err := os.Stat(dir); !os.IsNotExist(err) {
							t.Errorf("decommissioned directory remains: %v", err)
						}
						return
					}
					for name, want := range map[string]string{"SKILL.md": "obsolete", skillreg.MarkerFile: tc.marker} {
						if data, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(data) != want {
							t.Errorf("registry file %s changed: content=%q, err=%v", name, data, err)
						}
					}
				})
			}
			if changed, err := removeDecommissionedArtifacts(filepath.Join(home, ".claude"), cfg); err != nil || changed != 0 {
				t.Errorf("second cleanup = (%d, %v), want (0, nil)", changed, err)
			}
		})
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
