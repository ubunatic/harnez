package assess

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	// Configure user name and email for commits in tests
	_ = exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	_ = exec.Command("git", "-C", dir, "config", "user.email", "test@example.com").Run()
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	_ = exec.Command("git", "-C", dir, "add", "-A").Run()
	cmd := exec.Command("git", "-C", dir, "commit", "-m", msg, "--allow-empty")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, string(out))
	}
}

func TestRAMP_L1_Unconfigured(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Add normal code and human docs (no AI artifacts)
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# My Project\nHuman description.\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte("test:\n\tgo test ./...\n"), 0644)
	commitAll(t, tmpDir, "Initial commit")

	profile, err := AssessRAMP(tmpDir)
	if err != nil {
		t.Fatalf("AssessRAMP failed: %v", err)
	}

	if profile.BaselineLevel != RAMPLevelL1 {
		t.Errorf("BaselineLevel = %v, want %v", profile.BaselineLevel, RAMPLevelL1)
	}
	if profile.ProjectedLevel != RAMPLevelL1 {
		t.Errorf("ProjectedLevel = %v, want %v", profile.ProjectedLevel, RAMPLevelL1)
	}
	if len(profile.Artifacts) != 0 {
		t.Errorf("Artifacts count = %d, want 0", len(profile.Artifacts))
	}
}

func TestRAMP_L2_GroundedPrompting(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("# Agent Guidelines\n- Rule 1\n"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".claude", "settings.json"), []byte(`{"permissions": {}}`), 0644)
	commitAll(t, tmpDir, "Add AI instructions")

	profile, err := AssessRAMP(tmpDir)
	if err != nil {
		t.Fatalf("AssessRAMP failed: %v", err)
	}

	if profile.BaselineLevel != RAMPLevelL2 {
		t.Errorf("BaselineLevel = %v, want %v", profile.BaselineLevel, RAMPLevelL2)
	}
	if profile.ProjectedLevel != RAMPLevelL2 {
		t.Errorf("ProjectedLevel = %v, want %v", profile.ProjectedLevel, RAMPLevelL2)
	}
	if len(profile.Artifacts) != 2 {
		t.Errorf("Artifacts count = %d, want 2", len(profile.Artifacts))
	}
	if len(profile.CoherenceAlerts) != 0 {
		t.Errorf("CoherenceAlerts = %v, want empty", profile.CoherenceAlerts)
	}
}

func TestRAMP_L3_AgentAugmented(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// L2 baseline
	os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("# Agent Guidelines\n"), 0644)
	// L3 domain skill
	os.MkdirAll(filepath.Join(tmpDir, "skills", "test-skill"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "skills", "test-skill", "SKILL.md"), []byte("---\nname: test-skill\ndescription: A test skill\n---\n# Test Skill\n"), 0644)
	// L3 reusable command
	os.MkdirAll(filepath.Join(tmpDir, ".claude", "commands"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".claude", "commands", "review.md"), []byte("# Reusable review command\nRun review procedure.\n"), 0644)
	commitAll(t, tmpDir, "Add L2 and L3 artifacts")

	profile, err := AssessRAMP(tmpDir)
	if err != nil {
		t.Fatalf("AssessRAMP failed: %v", err)
	}

	if profile.BaselineLevel != RAMPLevelL3 {
		t.Errorf("BaselineLevel = %v, want %v", profile.BaselineLevel, RAMPLevelL3)
	}
	if profile.ProjectedLevel != RAMPLevelL3 {
		t.Errorf("ProjectedLevel = %v, want %v", profile.ProjectedLevel, RAMPLevelL3)
	}
	if len(profile.CoherenceAlerts) != 0 {
		t.Errorf("unexpected CoherenceAlerts: %v", profile.CoherenceAlerts)
	}
}

func TestRAMP_L4_Orchestration(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// L2
	os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("# Agent Guidelines\n"), 0644)
	// L3
	os.MkdirAll(filepath.Join(tmpDir, "skills", "audit"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "skills", "audit", "SKILL.md"), []byte("---\nname: audit\ndescription: Audit skill\n---\n"), 0644)
	// L4: committed transcripts
	os.MkdirAll(filepath.Join(tmpDir, ".system_generated", "logs"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".system_generated", "logs", "transcript.jsonl"), []byte(`{"step_index":1,"type":"USER_INPUT"}`+"\n"), 0644)
	// L4: multi-agent flow
	os.MkdirAll(filepath.Join(tmpDir, "flows"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "flows", "triage.yaml"), []byte("workflow: triage\nagents:\n  - role: triage\n  - role: planner\n"), 0644)
	commitAll(t, tmpDir, "Add full L1-L4 artifacts")

	profile, err := AssessRAMP(tmpDir)
	if err != nil {
		t.Fatalf("AssessRAMP failed: %v", err)
	}

	if profile.BaselineLevel != RAMPLevelL4 {
		t.Errorf("BaselineLevel = %v, want %v", profile.BaselineLevel, RAMPLevelL4)
	}
	if profile.ProjectedLevel != RAMPLevelL4 {
		t.Errorf("ProjectedLevel = %v, want %v", profile.ProjectedLevel, RAMPLevelL4)
	}
	if len(profile.CoherenceAlerts) != 0 {
		t.Errorf("unexpected CoherenceAlerts: %v", profile.CoherenceAlerts)
	}
}

func TestRAMP_MissingLowerLevels_CoherenceAlert(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Only L3 skill without L2 AGENTS.md / AI rules
	os.MkdirAll(filepath.Join(tmpDir, "skills", "helper"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "skills", "helper", "SKILL.md"), []byte("---\nname: helper\ndescription: helper skill\n---\n"), 0644)
	commitAll(t, tmpDir, "Add skill only")

	profile, err := AssessRAMP(tmpDir)
	if err != nil {
		t.Fatalf("AssessRAMP failed: %v", err)
	}

	// Score is NOT capped: highest evidenced level is returned
	if profile.BaselineLevel != RAMPLevelL3 {
		t.Errorf("BaselineLevel = %v, want %v", profile.BaselineLevel, RAMPLevelL3)
	}
	// Coherence alert must be emitted
	if len(profile.CoherenceAlerts) == 0 {
		t.Fatalf("expected CoherenceAlerts for missing L2, got none")
	}
	foundL2Alert := false
	for _, alert := range profile.CoherenceAlerts {
		if strings.Contains(alert, "L3") && strings.Contains(alert, "without L2") {
			foundL2Alert = true
			break
		}
	}
	if !foundL2Alert {
		t.Errorf("expected missing L2 alert, got %v", profile.CoherenceAlerts)
	}
}

func TestRAMP_L4_MissingL2AndL3_CoherenceAlert(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Only L4 session transcript without L2 or L3
	os.MkdirAll(filepath.Join(tmpDir, "transcripts"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "transcripts", "session-001.jsonl"), []byte(`{"step_index":1}`+"\n"), 0644)
	commitAll(t, tmpDir, "Add transcript only")

	profile, err := AssessRAMP(tmpDir)
	if err != nil {
		t.Fatalf("AssessRAMP failed: %v", err)
	}

	if profile.BaselineLevel != RAMPLevelL4 {
		t.Errorf("BaselineLevel = %v, want %v", profile.BaselineLevel, RAMPLevelL4)
	}
	if len(profile.CoherenceAlerts) < 2 {
		t.Errorf("expected at least 2 coherence alerts (missing L3 and missing L2), got %v", profile.CoherenceAlerts)
	}
}

func TestRAMP_AvoidFalsePromotions(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Add files that should NOT be classified as AI artifacts
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Human Docs\nArchitecture overview.\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte("test-q1:\n\tharnez exec --quota-1 -- make test\n"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, ".github", "workflows"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".github", "workflows", "ci.yml"), []byte("name: CI\non: [push]\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "service.service"), []byte("[Unit]\nDescription=App service\n"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "docs"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "docs", "architecture.md"), []byte("# Architecture\nDetailed components.\n"), 0644)
	commitAll(t, tmpDir, "Add ordinary project files")

	profile, err := AssessRAMP(tmpDir)
	if err != nil {
		t.Fatalf("AssessRAMP failed: %v", err)
	}

	if profile.BaselineLevel != RAMPLevelL1 {
		t.Errorf("BaselineLevel = %v, want %v (false promotion occurred)", profile.BaselineLevel, RAMPLevelL1)
	}
	if len(profile.Artifacts) != 0 {
		t.Errorf("Artifacts count = %d, want 0", len(profile.Artifacts))
	}
}

func TestRAMP_AmbiguousPaths_MarkedUnknown(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	os.MkdirAll(filepath.Join(tmpDir, "scripts"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "scripts", "agent_helper.sh"), []byte("#!/bin/bash\necho helper\n"), 0755)
	commitAll(t, tmpDir, "Add ambiguous script")

	profile, err := AssessRAMP(tmpDir)
	if err != nil {
		t.Fatalf("AssessRAMP failed: %v", err)
	}

	// Should remain L1 because ambiguous artifact has ConfidenceUnknown
	if profile.BaselineLevel != RAMPLevelL1 {
		t.Errorf("BaselineLevel = %v, want %v", profile.BaselineLevel, RAMPLevelL1)
	}
	if len(profile.Artifacts) != 1 {
		t.Fatalf("Artifacts count = %d, want 1", len(profile.Artifacts))
	}
	if profile.Artifacts[0].Confidence != ConfidenceUnknown {
		t.Errorf("Confidence = %v, want %v", profile.Artifacts[0].Confidence, ConfidenceUnknown)
	}
}

func TestRAMP_BaselineVsProjectedAndUncommitted(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Clean commit with L1
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644)
	commitAll(t, tmpDir, "Initial commit")

	// Create uncommitted working-tree AGENTS.md
	os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("# Agent Guidelines\n"), 0644)

	profile, err := AssessRAMP(tmpDir)
	if err != nil {
		t.Fatalf("AssessRAMP failed: %v", err)
	}

	// Baseline is still L1 because AGENTS.md is uncommitted!
	if profile.BaselineLevel != RAMPLevelL1 {
		t.Errorf("BaselineLevel = %v, want %v", profile.BaselineLevel, RAMPLevelL1)
	}
	// Projected level is L2 because AGENTS.md exists in working tree
	if profile.ProjectedLevel != RAMPLevelL2 {
		t.Errorf("ProjectedLevel = %v, want %v", profile.ProjectedLevel, RAMPLevelL2)
	}

	// Now assess with projected init files (e.g. docs/practices/AgenticLoop.md)
	profileWithProj, err := AssessRAMPWithProjected(tmpDir, []string{"docs/practices/AgenticLoop.md"})
	if err != nil {
		t.Fatalf("AssessRAMPWithProjected failed: %v", err)
	}

	if profileWithProj.BaselineLevel != RAMPLevelL1 {
		t.Errorf("BaselineLevel = %v, want %v", profileWithProj.BaselineLevel, RAMPLevelL1)
	}
	if profileWithProj.ProjectedLevel != RAMPLevelL2 {
		t.Errorf("ProjectedLevel = %v, want %v", profileWithProj.ProjectedLevel, RAMPLevelL2)
	}

	// Now commit AGENTS.md
	commitAll(t, tmpDir, "Commit AGENTS.md")

	profileAfterCommit, err := AssessRAMP(tmpDir)
	if err != nil {
		t.Fatalf("AssessRAMP failed: %v", err)
	}
	if profileAfterCommit.BaselineLevel != RAMPLevelL2 {
		t.Errorf("BaselineLevel after commit = %v, want %v", profileAfterCommit.BaselineLevel, RAMPLevelL2)
	}
}

func TestRAMP_NonGitDirectory(t *testing.T) {
	tmpDir := t.TempDir() // Not a git repo

	os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("# Agent Guidelines\n"), 0644)

	profile, err := AssessRAMP(tmpDir)
	if err != nil {
		t.Fatalf("AssessRAMP failed: %v", err)
	}

	if profile.GitAvailable {
		t.Errorf("expected GitAvailable to be false")
	}
	if profile.GitNotice == "" {
		t.Errorf("expected GitNotice to be populated")
	}
	// In non-git directory, files on disk are uncommitted, so baseline is L1, projected is L2
	if profile.BaselineLevel != RAMPLevelL1 {
		t.Errorf("BaselineLevel = %v, want %v", profile.BaselineLevel, RAMPLevelL1)
	}
	if profile.ProjectedLevel != RAMPLevelL2 {
		t.Errorf("ProjectedLevel = %v, want %v", profile.ProjectedLevel, RAMPLevelL2)
	}
}

func TestRAMP_Rendering(t *testing.T) {
	profile := &RAMPProfile{
		RuleSetVersion: "ramp-v1",
		BaselineLevel:  RAMPLevelL2,
		ProjectedLevel: RAMPLevelL3,
		GitAvailable:   true,
		Artifacts: []RAMPArtifact{
			{
				Path:       "AGENTS.md",
				Category:   RAMPCatAIRules,
				Level:      RAMPLevelL2,
				RuleID:     "RULE_AGENTS_MD",
				Reason:     "Canonical agent instructions",
				Confidence: ConfidenceHigh,
				Status:     StatusCommitted,
				IsBaseline: true,
			},
			{
				Path:       "skills/test/SKILL.md",
				Category:   RAMPCatDomainSkills,
				Level:      RAMPLevelL3,
				RuleID:     "RULE_DOMAIN_SKILLS",
				Reason:     "Domain skill",
				Confidence: ConfidenceHigh,
				Status:     StatusProjected,
			},
		},
	}

	text := RenderRAMPText(profile)
	if !strings.Contains(text, "RAMP Repository AI Maturity Profile") {
		t.Errorf("RenderRAMPText missing header: %s", text)
	}
	if !strings.Contains(text, "Baseline Level:    L2") {
		t.Errorf("RenderRAMPText missing baseline level: %s", text)
	}

	initText := RenderRAMPInitText(profile)
	if !strings.Contains(initText, "RAMP Maturity Profile:") {
		t.Errorf("RenderRAMPInitText missing header: %s", initText)
	}
	if !strings.Contains(initText, "Projected Evidence") {
		t.Errorf("RenderRAMPInitText missing projected evidence: %s", initText)
	}

	jsonStr, err := RenderRAMPJSON(profile)
	if err != nil {
		t.Fatalf("RenderRAMPJSON failed: %v", err)
	}
	var parsed RAMPProfile
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}
	if parsed.BaselineLevel != RAMPLevelL2 {
		t.Errorf("parsed BaselineLevel = %v, want %v", parsed.BaselineLevel, RAMPLevelL2)
	}
}
