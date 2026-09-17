package claude

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/jsonc"
)

// captureStdoutTelemetry mirrors claude_test's captureStdout helper
// (internal/claude/integration_test.go) but lives in package claude
// (whitebox) so this file can call ApplyAll/RunStatus/CleanAll directly
// alongside the config-internal jsonc.Read assertions above.
func captureStdoutTelemetry(f func() error) (string, error) {
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

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// TestTelemetryHookConfigEntry verifies issue 119's decision: the
// telemetry hook (harnez exec hook, see issue 118) is a declarative
// config.yaml entry consumed by the same hooks-merge apply.go already
// uses for distill's hook (issue 069), not a new code path.
func TestTelemetryHookConfigEntry(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	var found *Hook
	for i := range cfg.Hooks {
		h := &cfg.Hooks[i]
		if h.Event == "PreToolUse" && h.Matcher == "Bash" && h.Command == "harnez exec hook" {
			found = h
			break
		}
	}
	if found == nil {
		t.Fatalf("expected a PreToolUse/Bash hook with command %q in embedded config.yaml, hooks: %+v",
			"harnez exec hook", cfg.Hooks)
	}
}

// TestApplyInstallsTelemetryHook exercises the same install → idempotent
// re-apply → clean lifecycle TestIntegrationWorkflow covers for the rest
// of apply's managed keys, focused on the new telemetry hook entry.
func TestApplyInstallsTelemetryHook(t *testing.T) {
	targetDir := t.TempDir()
	settingsPath := filepath.Join(targetDir, "settings.json")

	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	// Isolate every other target this apply run would touch so this test
	// only exercises settings.json.
	cfg.SkillsTarget = filepath.Join(t.TempDir(), "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(t.TempDir(), "codex-skills")
	cfg.CodexHooksTarget = filepath.Join(t.TempDir(), "codex-config.toml")
	cfg.AgyHooksTarget = filepath.Join(t.TempDir(), "gemini", "config", "hooks.json")
	cfg.ClaudeSkillsTarget = filepath.Join(t.TempDir(), "claude-skills")
	cfg.PrimeAgentTarget = filepath.Join(t.TempDir(), "prime-agent")
	cfg.AgentsMD.Global.Target = filepath.Join(t.TempDir(), "CLAUDE.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil // issue 149: keep tests off the real ~/.codex path
	cfg.DistillAutopipe.PiExtensionTarget = filepath.Join(t.TempDir(), "pi", "harnez-distill.ts")
	cfg.DistillAutopipe.OpenCodePluginTarget = filepath.Join(t.TempDir(), "opencode", "harnez-distill.ts")

	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("first ApplyAll failed: %v", err)
	}

	if !settingsHasTelemetryHook(t, settingsPath) {
		t.Fatalf("expected settings.json to contain the telemetry hook (harnez exec hook) after apply")
	}

	// Idempotency: a second apply must not duplicate the entry.
	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("second ApplyAll failed: %v", err)
	}
	countBashPreToolUseHooks := func() int {
		m := jsonc.Read(settingsPath)
		hooks, _ := m["hooks"].(map[string]any)
		pre, _ := hooks["PreToolUse"].([]any)
		n := 0
		for _, entry := range pre {
			em, ok := entry.(map[string]any)
			if !ok || em["matcher"] != "Bash" {
				continue
			}
			hs, _ := em["hooks"].([]any)
			n += len(hs)
		}
		return n
	}
	firstCount := countBashPreToolUseHooks()
	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("third ApplyAll failed: %v", err)
	}
	if got := countBashPreToolUseHooks(); got != firstCount {
		t.Fatalf("expected repeated apply to be idempotent, PreToolUse/Bash hook count changed %d -> %d", firstCount, got)
	}

	// status must report the hook via the same generic "hooks:" count
	// counter it already uses for distill's hook (issue 119 AC).
	out, err := captureStdoutTelemetry(func() error {
		return RunStatus("(embedded)", cfg, targetDir)
	})
	if err != nil {
		t.Fatalf("RunStatus failed: %v", err)
	}
	if !containsAll(out, "hooks:", "settings.json [hooks]", "ok") {
		t.Errorf("expected RunStatus to report hooks count and settings state, got:\n%s", out)
	}

	// clean must remove the telemetry hook the same way it removes every
	// other managed "hooks" entry (data-driven off managedSettingsKeys).
	if err := CleanAll(targetDir, cfg); err != nil {
		t.Fatalf("CleanAll failed: %v", err)
	}
	if _, err := os.Stat(settingsPath); err == nil {
		t.Fatalf("expected settings.json to be removed by CleanAll (it only held managed keys)")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error stating settings.json after clean: %v", err)
	}
}

func settingsHasTelemetryHook(t *testing.T, settingsPath string) bool {
	t.Helper()
	m := jsonc.Read(settingsPath)
	hooks, ok := m["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("settings.json has no hooks key: %+v", m)
	}
	pre, ok := hooks["PreToolUse"].([]any)
	if !ok {
		t.Fatalf("settings.json hooks has no PreToolUse entries: %+v", hooks)
	}
	for _, entry := range pre {
		em, ok := entry.(map[string]any)
		if !ok || em["matcher"] != "Bash" {
			continue
		}
		hs, _ := em["hooks"].([]any)
		for _, h := range hs {
			hm, ok := h.(map[string]any)
			if ok && hm["command"] == "harnez exec hook" {
				return true
			}
		}
	}
	return false
}

// TestReadHookConfigEntry verifies issue 405's decision: the file-read interception
// hooks (harnez hook read) for View and ReadMultipleFiles are declarative config.yaml
// entries installed into settings.json under PreToolUse.
func TestReadHookConfigEntry(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	matchers := map[string]bool{"View": false, "ReadMultipleFiles": false}
	for i := range cfg.Hooks {
		h := &cfg.Hooks[i]
		if h.Event == "PreToolUse" && h.Command == "harnez hook read" {
			if _, ok := matchers[h.Matcher]; ok {
				matchers[h.Matcher] = true
			}
		}
	}
	for m, found := range matchers {
		if !found {
			t.Errorf("expected PreToolUse hook for matcher %q with command %q, hooks: %+v",
				m, "harnez hook read", cfg.Hooks)
		}
	}
}

func TestApplyInstallsReadHooks(t *testing.T) {
	targetDir := t.TempDir()
	settingsPath := filepath.Join(targetDir, "settings.json")

	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	cfg.SkillsTarget = filepath.Join(t.TempDir(), "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(t.TempDir(), "codex-skills")
	cfg.CodexHooksTarget = filepath.Join(t.TempDir(), "codex-config.toml")
	cfg.AgyHooksTarget = filepath.Join(t.TempDir(), "gemini", "config", "hooks.json")
	cfg.ClaudeSkillsTarget = filepath.Join(t.TempDir(), "claude-skills")
	cfg.PrimeAgentTarget = filepath.Join(t.TempDir(), "prime-agent")
	cfg.AgentsMD.Global.Target = filepath.Join(t.TempDir(), "CLAUDE.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil
	cfg.DistillAutopipe.PiExtensionTarget = filepath.Join(t.TempDir(), "pi", "harnez-distill.ts")
	cfg.DistillAutopipe.OpenCodePluginTarget = filepath.Join(t.TempDir(), "opencode", "harnez-distill.ts")

	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("first ApplyAll failed: %v", err)
	}

	if !settingsHasHook(t, settingsPath, "PreToolUse", "View", "harnez hook read") {
		t.Errorf("expected settings.json to contain PreToolUse/View hook")
	}
	if !settingsHasHook(t, settingsPath, "PreToolUse", "ReadMultipleFiles", "harnez hook read") {
		t.Errorf("expected settings.json to contain PreToolUse/ReadMultipleFiles hook")
	}

	// Idempotency: second apply must preserve hooks without duplication
	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("second ApplyAll failed: %v", err)
	}
	if !settingsHasHook(t, settingsPath, "PreToolUse", "View", "harnez hook read") {
		t.Errorf("expected settings.json to retain PreToolUse/View hook after second apply")
	}
	if !settingsHasHook(t, settingsPath, "PreToolUse", "ReadMultipleFiles", "harnez hook read") {
		t.Errorf("expected settings.json to retain PreToolUse/ReadMultipleFiles hook after second apply")
	}
}

func settingsHasHook(t *testing.T, settingsPath, event, matcher, command string) bool {
	t.Helper()
	m := jsonc.Read(settingsPath)
	hooks, ok := m["hooks"].(map[string]any)
	if !ok {
		return false
	}
	entries, ok := hooks[event].([]any)
	if !ok {
		return false
	}
	for _, entry := range entries {
		em, ok := entry.(map[string]any)
		if !ok || em["matcher"] != matcher {
			continue
		}
		hs, _ := em["hooks"].([]any)
		for _, h := range hs {
			hm, ok := h.(map[string]any)
			if ok && hm["command"] == command {
				return true
			}
		}
	}
	return false
}
