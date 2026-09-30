package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez"
)

func TestJevCompactionSpecEmbeddedAndSchemaAligned(t *testing.T) {
	if err := validateJevSpecSchema(harnez.DefaultFS); err != nil {
		t.Fatalf("embedded YAML/schema validation: %v", err)
	}
	spec, err := loadJevCompactionSpec(harnez.DefaultFS)
	if err != nil {
		t.Fatal(err)
	}
	if spec.AutoTriggerPercent <= 0 || spec.MinimumReductionPercent <= 0 || spec.PinnedRecentMessages < 0 {
		t.Fatalf("unexpected embedded spec: %+v", spec)
	}
	hook, err := renderJevHook(spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		"const AUTO_TRIGGER_PERCENT = 60;",
		"const MINIMUM_REDUCTION_PERCENT = 25;",
		"const PINNED_RECENT_MESSAGES = 5;",
	} {
		if !strings.Contains(string(hook), value) {
			t.Errorf("rendered hook missing spec value %q", value)
		}
	}
	for _, path := range []string{
		"jevcompaction/marketplace.json",
		"jevcompaction/plugin.json",
		"jevcompaction/hooks.json",
		"jevcompaction/hook.ts.tmpl",
	} {
		if _, err := jevPluginAssets.Open(path); err != nil {
			t.Errorf("embedded asset %s: %v", path, err)
		}
	}
}

func TestJevCompactionApplyOptInDefaultsOff(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JevCompactionEnabled {
		t.Fatal("embedded config must leave the plugin opt-in disabled")
	}
	copyPath := filepath.Join(t.TempDir(), "config.yaml")
	embedded, err := harnez.DefaultFS.ReadFile("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(embedded), "jev_compaction_enabled: false") {
		t.Fatal("embedded config opt-in value is missing")
	}
	embedded = []byte(strings.Replace(string(embedded), "jev_compaction_enabled: false", "jev_compaction_enabled: true", 1))
	if err := os.WriteFile(copyPath, embedded, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadConfig(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.JevCompactionEnabled {
		t.Fatal("custom config opt-in was ignored")
	}
}

func TestApplyJevCompactionPluginWriteIdempotencyAndRemoval(t *testing.T) {
	target := t.TempDir()
	settingsPath := filepath.Join(target, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"enabledPlugins":{"other@market":true},"env":{"KEEP":"yes"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := applyJevCompactionPlugin(target, true)
	if err != nil || !result.changed {
		t.Fatalf("first apply = %+v, %v", result, err)
	}
	pluginRoot := filepath.Join(target, "plugins", "cache", jevMarketplaceName, jevPluginName, harnez.Version)
	for _, name := range []string{".claude-plugin/plugin.json", "hooks/hooks.json", "hooks/jev-compact.ts"} {
		if _, err := os.Stat(filepath.Join(pluginRoot, name)); err != nil {
			t.Errorf("plugin file %s: %v", name, err)
		}
	}
	result, err = applyJevCompactionPlugin(target, true)
	if err != nil || result.changed {
		t.Fatalf("second apply = %+v, %v; want unchanged", result, err)
	}

	var settings map[string]any
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if settings["enabledPlugins"].(map[string]any)["other@market"] != true || settings["env"].(map[string]any)["KEEP"] != "yes" {
		t.Fatalf("unrelated settings were not preserved: %#v", settings)
	}
	marketplaces := settings["extraKnownMarketplaces"].(map[string]any)
	if marketplaces[jevMarketplaceName] == nil {
		t.Fatalf("local plugin marketplace not registered: %#v", marketplaces)
	}
	if settings["env"].(map[string]any)["CLAUDE_CODE_ENABLE_FUNCTION_HOOKS"] != "1" {
		t.Fatalf("function hook runtime not enabled: %#v", settings["env"])
	}

	result, err = applyJevCompactionPlugin(target, false)
	if err != nil || !result.changed {
		t.Fatalf("remove = %+v, %v", result, err)
	}
	if _, err := os.Stat(pluginRoot); !os.IsNotExist(err) {
		t.Errorf("plugin cache remains after removal: %v", err)
	}
	data, err = os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if settings["enabledPlugins"].(map[string]any)["other@market"] != true {
		t.Errorf("removal dropped unrelated plugin: %#v", settings["enabledPlugins"])
	}
	marketplaces, _ = settings["extraKnownMarketplaces"].(map[string]any)
	if _, ok := marketplaces[jevMarketplaceName]; ok {
		t.Errorf("local plugin marketplace remains after removal: %#v", settings["extraKnownMarketplaces"])
	}
	if _, ok := settings["env"].(map[string]any)["CLAUDE_CODE_ENABLE_FUNCTION_HOOKS"]; ok {
		t.Errorf("function hook flag remains after removal: %#v", settings["env"])
	}
}
