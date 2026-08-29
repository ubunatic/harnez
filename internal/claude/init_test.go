// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude_test

import (
	"os"
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

