package claude

import (
	"encoding/json"
	"fmt"
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
	copyPath := filepath.Join(t.TempDir(), "config.yaml")
	embedded, err := harnez.DefaultFS.ReadFile("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(embedded), "jev_compaction_enabled:") {
		t.Fatal("embedded config opt-in value is missing")
	}
	embedded = []byte(strings.Replace(string(embedded), "jev_compaction_enabled: true", "jev_compaction_enabled: false", 1))
	if err := os.WriteFile(copyPath, embedded, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JevCompactionEnabled {
		t.Fatal("explicitly disabled custom config opt-in was ignored")
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
		t.Fatal("explicitly enabled custom config opt-in was ignored")
	}
}

func TestApplyJevCompactionPluginWriteIdempotencyAndRemoval(t *testing.T) {
	target := t.TempDir()
	settingsPath := filepath.Join(target, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"enabledPlugins":{"other@market":true},"env":{"KEEP":"yes"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := applyManagedLocalPlugins(target, true, true)
	if err != nil || !result.changed {
		t.Fatalf("first apply = %+v, %v", result, err)
	}
	pluginRoot := filepath.Join(target, "plugins", "cache", jevMarketplaceName, jevPluginName, harnez.Version)
	for _, name := range []string{".claude-plugin/plugin.json", "hooks/hooks.json", "hooks/jev-compact.ts"} {
		if _, err := os.Stat(filepath.Join(pluginRoot, name)); err != nil {
			t.Errorf("plugin file %s: %v", name, err)
		}
	}
	result, err = applyManagedLocalPlugins(target, true, true)
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

	result, err = applyManagedLocalPlugins(target, false, true)
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
	settings = nil
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if settings["enabledPlugins"].(map[string]any)["other@market"] != true {
		t.Errorf("removal dropped unrelated plugin: %#v", settings["enabledPlugins"])
	}
	marketplaces, _ = settings["extraKnownMarketplaces"].(map[string]any)
	if _, ok := marketplaces[jevMarketplaceName]; !ok {
		t.Errorf("channel marketplace removed with compaction: %#v", settings["extraKnownMarketplaces"])
	}
	if _, ok := settings["env"].(map[string]any)["CLAUDE_CODE_ENABLE_FUNCTION_HOOKS"]; ok {
		t.Errorf("function hook flag remains after removal: %#v", settings["env"])
	}
}

// All plugin state is written under temporary targets, including migration repairs.
func TestManagedMarketplaceChannelRepair(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			target := t.TempDir()
			marketplaceDir := filepath.Join(target, "plugins", "marketplaces", jevMarketplaceName)
			knownPath := filepath.Join(target, "plugins", "known_marketplaces.json")
			oldDir := t.TempDir()
			if err := os.MkdirAll(filepath.Dir(knownPath), 0755); err != nil {
				t.Fatal(err)
			}
			seed := map[string]any{jevMarketplaceName: map[string]any{"installLocation": oldDir, "source": map[string]any{"source": "directory", "path": oldDir}}, "other": map[string]any{"installLocation": oldDir}}
			data, _ := json.Marshal(seed)
			if err := os.WriteFile(knownPath, data, 0644); err != nil {
				t.Fatal(err)
			}
			result, err := applyManagedLocalPlugins(target, enabled, true)
			if err != nil || !result.changed {
				t.Fatalf("apply: %+v, %v", result, err)
			}
			market := readJSONMap(filepath.Join(marketplaceDir, ".claude-plugin", "marketplace.json"))
			entries := market["plugins"].([]any)
			if len(entries) != 2 {
				t.Fatalf("plugins: %#v", entries)
			}
			names := map[string]bool{}
			for _, entry := range entries {
				names[entry.(map[string]any)["name"].(string)] = true
			}
			if !names[jevPluginName] || !names["harnez-channel"] {
				t.Fatalf("plugins: %#v", names)
			}
			cache := filepath.Join(target, "plugins", "cache", jevMarketplaceName, "harnez-channel", harnez.Version)
			for _, rel := range []string{".claude-plugin/plugin.json", ".mcp.json"} {
				cached, err := os.ReadFile(filepath.Join(cache, rel))
				if err != nil {
					t.Fatal(err)
				}
				source, err := os.ReadFile(filepath.Join(marketplaceDir, "plugins", "harnez-channel", rel))
				if err != nil {
					t.Fatal(err)
				}
				if string(cached) != string(source) {
					t.Fatalf("cache differs: %s", rel)
				}
			}
			manifest := readJSONMap(filepath.Join(cache, ".claude-plugin", "plugin.json"))
			if manifest["name"] != "harnez-channel" || manifest["version"] != harnez.Version {
				t.Fatalf("manifest: %#v", manifest)
			}
			settings := readJSONMap(filepath.Join(target, "settings.json"))
			id := "harnez-channel@" + jevMarketplaceName
			if settings["enabledPlugins"].(map[string]any)[id] != true {
				t.Fatal("channel not enabled")
			}
			installed := readJSONMap(filepath.Join(target, "plugins", "installed_plugins.json"))
			entry := installed["plugins"].(map[string]any)[id].([]any)[0].(map[string]any)
			if entry["installPath"] != cache || entry["version"] != harnez.Version || entry["scope"] != "user" {
				t.Fatalf("installed: %#v", entry)
			}
			known := readJSONMap(knownPath)
			repaired := known[jevMarketplaceName].(map[string]any)
			if repaired["installLocation"] != marketplaceDir || repaired["source"].(map[string]any)["path"] != marketplaceDir || known["other"] == nil {
				t.Fatalf("known: %#v", known)
			}
			result, err = applyManagedLocalPlugins(target, enabled, true)
			if err != nil || result.changed {
				t.Fatalf("second apply: %+v, %v", result, err)
			}
		})
	}
}
