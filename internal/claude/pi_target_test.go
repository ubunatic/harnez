// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPiAgentTargetResolution(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := &Config{
		PiAgentTarget: "~/.pi/agent",
	}

	wantRoot := filepath.Join(home, ".pi", "agent")
	if got := piAgentRoot(cfg); got != wantRoot {
		t.Errorf("piAgentRoot() = %q, want %q", got, wantRoot)
	}

	// Environment variable PI_CODING_AGENT_DIR overrides cfg.PiAgentTarget
	customPiDir := filepath.Join(t.TempDir(), "custom-pi")
	t.Setenv("PI_CODING_AGENT_DIR", customPiDir)

	if got := piAgentRoot(cfg); got != customPiDir {
		t.Errorf("piAgentRoot() with PI_CODING_AGENT_DIR = %q, want %q", got, customPiDir)
	}

	targets := skillTargets(cfg)
	expectedPiSkills := filepath.Join(customPiDir, "skills")
	found := false
	for _, target := range targets {
		if target == expectedPiSkills {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("skillTargets() missing Pi skill target %q, got: %v", expectedPiSkills, targets)
	}
}

func TestPiSkillInstallationAndCleanup(t *testing.T) {
	piDir := filepath.Join(t.TempDir(), "pi-agent")
	t.Setenv("PI_CODING_AGENT_DIR", piDir)

	cfg := loadTestConfig(t)
	cfg.PiAgentTarget = piDir
	targetDir := t.TempDir()
	cfg.AgentsMD.Global.Target = filepath.Join(targetDir, "CLAUDE.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil

	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	piSkillsDir := filepath.Join(piDir, "skills")
	docupSkill := filepath.Join(piSkillsDir, "docup", "SKILL.md")
	if _, err := os.Stat(docupSkill); err != nil {
		t.Errorf("expected Pi skill %s to be installed: %v", docupSkill, err)
	}

	piDistillExt := filepath.Join(piDir, "extensions", "harnez-distill.ts")
	if _, err := os.Stat(piDistillExt); err != nil {
		t.Errorf("expected Pi extension %s to be installed: %v", piDistillExt, err)
	}

	if err := CleanAll(targetDir, cfg); err != nil {
		t.Fatalf("CleanAll failed: %v", err)
	}

	if _, err := os.Stat(docupSkill); !os.IsNotExist(err) {
		t.Errorf("expected Pi skill %s to be removed after CleanAll", docupSkill)
	}
}
