package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/skillreg"
)

func TestDroppedSkillsOwnershipAndDrift(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	root := t.TempDir()
	cfg := &Config{SkillsTarget: root, Skills: []Command{{Name: "configured"}}}
	for name, markers := range map[string][]string{
		"dropped":    {ManagedSkillMarker},
		"configured": {ManagedSkillMarker},
		"external":   {ManagedSkillMarker, skillreg.MarkerFile},
		"user":       {},
	} {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for _, file := range append(markers, "SKILL.md") {
			if err := os.WriteFile(filepath.Join(dir, file), []byte("keep contents"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Dropped skills drift even when skills are outside the selected components.
	selection := Set{Component("docs"): true}
	if drift, err := diffSkills(cfg, selection); err != nil || !drift {
		t.Fatalf("before cleanup: drift=%v err=%v", drift, err)
	}
	if changes, err := removeDroppedSkills(cfg); err != nil || changes != 1 {
		t.Fatalf("cleanup: changes=%d err=%v", changes, err)
	}
	if _, err := os.Stat(filepath.Join(root, "dropped")); !os.IsNotExist(err) {
		t.Fatalf("dropped directory remains: %v", err)
	}
	for _, name := range []string{"configured", "external", "user"} {
		data, err := os.ReadFile(filepath.Join(root, name, "SKILL.md"))
		if err != nil || string(data) != "keep contents" {
			t.Fatalf("%s was modified: %q, %v", name, data, err)
		}
	}
	if drift, err := diffSkills(cfg, selection); err != nil || drift {
		t.Fatalf("after cleanup: drift=%v err=%v", drift, err)
	}
	if changes, err := removeDroppedSkills(cfg); err != nil || changes != 0 {
		t.Fatalf("second cleanup: changes=%d err=%v", changes, err)
	}
}

func TestDroppedSkillsReadError(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	root := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(root, nil, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{SkillsTarget: root}
	if _, err := removeDroppedSkills(cfg); err == nil || !strings.Contains(err.Error(), "skills dir "+root) {
		t.Fatalf("cleanup error = %v", err)
	}
	if _, err := diffSkills(cfg, nil); err == nil || !strings.Contains(err.Error(), "skills dir "+root) {
		t.Fatalf("diff error = %v", err)
	}
}
