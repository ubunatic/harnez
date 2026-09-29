package claude

import (
	"os"
	"path/filepath"
	"testing"

	"ubunatic.com/harnez/internal/skillreg"
)

func TestExternalSkillClash(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "website")
	if externalSkillClash(dir) {
		t.Fatal("missing dir reported as external")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if externalSkillClash(dir) {
		t.Fatal("plain dir reported as external")
	}
	if err := os.WriteFile(filepath.Join(dir, skillreg.MarkerFile), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !externalSkillClash(dir) {
		t.Fatal("marked dir not reported as external")
	}
}

func TestSkillTargetsByAgentPrefersExplicitLabelOnSharedDir(t *testing.T) {
	shared := t.TempDir()
	cfg := &Config{SkillsTarget: shared, ClaudeSkillsTarget: shared, CodexSkillsTarget: filepath.Join(shared, "codex")}
	got := SkillTargetsByAgent(cfg)
	if len(got) != 2 || got[0].Dir != shared || got[0].Agent != "claude" || got[1].Agent != "codex" {
		t.Fatalf("targets: %+v", got)
	}
}

func TestStatusAndDiffSkipExternalSkillUnderManagedName(t *testing.T) {
	root := t.TempDir()
	cfg := &Config{ClaudeSkillsTarget: root, SkillsTarget: root, Skills: []Command{{Name: "website", Content: "managed"}}}
	dir := filepath.Join(root, "website")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("external"), 0o644)
	os.WriteFile(filepath.Join(dir, skillreg.MarkerFile), nil, 0o644)
	if !isExternalSkill(dir) {
		t.Fatal("marker not detected")
	}
	for _, target := range skillTargets(cfg) {
		if !isExternalSkill(filepath.Join(target, "website")) {
			t.Errorf("target %s: external skill would be treated as managed", target)
		}
	}
}
