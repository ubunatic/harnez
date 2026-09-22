package claude

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/jsonc"
)

func TestUnfilteredApplySettingsMatchesFixedGolden(t *testing.T) {
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
	got := sha256.Sum256(data)
	const want = "104fe8958c49399cc9160624f1934dac0735d3cef8a0b801cf5db79c388c38e4"
	if actual := fmt.Sprintf("%x", got); actual != want {
		t.Fatalf("settings.json SHA-256 = %s, want fixed golden %s", actual, want)
	}
}

type testComponentSelection map[string]bool

func (s testComponentSelection) HasComponent(name string) bool {
	return s[name]
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
		Permissions: Permissions{Allow: []string{"harnez:permission"}},
		Hooks: []Hook{
			{Event: "PreToolUse", Command: "harnez exec hook"},
			{Event: "PostToolUse", Command: "harnez only hook"},
		},
		MCPServers: []MCPServer{{Name: "harnez-server", Command: "harnez-mcp"}},
		StatusLine: true,
	}
	docsOnly := testComponentSelection{"docs": true, "skills": true}
	if err := ApplyAllVariant(target, cfg, docsOnly, nil, false, false, "", false); err != nil {
		t.Fatalf("selected apply: %v", err)
	}

	doc := jsonc.Read(settingsPath)
	permissions := doc["permissions"].(map[string]any)
	if got := jsonc.ToStrings(permissions["allow"]); len(got) != 2 || !slices.Contains(got, "user:permission") || !slices.Contains(got, "harnez:permission") {
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
	} {
		got := applyMerge(existing, map[string]any{"hooks": nil})
		if !reflect.DeepEqual(got, existing) {
			t.Errorf("malformed hooks changed: got %v, want %v", got, existing)
		}
	}
}
