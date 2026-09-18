package claude_test

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/jsonc"
)

// captureStdout executes f and returns whatever was written to stdout, alongside the returned error.
func captureStdout(f func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	errVal := f()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String(), errVal
}

func TestIntegrationWorkflow(t *testing.T) {
	// 1. Setup target sandbox directory
	targetDir := t.TempDir()
	settingsPath := filepath.Join(targetDir, "settings.json")

	// 2. Load the default embedded configuration
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	geminiSkillsDir := filepath.Join(t.TempDir(), "gemini-skills")
	codexSkillsDir := filepath.Join(t.TempDir(), "codex-skills")
	claudeSkillsDir := filepath.Join(t.TempDir(), "claude-skills")
	primeAgentDir := filepath.Join(t.TempDir(), "prime-agent")
	cfg.SkillsTarget = geminiSkillsDir
	cfg.CodexSkillsTarget = codexSkillsDir
	cfg.CodexHooksTarget = filepath.Join(t.TempDir(), "codex-config.toml")
	cfg.AgyHooksTarget = filepath.Join(t.TempDir(), "gemini", "config", "hooks.json")
	cfg.ClaudeSkillsTarget = claudeSkillsDir
	cfg.PrimeAgentTarget = primeAgentDir
	cfg.AgentsMD.Global.Target = filepath.Join(t.TempDir(), "CLAUDE.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil // issue 149: keep tests off the real ~/.codex path
	piExtensionPath := filepath.Join(t.TempDir(), "pi", "harnez-distill.ts")
	openCodePluginPath := filepath.Join(t.TempDir(), "opencode", "harnez-distill.ts")
	cfg.DistillAutopipe.PiExtensionTarget = piExtensionPath
	cfg.DistillAutopipe.OpenCodePluginTarget = openCodePluginPath

	// 3. First apply: should write files and show changes
	out, err := captureStdout(func() error {
		return claude.ApplyAll(targetDir, cfg, nil, false, false)
	})
	if err != nil {
		t.Fatalf("First ApplyAll failed: %v", err)
	}
	if !strings.Contains(out, "wrote") {
		t.Errorf("Expected 'wrote' status in output, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(geminiSkillsDir, "evergreen", "SKILL.md")); err != nil {
		t.Fatalf("Expected Gemini skill to be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(geminiSkillsDir, "sprint", "SKILL.md")); err != nil {
		t.Fatalf("Expected Gemini sprint skill to be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(codexSkillsDir, "evergreen", "SKILL.md")); err != nil {
		t.Fatalf("Expected Codex skill to be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(codexSkillsDir, "sprint", "SKILL.md")); err != nil {
		t.Fatalf("Expected Codex sprint skill to be written: %v", err)
	}
	claudeSkillPath := filepath.Join(claudeSkillsDir, "evergreen", "SKILL.md")
	claudeSkill, err := os.ReadFile(claudeSkillPath)
	if err != nil {
		t.Fatalf("Expected Claude Code skill to be written: %v", err)
	}
	if !strings.HasPrefix(string(claudeSkill), "---\nname: \"evergreen\"\ndescription:") {
		t.Errorf("Expected Agent Skills frontmatter in %s, got:\n%s", claudeSkillPath, claudeSkill)
	}
	if _, err := os.Stat(filepath.Join(claudeSkillsDir, "sprint", "SKILL.md")); err != nil {
		t.Fatalf("Expected Claude Code sprint skill to be written: %v", err)
	}
	primeSkillPath := filepath.Join(primeAgentDir, "skills", "evergreen", "SKILL.md")
	primeSkill, err := os.ReadFile(primeSkillPath)
	if err != nil {
		t.Fatalf("Expected Prime Agent skill to be written: %v", err)
	}
	if !strings.HasPrefix(string(primeSkill), "---\nname: \"evergreen\"\ndescription:") {
		t.Errorf("Expected Agent Skills frontmatter in %s, got:\n%s", primeSkillPath, primeSkill)
	}
	if _, err := os.Stat(filepath.Join(primeAgentDir, "skills", "sprint", "SKILL.md")); err != nil {
		t.Fatalf("Expected Prime Agent sprint skill to be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(primeAgentDir, "prompts", "standup.md")); err != nil {
		t.Fatalf("Expected Prime Agent prompt to be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(primeAgentDir, "skills", "review", "SKILL.md")); err != nil {
		t.Fatalf("Expected Prime Agent review skill to be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(primeAgentDir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("Expected Prime Agent AGENTS.md to NOT be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(primeAgentDir, "docs", "Go.md")); !os.IsNotExist(err) {
		t.Fatalf("Expected Prime Agent doc to NOT be written by default bare ApplyAll: %v", err)
	}
	if _, err := os.Stat(filepath.Join(primeAgentDir, "docs", "AgenticLoop.md")); !os.IsNotExist(err) {
		t.Fatalf("Expected Prime Agent AgenticLoop doc to NOT be written by default bare ApplyAll: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "commands", "standup.md")); err != nil {
		t.Fatalf("Expected Claude standup command to be written: %v", err)
	}
	if data, err := os.ReadFile(piExtensionPath); err != nil {
		t.Fatalf("Expected Pi distill adapter to be written: %v", err)
	} else if !strings.Contains(string(data), `pi.on("tool_call"`) {
		t.Fatalf("Expected Pi distill adapter content, got:\n%s", string(data))
	}
	if data, err := os.ReadFile(openCodePluginPath); err != nil {
		t.Fatalf("Expected OpenCode distill adapter to be written: %v", err)
	} else if !strings.Contains(string(data), `"tool.execute.before"`) {
		t.Fatalf("Expected OpenCode distill adapter content, got:\n%s", string(data))
	}

	// 4. Second apply: must be idempotent and report "No changes."
	out, err = captureStdout(func() error {
		return claude.ApplyAll(targetDir, cfg, nil, false, false)
	})
	if err != nil {
		t.Fatalf("Second ApplyAll failed: %v", err)
	}
	if !strings.Contains(out, "No changes.") {
		t.Errorf("Expected second ApplyAll to report no changes, got:\n%s", out)
	}

	// 5. Diff when aligned: must report "No changes." and changed == false
	var diffChanged bool
	out, err = captureStdout(func() error {
		var dErr error
		diffChanged, dErr = claude.DiffAll(targetDir, cfg)
		return dErr
	})
	if err != nil {
		t.Fatalf("DiffAll failed: %v", err)
	}
	if diffChanged {
		t.Errorf("Expected DiffAll changed to be false when aligned, got true")
	}
	if !strings.Contains(out, "No changes.") {
		t.Errorf("Expected DiffAll to report no changes when aligned, got:\n%s", out)
	}

	// 6. Simulate permission drift by dropping an allow entry from settings.json
	m := jsonc.Read(settingsPath)
	perms, ok := m["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("permissions block not found in generated settings.json")
	}
	allow := jsonc.ToStrings(perms["allow"])
	if len(allow) == 0 {
		t.Fatalf("permissions.allow list is empty in generated settings.json")
	}
	// Drop the first permission entry
	perms["allow"] = allow[1:]
	err = os.WriteFile(settingsPath, append(jsonc.MarshalPretty(m), '\n'), 0644)
	if err != nil {
		t.Fatalf("Failed to write drifted settings.json: %v", err)
	}

	// 7. Diff under drift: must output changes and changed == true
	out, err = captureStdout(func() error {
		var dErr error
		diffChanged, dErr = claude.DiffAll(targetDir, cfg)
		return dErr
	})
	if err != nil {
		t.Fatalf("DiffAll under drift failed: %v", err)
	}
	if !diffChanged {
		t.Errorf("Expected DiffAll changed to be true under drift, got false")
	}
	if !strings.Contains(out, "Bash(journalctl *)") {
		t.Errorf("Expected DiffAll to report differences under drift, got:\n%s", out)
	}

	// 8. Apply after drift: must restore the dropped permission
	out, err = captureStdout(func() error {
		return claude.ApplyAll(targetDir, cfg, nil, false, false)
	})
	if err != nil {
		t.Fatalf("ApplyAll to repair drift failed: %v", err)
	}
	if !strings.Contains(out, "permissions.allow: +1") {
		t.Errorf("Expected ApplyAll to show added permission entry, got:\n%s", out)
	}

	// 9. Run status checking
	out, err = captureStdout(func() error {
		return claude.RunStatus("(embedded)", cfg, targetDir)
	})
	if err != nil {
		t.Fatalf("RunStatus failed: %v", err)
	}
	if !strings.Contains(out, "settings.json [effortLevel]") || !strings.Contains(out, "ok") {
		t.Errorf("Expected RunStatus to list settings state as ok, got:\n%s", out)
	}
	if !strings.Contains(out, "catalog:       ok") {
		t.Errorf("Expected RunStatus to report copied-doc catalog health, got:\n%s", out)
	}
	if !strings.Contains(out, filepath.Join(geminiSkillsDir, "evergreen", "SKILL.md")) {
		t.Errorf("Expected RunStatus to list Gemini skill target, got:\n%s", out)
	}
	if !strings.Contains(out, filepath.Join(codexSkillsDir, "evergreen", "SKILL.md")) {
		t.Errorf("Expected RunStatus to list Codex skill target, got:\n%s", out)
	}
	if !strings.Contains(out, filepath.Join(claudeSkillsDir, "evergreen", "SKILL.md")) {
		t.Errorf("Expected RunStatus to list Claude Code skill target, got:\n%s", out)
	}
	if !strings.Contains(out, filepath.Join(primeAgentDir, "skills", "evergreen", "SKILL.md")) {
		t.Errorf("Expected RunStatus to list Prime Agent skill target, got:\n%s", out)
	}
	if !strings.Contains(out, filepath.Join(primeAgentDir, "prompts", "standup.md")) {
		t.Errorf("Expected RunStatus to list Prime Agent prompt target, got:\n%s", out)
	}
	if strings.Contains(out, filepath.Join(primeAgentDir, "AGENTS.md")) {
		t.Errorf("Expected RunStatus to NOT list Prime Agent rules target, got:\n%s", out)
	}
	if strings.Contains(out, filepath.Join(primeAgentDir, "docs", "Go.md")) {
		t.Errorf("Expected RunStatus to NOT list Prime Agent docs target by default, got:\n%s", out)
	}
	if !strings.Contains(out, piExtensionPath) {
		t.Errorf("Expected RunStatus to list Pi distill adapter, got:\n%s", out)
	}
	if !strings.Contains(out, openCodePluginPath) {
		t.Errorf("Expected RunStatus to list OpenCode distill adapter, got:\n%s", out)
	}

	// 10. Run project init (without CLI summary)
	projDir := t.TempDir()
	if err := exec.Command("git", "-C", projDir, "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	err = claude.RunInit(projDir, nil, nil, "", false, false, false, false)
	if err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}
	agentsPath := filepath.Join(projDir, "AGENTS.md")
	claudePath := filepath.Join(projDir, "CLAUDE.md")
	if _, err := os.Stat(agentsPath); err != nil {
		t.Errorf("Expected AGENTS.md to exist: %v", err)
	}
	if _, err := os.Stat(claudePath); err != nil {
		t.Errorf("Expected CLAUDE.md symlink to exist: %v", err)
	}
	agentsContent, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("Failed to read generated AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agentsContent), "<!-- harnez:begin Project Summary -->") {
		t.Errorf("Expected AGENTS.md to contain harnez marker, got:\n%s", string(agentsContent))
	}

	// 11. Clean command: must delete all managed configurations
	out, err = captureStdout(func() error {
		return claude.CleanAll(targetDir, cfg)
	})
	if err != nil {
		t.Fatalf("CleanAll failed: %v", err)
	}
	if !strings.Contains(out, "removed") && !strings.Contains(out, "cleaned") {
		t.Errorf("Expected CleanAll to report removed/cleaned files, got:\n%s", out)
	}
	if _, err := os.Stat(settingsPath); err == nil || !os.IsNotExist(err) {
		t.Errorf("Expected settings.json to be deleted after CleanAll, but it exists")
	}
	if _, err := os.Stat(claudeSkillPath); err == nil || !os.IsNotExist(err) {
		t.Errorf("Expected Claude Code skill %s to be removed by CleanAll, but it exists", claudeSkillPath)
	}
}

func TestDiffAll_ExecError(t *testing.T) {
	targetDir := t.TempDir()
	settingsPath := filepath.Join(targetDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	// Set PATH to empty temp dir so 'diff' binary is not found
	t.Setenv("PATH", t.TempDir())

	_, err = claude.DiffAll(targetDir, cfg)
	if err == nil {
		t.Fatalf("Expected DiffAll to return error when diff binary is missing, got nil")
	}
}

func TestBatchProjectInitialization_HeterogeneousWorkspace(t *testing.T) {
	workspaceDir := t.TempDir()

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	// Create 4 distinct mock projects modeling actual sibling repositories:
	// 1. go-service (Go + Makefile)
	goProj := filepath.Join(workspaceDir, "go-service")
	_ = os.MkdirAll(goProj, 0o755)
	_ = os.WriteFile(filepath.Join(goProj, "go.mod"), []byte("module example.com/gosvc\n"), 0o644)
	_ = os.WriteFile(filepath.Join(goProj, "Makefile"), []byte("all:\n\t@echo ok\n"), 0o644)

	// 2. zig-cli (pure Zig, no Makefile, like zterm)
	zigProj := filepath.Join(workspaceDir, "zig-cli")
	_ = os.MkdirAll(zigProj, 0o755)
	_ = os.WriteFile(filepath.Join(zigProj, "build.zig"), []byte("const std = @import(\"std\");\n"), 0o644)

	// 3. custom-notes (unmanaged repo with custom AGENTS.md, like homeserver)
	customProj := filepath.Join(workspaceDir, "custom-notes")
	_ = os.MkdirAll(customProj, 0o755)
	customProse := "# Custom Admin Guidelines\nStrictly manual setup.\n"
	_ = os.WriteFile(filepath.Join(customProj, "AGENTS.md"), []byte(customProse), 0o644)

	// 4. bare-dir (non-project folder with just random notes, like mindstore/rocmctl)
	bareDir := filepath.Join(workspaceDir, "bare-dir")
	_ = os.MkdirAll(bareDir, 0o755)
	_ = os.WriteFile(filepath.Join(bareDir, "README.txt"), []byte("bare notes\n"), 0o644)

	projects := []string{goProj, zigProj, customProj}

	// 1. Run Init across all projects
	for _, p := range projects {
		if err := claude.RunInit(p, cfg, nil, "", true, false, false, false); err != nil {
			t.Fatalf("RunInit on %s failed: %v", filepath.Base(p), err)
		}
	}

	// Assertions for go-service:
	if _, err := os.Stat(filepath.Join(goProj, "docs", "Go.md")); err != nil {
		t.Errorf("Expected go-service to have docs/Go.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(goProj, "docs", "AgenticLoop.md")); err != nil {
		t.Errorf("Expected go-service to have docs/AgenticLoop.md: %v", err)
	}
	goAgents, _ := os.ReadFile(filepath.Join(goProj, "AGENTS.md"))
	if !strings.Contains(string(goAgents), "Go/Golang") || !strings.Contains(string(goAgents), "Agentic Loop Practices") {
		t.Errorf("go-service AGENTS.md missing expected conventions:\n%s", string(goAgents))
	}

	// Assertions for zig-cli:
	if _, err := os.Stat(filepath.Join(zigProj, "docs", "Zig.md")); err != nil {
		t.Errorf("Expected zig-cli to have docs/Zig.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(zigProj, "Makefile")); !os.IsNotExist(err) {
		t.Errorf("zig-cli should NOT have Makefile created")
	}

	// Assertions for custom-notes:
	customAgents, _ := os.ReadFile(filepath.Join(customProj, "AGENTS.md"))
	if !strings.Contains(string(customAgents), "Custom Admin Guidelines") {
		t.Errorf("custom-notes AGENTS.md corrupted custom prose:\n%s", string(customAgents))
	}

	// Assertions for bare-dir (was untouched):
	if _, err := os.Stat(filepath.Join(bareDir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Errorf("bare-dir should remain untouched")
	}

	// Idempotency: second batch run should succeed cleanly
	for _, p := range projects {
		if err := claude.RunInit(p, cfg, nil, "", true, false, false, false); err != nil {
			t.Fatalf("Second RunInit on %s failed: %v", filepath.Base(p), err)
		}
	}
}
