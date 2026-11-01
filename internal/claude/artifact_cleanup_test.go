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
	report := &applyReport{}
	if err := removeDroppedSkills(report, cfg); err != nil || report.changes != 1 {
		t.Fatalf("cleanup: changes=%d err=%v", report.changes, err)
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
	second := &applyReport{}
	if err := removeDroppedSkills(second, cfg); err != nil || second.changes != 0 {
		t.Fatalf("second cleanup: changes=%d err=%v", second.changes, err)
	}
}

func TestDroppedSkillsReadError(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	root := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(root, nil, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{SkillsTarget: root}
	if err := removeDroppedSkills(&applyReport{}, cfg); err == nil || !strings.Contains(err.Error(), "skills dir "+root) {
		t.Fatalf("cleanup error = %v", err)
	}
	if _, err := diffSkills(cfg, nil); err == nil || !strings.Contains(err.Error(), "skills dir "+root) {
		t.Fatalf("diff error = %v", err)
	}
}

// Dropped and decommissioned removals run regardless of the selection (D2), so
// the end-of-run "removed:" summary must name them with their reason, as it
// does for the other removals (issue 1061 review item 4).
func TestDroppedAndDecommissionedRemovalsAreSummarized(t *testing.T) {
	target, cfg, _, _ := adapterOwnershipSetup(t)
	droppedSkill := filepath.Join(cfg.ClaudeSkillsTarget, "old-skill")
	writeAdapterTestFile(t, filepath.Join(droppedSkill, ManagedSkillMarker), "")
	writeAdapterTestFile(t, filepath.Join(droppedSkill, "SKILL.md"), "obsolete")
	decommissionedSkill := filepath.Join(cfg.ClaudeSkillsTarget, "old-decom")
	writeAdapterTestFile(t, filepath.Join(decommissionedSkill, "SKILL.md"), "obsolete")
	cfg.Decommissioned = DecommissionedConfig{Commands: []string{"old-cmd"}}
	cfg.Decommissioned.Skills = []string{"old-decom"}
	var commandFiles []string
	for _, root := range commandTargets(target, cfg) {
		commandFiles = append(commandFiles, filepath.Join(root, "old-cmd.md"))
		writeAdapterTestFile(t, commandFiles[len(commandFiles)-1], "obsolete")
	}

	out, err := CaptureTestStdout(func() error {
		return ApplyAllVariant(target, cfg, ApplyOptions{})
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := []string{
		droppedSkill + " (not in config)",
		decommissionedSkill + " (decommissioned)",
	}
	for _, file := range commandFiles {
		want = append(want, file+" (decommissioned)")
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("removed summary does not name %q; output:\n%s", w, out)
		}
	}
	if !strings.Contains(out, "removed:") {
		t.Errorf("apply printed no removed: summary line:\n%s", out)
	}
}
