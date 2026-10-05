package claude

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/jsonc"
)

func captureApplyOutput(t *testing.T, apply func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	applyErr := apply()
	_ = w.Close()
	os.Stdout = old
	data, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		return "", readErr
	}
	return string(data), applyErr
}

func TestApplyWithJevEnvironmentIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, ".claude")
	cfg := loadTestConfig(t)
	cfg.JevCompactionEnabled = true
	cfg.AgentsMD.Global.Target = filepath.Join(home, "CLAUDE.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil
	cfg.SkillsTarget = filepath.Join(home, ".gemini", "skills")
	cfg.CodexSkillsTarget = filepath.Join(home, ".codex", "skills")
	cfg.CodexHooksTarget = filepath.Join(home, ".codex", "config.toml")
	cfg.AgyHooksTarget = filepath.Join(home, ".gemini", "hooks.json")
	cfg.ClaudeSkillsTarget = filepath.Join(target, "skills")
	cfg.PrimeAgentTarget = filepath.Join(home, ".prime", "agent")
	cfg.DistillAutopipe.PiExtensionTarget = filepath.Join(home, ".pi", "harnez-distill.ts")
	cfg.DistillAutopipe.OpenCodePluginTarget = filepath.Join(home, ".config", "opencode", "harnez-distill.ts")
	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	settingsPath := filepath.Join(target, "settings.json")
	before, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(settingsPath, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	out, err := captureApplyOutput(t, func() error { return ApplyAll(target, cfg, nil, false, false) })
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	t.Logf("second isolated apply output:\n%s", out)
	if !strings.Contains(out, "No changes.") || strings.Contains(out, "env: changed") || strings.Contains(out, "2 changes.") {
		t.Fatalf("second apply output reports changes:\n%s", out)
	}
	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	afterInfo, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("settings.json bytes changed on second apply")
	}
	if !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatalf("settings.json mtime changed: %v -> %v", beforeInfo.ModTime(), afterInfo.ModTime())
	}
	env, ok := jsonc.Read(settingsPath)["env"].(map[string]any)
	if !ok || env["CLAUDE_CODE_ENABLE_FUNCTION_HOOKS"] != "1" {
		t.Fatalf("jev function hook flag missing after second apply: %#v", env)
	}
}

func TestApplyMergeEnvPreservesUnmanagedJSONTypesAndUpdatesManagedValue(t *testing.T) {
	existing := map[string]any{"env": map[string]any{
		"UNMANAGED_BOOL":   true,
		"UNMANAGED_NUMBER": float64(7),
		"UNMANAGED_NULL":   nil,
		"DEBUG":            false,
	}}
	doc := map[string]any{"env": map[string]string{"DEBUG": "true"}}
	merged := applyMerge(existing, doc)
	env := merged["env"].(map[string]any)
	if env["DEBUG"] != "true" || env["UNMANAGED_BOOL"] != true || env["UNMANAGED_NUMBER"] != float64(7) {
		t.Fatalf("merged env = %#v", env)
	}
	if value, exists := env["UNMANAGED_NULL"]; !exists || value != nil {
		t.Fatalf("unmanaged null entry lost: %#v", env)
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"env":{"DEBUG":"false","UNMANAGED_BOOL":true,"UNMANAGED_NUMBER":7,"UNMANAGED_NULL":null}}`), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := applySettingsJSON(path, doc)
	if err != nil || !result.changed {
		t.Fatalf("changed managed value result = %+v, %v; want a write", result, err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), `"DEBUG": "true"`) || !strings.Contains(string(after), `"UNMANAGED_BOOL": true`) {
		t.Fatalf("written settings did not preserve/update expected values: %s", after)
	}
	result, err = applySettingsJSON(path, doc)
	if err != nil || result.changed {
		t.Fatalf("stable managed value result = %+v, %v; want no write", result, err)
	}
}

func TestUnfilteredApplySettingsDoesNotWriteMCPServers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg, err := LoadConfig("testdata/m3-settings.yaml")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	target := filepath.Join(home, ".claude")
	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(target, "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	doc := jsonc.Read(string(data))
	if _, ok := doc["mcpServers"]; ok {
		t.Fatal("settings.json contains mcpServers")
	}
}

func TestConfiguredHarnezMCPPermissionIsApplied(t *testing.T) {
	cfg := loadTestConfig(t)
	doc := buildSettingsDoc(cfg, Set{})
	permissions := doc["permissions"].(map[string]any)
	if got := jsonc.ToStrings(permissions["allow"]); !slices.Contains(got, "mcp__harnez__*") {
		t.Fatalf("configured permissions.allow = %v, want mcp__harnez__*", got)
	}
}

func TestSelectedApplyRemovesOnlyHarnezOwnedSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, ".claude")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(target, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{
  "permissions": {"allow": ["user:permission"]},
		"hooks": {"PreToolUse": [{"hooks": [{"type": "command", "command": "harnez old hook"}, {"type": "command", "command": "user-hook"}]}], "Stop": [{"hooks": [{"type": "command", "command": "user-stop"}]}]},
	  "statusLine": {"type": "command", "command": "starship"}
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		Model:       "sonnet",
		Permissions: Permissions{Allow: []string{"harnez:permission", "mcp__harnez__*"}},
		Hooks: []Hook{
			{Event: "PreToolUse", Command: "harnez exec hook"},
			{Event: "PostToolUse", Command: "harnez only hook"},
		},
		StatusLine: true,
	}
	docsOnly := Set{Docs: true, Skills: true}
	if err := ApplyAllVariant(target, cfg, docsOnly, nil, false, false, "", false); err != nil {
		t.Fatalf("selected apply: %v", err)
	}

	doc := jsonc.Read(settingsPath)
	permissions := doc["permissions"].(map[string]any)
	if got := jsonc.ToStrings(permissions["allow"]); len(got) != 3 || !slices.Contains(got, "user:permission") || !slices.Contains(got, "harnez:permission") || !slices.Contains(got, "mcp__harnez__*") {
		t.Errorf("permissions.allow = %v, want existing and configured entries", got)
	}
	hooks := doc["hooks"].(map[string]any)
	if len(hooks) != 2 {
		t.Fatalf("hooks = %v, want both user hook events", hooks)
	}
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("mixed user/Harnez hook event was removed")
	}
	if _, ok := hooks["Stop"]; !ok {
		t.Error("user Stop hook was removed")
	}
	rendered := fmt.Sprint(hooks)
	if !strings.Contains(rendered, "user-hook") || !strings.Contains(rendered, "user-stop") {
		t.Errorf("user hook commands missing: %v", hooks)
	}
	if strings.Contains(rendered, "harnez old hook") || strings.Contains(rendered, "harnez only hook") {
		t.Errorf("Harnez hook commands survived: %v", hooks)
	}
	if got := doc["statusLine"].(map[string]any)["command"]; got != "starship" {
		t.Errorf("statusLine command = %v, want user status line", got)
	}

	if err := os.WriteFile(settingsPath, []byte(`{"statusLine":{"type":"command","command":"harnez statusline"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ApplyAllVariant(target, cfg, docsOnly, nil, false, false, "", false); err != nil {
		t.Fatalf("selected apply with Harnez status line: %v", err)
	}
	if _, ok := jsonc.Read(settingsPath)["statusLine"]; ok {
		t.Error("Harnez statusLine survived selected removal")
	}

	for _, existing := range []map[string]any{
		{"hooks": "malformed"},
		{"hooks": map[string]any{"PreToolUse": "malformed"}},
		// issue 491 M5 nit: an entry whose "hooks" value isn't a list must be
		// kept unchanged rather than dropped, like the other malformed cases.
		{"hooks": map[string]any{"PreToolUse": []any{map[string]any{"hooks": "not-a-list"}}}},
	} {
		got := applyMerge(existing, map[string]any{"hooks": nil})
		if !reflect.DeepEqual(got, existing) {
			t.Errorf("malformed hooks changed: got %v, want %v", got, existing)
		}
	}
}
