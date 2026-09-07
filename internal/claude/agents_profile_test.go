// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildAgentProfileTestConfig returns a minimal config exercising
// agents_md.global and agents_md.agents together, so the negative assertion
// (the agent section must NOT leak into the shared global/prime targets) is
// meaningful. AgentsMD.Local is applied by `init` (project scaffolding), not
// `ApplyAll` (global apply) — see init.go — so it is deliberately left unset
// here; local-section behavior is out of scope for this feature's tests.
func buildAgentProfileTestConfig(t *testing.T) (cfg *Config, globalTarget, primeAGENTSMD, codexTarget string) {
	t.Helper()
	dir := t.TempDir()

	globalTarget = filepath.Join(dir, "claude", "CLAUDE.md")
	primeDir := filepath.Join(dir, "prime")
	primeAGENTSMD = filepath.Join(primeDir, "AGENTS.md")
	codexDir := filepath.Join(dir, "codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatalf("MkdirAll codex dir: %v", err)
	}
	codexTarget = filepath.Join(codexDir, "AGENTS.md")

	cfg = &Config{
		PrimeAgentTarget: primeDir,
		AgentsMD: AgentsMD{
			Global: AgentsMDTarget{
				Target: globalTarget,
				Sections: []MDSection{
					{Name: "Shared Rule", Content: "shared content for every agent"},
				},
			},
			Agents: map[string]AgentsMDTarget{
				"codex": {
					Target: codexTarget,
					Sections: []MDSection{
						{Name: "Background Job Waiting", Content: "codex-only async wait guidance"},
					},
				},
			},
		},
	}
	return cfg, globalTarget, primeAGENTSMD, codexTarget
}

func TestApplyAll_AgentProfile_LandsInOwnTarget(t *testing.T) {
	cfg, _, _, codexTarget := buildAgentProfileTestConfig(t)
	target := t.TempDir()

	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	data, err := os.ReadFile(codexTarget)
	if err != nil {
		t.Fatalf("expected codex target to be written: %v", err)
	}
	if !strings.Contains(string(data), "codex-only async wait guidance") {
		t.Errorf("expected codex target to contain agent section content, got:\n%s", data)
	}
	if !strings.Contains(string(data), "Background Job Waiting") {
		t.Errorf("expected codex target to contain the section heading, got:\n%s", data)
	}
}

// TestApplyAll_AgentProfile_DoesNotLeakToSharedTargets is the ticket's required
// negative assertion (scope item 3): the codex-only section must not appear
// in the global (~/.claude/CLAUDE.md-equivalent) or Prime Agent
// (~/.prime/agent/AGENTS.md-equivalent) managed files.
func TestApplyAll_AgentProfile_DoesNotLeakToSharedTargets(t *testing.T) {
	cfg, globalTarget, primeAGENTSMD, _ := buildAgentProfileTestConfig(t)
	target := t.TempDir()

	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	globalData, err := os.ReadFile(globalTarget)
	if err != nil {
		t.Fatalf("expected global target to be written: %v", err)
	}
	if strings.Contains(string(globalData), "codex-only async wait guidance") {
		t.Errorf("agent-only content leaked into global target:\n%s", globalData)
	}
	if strings.Contains(string(globalData), "Background Job Waiting") {
		t.Errorf("agent-only section heading leaked into global target:\n%s", globalData)
	}

	primeData, err := os.ReadFile(primeAGENTSMD)
	if err != nil {
		t.Fatalf("expected prime agent target to be written: %v", err)
	}
	if strings.Contains(string(primeData), "codex-only async wait guidance") {
		t.Errorf("agent-only content leaked into prime agent target:\n%s", primeData)
	}
}

// TestApplyAll_AgentProfile_SkipsMissingAgentRoot verifies the soft-skip
// posture: an agent profile whose target's parent directory does not exist
// on disk must not be created (mirrors not creating ~/.codex for a user who
// doesn't run Codex).
func TestApplyAll_AgentProfile_SkipsMissingAgentRoot(t *testing.T) {
	dir := t.TempDir()
	missingCodexTarget := filepath.Join(dir, "does-not-exist", "AGENTS.md")

	cfg := &Config{
		AgentsMD: AgentsMD{
			Agents: map[string]AgentsMDTarget{
				"codex": {
					Target: missingCodexTarget,
					Sections: []MDSection{
						{Name: "Background Job Waiting", Content: "codex-only async wait guidance"},
					},
				},
			},
		},
	}

	target := t.TempDir()
	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	if _, err := os.Stat(missingCodexTarget); err == nil {
		t.Errorf("expected codex target NOT to be created when parent dir is absent")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected stat error: %v", err)
	}
}

func TestApplyAll_AgentProfile_Idempotent(t *testing.T) {
	cfg, _, _, _ := buildAgentProfileTestConfig(t)
	target := t.TempDir()

	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatalf("first ApplyAll failed: %v", err)
	}

	out, err := captureStdoutStatus(func() error {
		return ApplyAll(target, cfg, nil, false, false)
	})
	if err != nil {
		t.Fatalf("second ApplyAll failed: %v", err)
	}
	if strings.Contains(out, "wrote") {
		t.Errorf("expected second ApplyAll to be a no-op (idempotent), got:\n%s", out)
	}
}

func TestCleanAll_RemovesAgentProfileSection(t *testing.T) {
	cfg, _, _, codexTarget := buildAgentProfileTestConfig(t)
	target := t.TempDir()

	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}
	if _, err := os.Stat(codexTarget); err != nil {
		t.Fatalf("expected codex target to exist before clean: %v", err)
	}

	if err := CleanAll(target, cfg); err != nil {
		t.Fatalf("CleanAll failed: %v", err)
	}

	data, err := os.ReadFile(codexTarget)
	if err == nil {
		if strings.Contains(string(data), "codex-only async wait guidance") {
			t.Errorf("expected agent section content to be removed by clean, got:\n%s", data)
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error reading codex target after clean: %v", err)
	}
}

func TestDiffAll_AgentProfile_NoFalseDriftWhenApplied(t *testing.T) {
	cfg, _, _, _ := buildAgentProfileTestConfig(t)
	target := t.TempDir()

	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	changed, err := DiffAll(target, cfg)
	if err != nil {
		t.Fatalf("DiffAll failed: %v", err)
	}
	if changed {
		t.Errorf("expected DiffAll to report no drift right after apply")
	}
}

func TestRunStatus_ReportsAgentSectionCount(t *testing.T) {
	cfg, _, _, _ := buildAgentProfileTestConfig(t)
	target := t.TempDir()

	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	out, err := captureStdoutStatus(func() error {
		return RunStatus("(test)", cfg, target)
	})
	if err != nil {
		t.Fatalf("RunStatus failed: %v", err)
	}
	if !strings.Contains(out, "1 global, 0 local, 1 agent") {
		t.Errorf("expected agents_md census line to report 1 global, 0 local, 1 agent, got:\n%s", out)
	}
}
