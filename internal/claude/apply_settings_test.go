package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/jsonc"
)

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
  "mcpServers": {"user-server": {"command": "user-mcp"}},
  "statusLine": {"type": "command", "command": "starship"}
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		Model:       "sonnet",
		Permissions: Permissions{Allow: []string{"harnez:permission"}},
		Hooks:       []Hook{{Event: "PreToolUse", Command: "harnez exec hook"}},
		MCPServers:  []MCPServer{{Name: "harnez-server", Command: "harnez-mcp"}},
		StatusLine:  true,
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
	servers := doc["mcpServers"].(map[string]any)
	if _, ok := servers["user-server"]; !ok {
		t.Error("user mcpServers entry was removed")
	}
	if _, ok := servers["harnez-server"]; !ok {
		t.Error("configured mcpServers entry was not applied")
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
	if rendered := fmt.Sprint(hooks); !strings.Contains(rendered, "user-hook") || !strings.Contains(rendered, "user-stop") {
		t.Errorf("user hook commands missing: %v", hooks)
	}
	if got := doc["statusLine"].(map[string]any)["command"]; got != "starship" {
		t.Errorf("statusLine command = %v, want user status line", got)
	}
}
