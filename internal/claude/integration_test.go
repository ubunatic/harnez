package claude_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/claudeconfig/internal/claude"
	"ubunatic.com/claudeconfig/internal/jsonc"
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

	// 3. First apply: should write files and show changes
	out, err := captureStdout(func() error {
		return claude.ApplyAll(targetDir, "", cfg, nil, false)
	})
	if err != nil {
		t.Fatalf("First ApplyAll failed: %v", err)
	}
	if !strings.Contains(out, "wrote") {
		t.Errorf("Expected 'wrote' status in output, got:\n%s", out)
	}

	// 4. Second apply: must be idempotent and report "No changes."
	out, err = captureStdout(func() error {
		return claude.ApplyAll(targetDir, "", cfg, nil, false)
	})
	if err != nil {
		t.Fatalf("Second ApplyAll failed: %v", err)
	}
	if !strings.Contains(out, "No changes.") {
		t.Errorf("Expected second ApplyAll to report no changes, got:\n%s", out)
	}

	// 5. Diff when aligned: must report "No changes."
	out, err = captureStdout(func() error {
		return claude.DiffAll(targetDir, cfg)
	})
	if err != nil {
		t.Fatalf("DiffAll failed: %v", err)
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

	// 7. Diff under drift: must output changes
	out, err = captureStdout(func() error {
		return claude.DiffAll(targetDir, cfg)
	})
	if err != nil {
		t.Fatalf("DiffAll under drift failed: %v", err)
	}
	if !strings.Contains(out, "Bash(journalctl *)") {
		t.Errorf("Expected DiffAll to report differences under drift, got:\n%s", out)
	}

	// 8. Apply after drift: must restore the dropped permission
	out, err = captureStdout(func() error {
		return claude.ApplyAll(targetDir, "", cfg, nil, false)
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
	if !strings.Contains(out, "settings.json [model]") || !strings.Contains(out, "ok") {
		t.Errorf("Expected RunStatus to list settings state as ok, got:\n%s", out)
	}

	// 10. Run project init (without CLI summary)
	projDir := t.TempDir()
	err = claude.RunInit(projDir, false, false, false)
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
}
