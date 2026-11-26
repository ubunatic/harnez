package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/jsonc"
)

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
	cfg := loadTestConfig(t)

	// Issue 1304 M2: one PreToolUse entry runs every PreToolUse handler, and
	// its matcher lists the tools, Bash among them.
	var found *Hook
	for i := range cfg.Hooks {
		h := &cfg.Hooks[i]
		if h.Event == "PreToolUse" && h.Command == "harnez hook PreToolUse" && matcherHasTool(h.Matcher, "Bash") {
			found = h
			break
		}
	}
	if found == nil {
		t.Fatalf("expected a PreToolUse hook with command %q and a Bash matcher in embedded config, hooks: %+v",
			"harnez hook PreToolUse", cfg.Hooks)
	}
}

// matcherHasTool reports whether a "|" separated Claude matcher names tool.
func matcherHasTool(matcher, tool string) bool {
	for _, name := range strings.Split(matcher, "|") {
		if name == tool {
			return true
		}
	}
	return false
}

// TestApplyInstallsTelemetryHook exercises the same install → idempotent
// re-apply → clean lifecycle TestIntegrationWorkflow covers for the rest
// of apply's managed keys, focused on the new telemetry hook entry.
func TestApplyInstallsTelemetryHook(t *testing.T) {
	cfg := loadTestConfig(t)
	targetDir := t.TempDir()
	settingsPath := filepath.Join(targetDir, "settings.json")

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
		t.Fatalf("expected settings.json to contain the PreToolUse Bash hook (harnez hook PreToolUse) after apply")
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
			matcher, _ := em["matcher"].(string)
			if !ok || !matcherHasTool(matcher, "Bash") {
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
	out, err := CaptureTestStdout(func() error {
		return RunStatus("(embedded)", cfg, targetDir, nil)
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
		matcher, _ := em["matcher"].(string)
		if !ok || !matcherHasTool(matcher, "Bash") {
			continue
		}
		hs, _ := em["hooks"].([]any)
		for _, h := range hs {
			hm, ok := h.(map[string]any)
			if ok && hm["command"] == "harnez hook PreToolUse" {
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
	cfg := loadTestConfig(t)

	// Issue 1304 M2: the read tools share the single PreToolUse entry.
	for _, tool := range []string{"View", "ReadMultipleFiles"} {
		found := false
		for i := range cfg.Hooks {
			h := &cfg.Hooks[i]
			if h.Event == "PreToolUse" && h.Command == "harnez hook PreToolUse" && matcherHasTool(h.Matcher, tool) {
				found = true
			}
		}
		if !found {
			t.Errorf("expected PreToolUse hook covering tool %q with command %q, hooks: %+v",
				tool, "harnez hook PreToolUse", cfg.Hooks)
		}
	}
}

func TestApplyInstallsReadHooks(t *testing.T) {
	cfg := loadTestConfig(t)
	targetDir := t.TempDir()
	settingsPath := filepath.Join(targetDir, "settings.json")

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

	// Issue 1304 M2: every PreToolUse tool is covered by the one "harnez hook PreToolUse" entry.
	for _, tool := range []string{"View", "ReadMultipleFiles", "Edit", "Write"} {
		if !settingsHasHook(t, settingsPath, "PreToolUse", tool, "harnez hook PreToolUse") {
			t.Errorf("expected settings.json to contain PreToolUse hook covering %s", tool)
		}
	}

	// Idempotency: second apply must preserve hooks without duplication
	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("second ApplyAll failed: %v", err)
	}
	for _, tool := range []string{"View", "ReadMultipleFiles", "Edit", "Write"} {
		if !settingsHasHook(t, settingsPath, "PreToolUse", tool, "harnez hook PreToolUse") {
			t.Errorf("expected settings.json to retain PreToolUse hook covering %s after second apply", tool)
		}
	}
	if n := countPreToolUseHarnezEntries(t, settingsPath); n != 1 {
		t.Errorf("expected exactly one harnez PreToolUse entry after second apply, got %d", n)
	}
}

// countPreToolUseHarnezEntries counts PreToolUse entries that run harnez.
func countPreToolUseHarnezEntries(t *testing.T, settingsPath string) int {
	t.Helper()
	hooks, _ := jsonc.Read(settingsPath)["hooks"].(map[string]any)
	entries, _ := hooks["PreToolUse"].([]any)
	n := 0
	for _, entry := range entries {
		em, _ := entry.(map[string]any)
		hs, _ := em["hooks"].([]any)
		for _, h := range hs {
			hm, _ := h.(map[string]any)
			if command, _ := hm["command"].(string); strings.HasPrefix(command, "harnez ") {
				n++
			}
		}
	}
	return n
}

// settingsHasHook reports whether a settings event has an entry whose matcher
// names tool and that runs command.
func settingsHasHook(t *testing.T, settingsPath, event, tool, command string) bool {
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
		matcher, _ := em["matcher"].(string)
		if !ok || !matcherHasTool(matcher, tool) {
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
