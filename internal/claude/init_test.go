// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/claude"
)

func TestRunInit_LegacyMarkerMigrationAndProsePreservation(t *testing.T) {
	dir := t.TempDir()

	// 1. Setup mock Go repo with legacy claudeconfig markers and custom preamble/postamble
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	initialAgentsMD := `# Custom Project Working Agreement

- Preamble: This is a handwritten team agreement.
- Do not remove this preamble under any circumstance.

<!-- claudeconfig:begin Language Conventions -->
- Go/Golang @docs/Go.md,
  Old Go conventions
<!-- claudeconfig:end Language Conventions -->

## Custom Spec & Pixel Art Guidelines
- Custom Section 1: Detailed specification rules.
- Custom Section 2: Keep exact color palette.
`

	agentsPath := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(initialAgentsMD), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	// 2. Run RunInit
	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	// 3. Verify AGENTS.md content
	updated, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(updated)

	// Verify legacy markers migrated to harnez markers
	if strings.Contains(content, "<!-- claudeconfig:") {
		t.Errorf("Expected all claudeconfig markers to be migrated, got:\n%s", content)
	}
	if !strings.Contains(content, "<!-- harnez:begin Language Conventions -->") {
		t.Errorf("Expected harnez:begin Language Conventions marker, got:\n%s", content)
	}
	if !strings.Contains(content, "<!-- harnez:end Language Conventions -->") {
		t.Errorf("Expected harnez:end Language Conventions marker, got:\n%s", content)
	}

	// Verify custom preamble is preserved
	if !strings.Contains(content, "# Custom Project Working Agreement") ||
		!strings.Contains(content, "This is a handwritten team agreement.") {
		t.Errorf("Custom preamble was lost:\n%s", content)
	}

	// Verify custom downstream sections are preserved
	if !strings.Contains(content, "## Custom Spec & Pixel Art Guidelines") ||
		!strings.Contains(content, "Keep exact color palette.") {
		t.Errorf("Custom downstream section was lost:\n%s", content)
	}

	// Verify auto-detected docs updated in Language Conventions block
	if !strings.Contains(content, "Agentic Loop Practices @docs/AgenticLoop.md") {
		t.Errorf("Expected AgenticLoop.md convention in updated block, got:\n%s", content)
	}

	// Verify CLAUDE.md symlink points to AGENTS.md
	claudePath := filepath.Join(dir, "CLAUDE.md")
	dest, err := os.Readlink(claudePath)
	if err != nil {
		t.Fatalf("Expected CLAUDE.md symlink: %v", err)
	}
	if dest != "AGENTS.md" {
		t.Errorf("Expected symlink target 'AGENTS.md', got %q", dest)
	}
}

func TestRunInit_NonMakefileProjectSafety(t *testing.T) {
	dir := t.TempDir()

	// Setup pure Zig project with no Makefile (like zterm)
	if err := os.WriteFile(filepath.Join(dir, "build.zig"), []byte("const std = @import(\"std\");\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "main.zig"), []byte("pub func main() void {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	// Verify AGENTS.md and docs/Zig.md exist
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Errorf("Expected AGENTS.md to be created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "Zig.md")); err != nil {
		t.Errorf("Expected docs/Zig.md to be installed: %v", err)
	}

	// Crucial assertion: Makefile should NOT be created for non-Make project
	if _, err := os.Stat(filepath.Join(dir, "Makefile")); !os.IsNotExist(err) {
		t.Errorf("Makefile was unexpectedly created in a pure Zig project")
	}
}

func TestRunInit_UnmanagedCustomAgentsMD(t *testing.T) {
	dir := t.TempDir()

	customContent := `# Homeserver Custom Instructions

This project does not use automated markers.
It is completely managed manually by the administrator.
`
	agentsPath := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(customContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	// Content must retain custom text
	readBack, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readBack), "Homeserver Custom Instructions") ||
		!strings.Contains(string(readBack), "completely managed manually") {
		t.Errorf("Unmanaged custom AGENTS.md content was corrupted:\n%s", string(readBack))
	}
}

func TestRunInit_CustomTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	agentsPath := filepath.Join(dir, "AGENTS.md")
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "<!-- harnez:begin Project Summary -->") ||
		!strings.Contains(content, "## Development Scripts") {
		t.Errorf("AGENTS.md does not contain expected template structure:\n%s", content)
	}
}

func TestRunInit_IgnoresIssuesReadmeLockWithoutChangingGitignore(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitignore := "dist/\n# project-specific rules\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		t.Fatal(err)
	}
	excludePath := filepath.Join(dir, ".git", "info", "exclude")
	if err := os.WriteFile(excludePath, []byte("# existing local rules\n*.local-cache\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho user-hook\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := claude.RunInit(dir, nil, nil, "", false, false, false, false); err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "issues", "README.md.lock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "check-ignore", "issues/README.md.lock").CombinedOutput(); err != nil {
		t.Fatalf("issues lock is not ignored after init: %v (%s)", err, out)
	}
	gotGitignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotGitignore) != gitignore {
		t.Errorf("project .gitignore changed:\ngot:\n%s\nwant:\n%s", gotGitignore, gitignore)
	}

	if err := claude.RunInit(dir, nil, nil, "", false, false, false, false); err != nil {
		t.Fatalf("RunInit second run failed: %v", err)
	}
	exclude, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(exclude), "/issues/README.md.lock") != 1 {
		t.Errorf("lock ignore should appear exactly once after two init runs:\n%s", exclude)
	}
	if !strings.Contains(string(exclude), "*.local-cache") {
		t.Errorf("existing local exclude content was lost:\n%s", exclude)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitattributes")); !os.IsNotExist(err) {
		t.Errorf("plain init unexpectedly created .gitattributes: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "config", "--local", "--get", "merge.harnez-issues-index.driver").Run(); err == nil {
		t.Error("plain init unexpectedly configured merge driver")
	}
	hook, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(hook) != "#!/bin/sh\necho user-hook\n" {
		t.Errorf("plain init unexpectedly changed pre-commit hook:\n%s", hook)
	}
}

func TestRunInit_IssuesGitIsExplicitAndRemovable(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("*.bin binary\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho user-hook\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "config", "--local", "custom.keep", "yes").Run(); err != nil {
		t.Fatal(err)
	}

	if err := claude.RunInitWithIssuesGit(dir, nil, nil, "", false, false, false, false, nil); err != nil {
		t.Fatalf("plain RunInit failed: %v", err)
	}
	attributes, _ := os.ReadFile(filepath.Join(dir, ".gitattributes"))
	if string(attributes) != "*.bin binary\n" {
		t.Fatalf("plain init changed attributes: %q", attributes)
	}
	if err := exec.Command("git", "-C", dir, "config", "--local", "--get", "merge.harnez-issues-index.driver").Run(); err == nil {
		t.Fatal("plain init unexpectedly configured issue merge driver")
	}
	hook, _ := os.ReadFile(hookPath)
	if string(hook) != "#!/bin/sh\necho user-hook\n" {
		t.Fatalf("plain init changed hook: %q", hook)
	}

	enabled := true
	for range 2 {
		if err := claude.RunInitWithIssuesGit(dir, nil, nil, "", false, false, false, false, &enabled); err != nil {
			t.Fatalf("enable issues Git integration: %v", err)
		}
	}
	attributes, _ = os.ReadFile(filepath.Join(dir, ".gitattributes"))
	if strings.Count(string(attributes), "issues/README.md merge=harnez-issues-index") != 1 {
		t.Fatalf("enable did not install one managed attribute: %q", attributes)
	}
	driver, err := exec.Command("git", "-C", dir, "config", "--local", "--get", "merge.harnez-issues-index.driver").Output()
	if err != nil || strings.TrimSpace(string(driver)) != "harnez issues merge-driver %O %A %B" {
		t.Fatalf("enable did not configure merge driver: %v (%q)", err, driver)
	}
	hook, _ = os.ReadFile(hookPath)
	if strings.Count(string(hook), "# harnez:begin issues-index-lint") != 1 || !strings.Contains(string(hook), "echo user-hook") {
		t.Fatalf("enable did not install one managed hook block: %q", hook)
	}
	disabled := false
	if err := claude.RunInitWithIssuesGit(dir, nil, nil, "", false, false, false, false, &disabled); err != nil {
		t.Fatalf("disable issues Git integration: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "issues")); err != nil {
		t.Fatal(err)
	}
	if err := claude.RunInitWithIssuesGit(dir, nil, nil, "", false, false, false, false, &disabled); err != nil {
		t.Fatalf("repeat disable without issues directory: %v", err)
	}
	attributes, _ = os.ReadFile(filepath.Join(dir, ".gitattributes"))
	if string(attributes) != "*.bin binary\n" {
		t.Errorf("disable did not preserve unrelated attributes: %q", attributes)
	}
	hook, _ = os.ReadFile(hookPath)
	if string(hook) != "#!/bin/sh\necho user-hook\n" {
		t.Errorf("disable did not preserve unrelated hook: %q", hook)
	}
	if err := exec.Command("git", "-C", dir, "config", "--local", "--get", "merge.harnez-issues-index.driver").Run(); err == nil {
		t.Error("disable left issue merge driver configured")
	}
	keep, err := exec.Command("git", "-C", dir, "config", "--local", "--get", "custom.keep").Output()
	if err != nil || strings.TrimSpace(string(keep)) != "yes" {
		t.Errorf("disable did not preserve unrelated config: %v (%q)", err, keep)
	}
}

func TestRunInitAll_InitializesOnlyEligibleChildren(t *testing.T) {
	workspace := t.TempDir()

	eligible := filepath.Join(workspace, "repo-a")
	other := filepath.Join(workspace, "repo-b")
	bareDir := filepath.Join(workspace, "not-a-project")
	for _, dir := range []string{eligible, other, bareDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(eligible, "AGENTS.md"), []byte("# repo-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "CLAUDE.md"), []byte("# repo-b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// bareDir has neither AGENTS.md nor CLAUDE.md and must be skipped.

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	if err := claude.RunInitAll(workspace, cfg, nil, "", false, false, false); err != nil {
		t.Fatalf("RunInitAll failed: %v", err)
	}

	// repo-a already had AGENTS.md; RunInit should reconcile it, not clobber
	// its identity, and it must gain a CLAUDE.md symlink from the batch run.
	if _, err := os.Lstat(filepath.Join(eligible, "CLAUDE.md")); err != nil {
		t.Errorf("expected repo-a to gain a CLAUDE.md symlink from batch init: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(other, "AGENTS.md")); err != nil {
		t.Errorf("expected repo-b (CLAUDE.md-only) to gain a migrated AGENTS.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bareDir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Errorf("expected bare directory without AGENTS.md/CLAUDE.md to be skipped, got err=%v", err)
	}
}

func TestRunInitAll_RefusesHomeDirectory(t *testing.T) {
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot resolve home directory: %v", err)
	}
	err = claude.RunInitAll(home, cfg, nil, "", false, false, false)
	if err == nil {
		t.Fatal("expected RunInitAll to refuse the home directory, got nil error")
	}
	if !strings.Contains(err.Error(), "home directory") {
		t.Errorf("expected home-directory safety-guard error, got: %v", err)
	}
}

func TestRunInitAll_NoEligibleChildrenIsNotAnError(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "just-a-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	if err := claude.RunInitAll(workspace, cfg, nil, "", false, false, false); err != nil {
		t.Fatalf("expected no error scanning a workspace with no eligible children, got: %v", err)
	}
}

func TestCheckProjectDrift(t *testing.T) {
	tempRoot := t.TempDir()
	projDir := filepath.Join(tempRoot, "psync")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. Matching case
	if err := os.WriteFile(filepath.Join(projDir, "go.mod"), []byte("module ubunatic.com/psync\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if warnings := claude.CheckProjectDrift(projDir); len(warnings) != 0 {
		t.Errorf("expected 0 warnings for clean matching project, got %d: %v", len(warnings), warnings)
	}

	// 2. Mismatched go.mod module vs directory name
	if err := os.WriteFile(filepath.Join(projDir, "go.mod"), []byte("module ubunatic.com/uman\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	warnings := claude.CheckProjectDrift(projDir)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for mismatched module name, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "go.mod module \"uman\" does not match directory name \"psync\"") {
		t.Errorf("unexpected warning message: %s", warnings[0])
	}

	// 3. Initialize git with mismatched origin URL
	if err := exec.Command("git", "-C", projDir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", projDir, "remote", "add", "origin", "ssh://git@codeberg.org/ubunatic/other.git").Run(); err != nil {
		t.Fatal(err)
	}
	warnings = claude.CheckProjectDrift(projDir)
	if len(warnings) < 2 {
		t.Errorf("expected at least 2 warnings with mismatched git origin and go.mod, got %d: %v", len(warnings), warnings)
	}
}

// TestRunInit_AppliesManagedConventionsSection verifies issue 311's core
// contract: agents_md.local.sections are written into an existing AGENTS.md
// alongside its custom content, without duplicating on a second run.
func TestRunInit_AppliesManagedConventionsSection(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/managedconv\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	initialAgentsMD := `# Custom Project Working Agreement

- Preamble: hand-authored, must survive init.

## Custom Downstream Section
- Project-specific rule that must survive init.
`
	agentsPath := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(initialAgentsMD), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatalf("first RunInit failed: %v", err)
	}
	first, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(first)

	if n := strings.Count(content, "<!-- harnez:begin Harnez Managed Conventions -->"); n != 1 {
		t.Errorf("expected exactly one Harnez Managed Conventions section, got %d in:\n%s", n, content)
	}
	if !strings.Contains(content, "### Editing Discipline") {
		t.Errorf("expected Editing Discipline inside the managed section, got:\n%s", content)
	}
	if !strings.Contains(content, "# Custom Project Working Agreement") ||
		!strings.Contains(content, "## Custom Downstream Section") {
		t.Errorf("expected hand-authored custom content to survive, got:\n%s", content)
	}

	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatalf("second RunInit failed: %v", err)
	}
	second, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != content {
		t.Errorf("expected RunInit to be idempotent, got a diff between runs:\nfirst:\n%s\nsecond:\n%s", content, second)
	}
}

func TestRunInit_BackfillsLocalOverlaysSection(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/localoverlays\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	initialAgentsMD := `# Legacy Project Working Agreement

- Preamble: hand-authored, must survive init.

## Custom Downstream Section
- Project-specific rule that must survive init.
`
	agentsPath := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(initialAgentsMD), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatalf("first RunInit failed: %v", err)
	}
	first, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(first)
	wantBlock := "<!-- harnez:begin Local Overlays -->\n- Local ephemeral overrides: @AGENTS.local.md\n<!-- harnez:end Local Overlays -->"
	if strings.Count(content, "<!-- harnez:begin Local Overlays -->") != 1 {
		t.Errorf("expected exactly one Local Overlays section, got:\n%s", content)
	}
	if !strings.Contains(content, wantBlock) {
		t.Errorf("expected Local Overlays block, got:\n%s", content)
	}
	if !strings.Contains(content, "# Legacy Project Working Agreement") || !strings.Contains(content, "Project-specific rule that must survive init.") {
		t.Errorf("expected existing content to survive, got:\n%s", content)
	}

	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatalf("second RunInit failed: %v", err)
	}
	second, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != content {
		t.Errorf("expected RunInit to be idempotent, got a diff between runs:\nfirst:\n%s\nsecond:\n%s", content, second)
	}
}

// TestRunInit_PreservesOptInDocOnPlainReinit guards against a regression where
// a plain re-init (no --docs flag) silently dropped a previously opted-in
// optional (default: false) doc, because doc selection was recomputed purely
// from defaults/auto-detection with no awareness of the existing AGENTS.md.
func TestRunInit_PreservesOptInDocOnPlainReinit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/optindoc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	if err := claude.RunInit(dir, cfg, []string{"prototyping-features"}, "", true, false, false, false); err != nil {
		t.Fatalf("first RunInit (explicit opt-in) failed: %v", err)
	}
	agentsPath := filepath.Join(dir, "AGENTS.md")
	first, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), "@docs/PrototypingFeatures.md") {
		t.Fatalf("expected opt-in doc ref present after explicit --docs run, got:\n%s", first)
	}

	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatalf("second RunInit (plain re-init, no --docs) failed: %v", err)
	}
	second, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(second), "@docs/PrototypingFeatures.md") {
		t.Errorf("plain re-init dropped the previously opted-in optional doc, got:\n%s", second)
	}
}

func TestRunInit_RefusesHomeDirectoryWithoutForce(t *testing.T) {
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot resolve home directory: %v", err)
	}
	err = claude.RunInit(home, cfg, nil, "", false, false, false, false)
	if err == nil {
		t.Fatal("expected RunInit to refuse home directory without force, got nil error")
	}
	if !strings.Contains(err.Error(), "home directory") {
		t.Errorf("expected home directory safety error, got: %v", err)
	}
}

func TestRunInit_RefusesNonCodingDirectoryWithoutForce(t *testing.T) {
	dir := t.TempDir()
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	err = claude.RunInit(dir, cfg, nil, "", false, false, false, false)
	if err == nil {
		t.Fatal("expected RunInit to refuse empty non-coding directory without force, got nil error")
	}
	if !strings.Contains(err.Error(), "not a Git repository and contains no recognized project or source files") {
		t.Errorf("expected non-coding directory safety error, got: %v", err)
	}
}

func TestRunInit_AllowsNonCodingDirectoryWithForce(t *testing.T) {
	dir := t.TempDir()
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	err = claude.RunInitWithForce(dir, cfg, nil, "", true, false, false, false, nil, false, true)
	if err != nil {
		t.Fatalf("expected RunInitWithForce to succeed with force=true, got: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Errorf("expected AGENTS.md to be created: %v", err)
	}
}

func TestRunInit_AllowsCodingRepositories(t *testing.T) {
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	cases := []struct {
		name  string
		setup func(dir string)
	}{
		{
			name: "git repository",
			setup: func(dir string) {
				_ = exec.Command("git", "-C", dir, "init", "-q").Run()
			},
		},
		{
			name: "cargo toml",
			setup: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"demo\"\n"), 0o644)
			},
		},
		{
			name: "package json",
			setup: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}\n"), 0o644)
			},
		},
		{
			name: "makefile",
			setup: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "Makefile"), []byte("all:\n"), 0o644)
			},
		},
		{
			name: "source file in src",
			setup: func(dir string) {
				_ = os.MkdirAll(filepath.Join(dir, "src"), 0o755)
				_ = os.WriteFile(filepath.Join(dir, "src", "main.py"), []byte("print('hello')\n"), 0o644)
			},
		},
		{
			name: "issues directory",
			setup: func(dir string) {
				_ = os.MkdirAll(filepath.Join(dir, "issues"), 0o755)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.setup(dir)
			if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
				t.Fatalf("expected RunInit to succeed for %s, got: %v", tc.name, err)
			}
			if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
				t.Errorf("expected AGENTS.md to be created: %v", err)
			}
		})
	}
}
