package claude

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/jsonc"
)

const (
	jevMarketplaceName = "harnez-local"
	jevPluginName      = "jev-compaction"
	jevPluginID        = jevPluginName + "@" + jevMarketplaceName
)

//go:embed jevcompaction
var jevPluginAssets embed.FS

type jevCompactionSpec struct {
	AutoTriggerPercent      int `yaml:"auto_trigger_percent"`
	MinimumReductionPercent int `yaml:"minimum_reduction_percent"`
	PinnedRecentMessages    int `yaml:"pinned_recent_messages"`
	ProcessTimeoutSeconds   int `yaml:"process_timeout_seconds"`
	MaxTranscriptBytes      int `yaml:"max_transcript_bytes"`
}

func loadJevCompactionSpec(source fs.FS) (jevCompactionSpec, error) {
	data, err := source.Open("spec/jev_compaction.yaml")
	if err != nil {
		return jevCompactionSpec{}, err
	}
	defer data.Close()
	var spec jevCompactionSpec
	decoder := yaml.NewDecoder(data)
	decoder.KnownFields(true)
	if err := decoder.Decode(&spec); err != nil {
		return jevCompactionSpec{}, err
	}
	if err := validateJevSpecSchema(source); err != nil {
		return jevCompactionSpec{}, err
	}
	return spec, nil
}

func renderJevHook(spec jevCompactionSpec) ([]byte, error) {
	template, err := jevPluginAssets.ReadFile("jevcompaction/hook.ts.tmpl")
	if err != nil {
		return nil, err
	}
	replacements := map[string]string{
		"{{AUTO_TRIGGER_PERCENT}}":      fmt.Sprint(spec.AutoTriggerPercent),
		"{{MINIMUM_REDUCTION_PERCENT}}": fmt.Sprint(spec.MinimumReductionPercent),
		"{{PINNED_RECENT_MESSAGES}}":    fmt.Sprint(spec.PinnedRecentMessages),
		"{{PROCESS_TIMEOUT_MS}}":        fmt.Sprint(spec.ProcessTimeoutSeconds * 1000),
		"{{MAX_TRANSCRIPT_BYTES}}":      fmt.Sprint(spec.MaxTranscriptBytes),
	}
	content := string(template)
	for marker, value := range replacements {
		content = strings.ReplaceAll(content, marker, value)
	}
	if strings.Contains(content, "{{") {
		return nil, fmt.Errorf("unexpanded token in jev compaction hook")
	}
	return []byte(content), nil
}

func applyJevCompactionPlugin(target string, enabled bool, preserveFunctionHookFlag ...bool) (applyResult, error) {
	settingsPath := filepath.Join(target, "settings.json")
	settings := jsonc.Read(settingsPath)
	changed := false
	setMapKey := func(parent map[string]any, key string, value any) {
		if !jsonEquivalent(parent[key], value) {
			parent[key] = value
			changed = true
		}
	}
	removeMapKey := func(parent map[string]any, key string) {
		if _, ok := parent[key]; ok {
			delete(parent, key)
			changed = true
		}
	}
	plugins, _ := settings["enabledPlugins"].(map[string]any)
	if plugins == nil {
		plugins = map[string]any{}
	}
	extraKnown, _ := settings["extraKnownMarketplaces"].(map[string]any)
	if extraKnown == nil {
		extraKnown = map[string]any{}
	}
	env, _ := settings["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
	}
	knownPath := filepath.Join(target, "plugins", "known_marketplaces.json")
	known := readJSONMap(knownPath)
	installedPath := filepath.Join(target, "plugins", "installed_plugins.json")
	installed := readJSONMap(installedPath)
	marketplaceDir := filepath.Join(target, "plugins", "marketplaces", jevMarketplaceName)
	cacheDir := filepath.Join(target, "plugins", "cache", jevMarketplaceName, jevPluginName, harnez.Version)
	_, pluginEnabled := plugins[jevPluginID]
	_, marketplaceManaged := known[jevMarketplaceName]
	installedPlugins, _ := installed["plugins"].(map[string]any)
	_, installedPlugin := installedPlugins[jevPluginID]
	pluginManaged := pluginEnabled || marketplaceManaged || installedPlugin || fileExists(filepath.Join(target, "plugins", "cache", jevMarketplaceName, jevPluginName))

	if enabled {
		spec, err := loadJevCompactionSpec(harnez.DefaultFS)
		if err != nil {
			return applyResult{}, err
		}
		hook, err := renderJevHook(spec)
		if err != nil {
			return applyResult{}, err
		}
		for _, item := range []struct {
			path string
			data []byte
		}{
			{filepath.Join(marketplaceDir, "marketplace.json"), mustAsset("jevcompaction/marketplace.json")},
			{filepath.Join(marketplaceDir, "plugins", jevPluginName, ".claude-plugin", "plugin.json"), renderVersionAsset("jevcompaction/plugin.json")},
			{filepath.Join(marketplaceDir, "plugins", jevPluginName, "hooks", "hooks.json"), mustAsset("jevcompaction/hooks.json")},
			{filepath.Join(marketplaceDir, "plugins", jevPluginName, "hooks", "jev-compact.ts"), hook},
			{filepath.Join(cacheDir, ".claude-plugin", "plugin.json"), renderVersionAsset("jevcompaction/plugin.json")},
			{filepath.Join(cacheDir, "hooks", "hooks.json"), mustAsset("jevcompaction/hooks.json")},
			{filepath.Join(cacheDir, "hooks", "jev-compact.ts"), hook},
		} {
			result, err := writeFileIfChanged(item.path, item.data)
			if err != nil {
				return applyResult{}, err
			}
			changed = changed || result.changed
		}
		setMapKey(plugins, jevPluginID, true)
		settings["enabledPlugins"] = plugins
		setMapKey(extraKnown, jevMarketplaceName, map[string]any{
			"source": map[string]any{"source": "directory", "path": marketplaceDir},
		})
		settings["extraKnownMarketplaces"] = extraKnown
		setMapKey(env, "CLAUDE_CODE_ENABLE_FUNCTION_HOOKS", "1")
		settings["env"] = env
		known[jevMarketplaceName] = map[string]any{
			"installLocation": marketplaceDir,
			"lastUpdated":     "1970-01-01T00:00:00.000Z",
			"source":          map[string]any{"source": "directory", "path": marketplaceDir},
		}
		installed["version"] = 2
		pluginEntries, _ := installed["plugins"].(map[string]any)
		if pluginEntries == nil {
			pluginEntries = map[string]any{}
		}
		entry := map[string]any{"scope": "user", "installPath": cacheDir, "version": harnez.Version}
		setMapKey(pluginEntries, jevPluginID, []any{entry})
		installed["plugins"] = pluginEntries
	} else {
		removeMapKey(plugins, jevPluginID)
		if len(plugins) == 0 {
			delete(settings, "enabledPlugins")
		} else {
			settings["enabledPlugins"] = plugins
		}
		if source, ok := extraKnown[jevMarketplaceName].(map[string]any); ok {
			if sourceDetails, ok := source["source"].(map[string]any); ok && sourceDetails["path"] == marketplaceDir {
				removeMapKey(extraKnown, jevMarketplaceName)
			}
		}
		if len(extraKnown) == 0 {
			delete(settings, "extraKnownMarketplaces")
		} else {
			settings["extraKnownMarketplaces"] = extraKnown
		}
		preserveFlag := len(preserveFunctionHookFlag) > 0 && preserveFunctionHookFlag[0]
		if pluginManaged && !preserveFlag {
			removeMapKey(env, "CLAUDE_CODE_ENABLE_FUNCTION_HOOKS")
		}
		if len(env) == 0 {
			delete(settings, "env")
		} else {
			settings["env"] = env
		}
		if marketplaceManaged {
			removeMapKey(known, jevMarketplaceName)
		}
		pluginEntries := installedPlugins
		removeMapKey(pluginEntries, jevPluginID)
		if len(pluginEntries) == 0 {
			delete(installed, "plugins")
		} else {
			installed["plugins"] = pluginEntries
		}
		for _, dir := range []string{
			filepath.Join(marketplaceDir, "plugins", jevPluginName),
			filepath.Join(target, "plugins", "cache", jevMarketplaceName, jevPluginName),
		} {
			if _, err := os.Stat(dir); err == nil {
				changed = true
			}
			if err := os.RemoveAll(dir); err != nil {
				return applyResult{}, err
			}
		}
		if marketplaceManaged {
			if err := os.RemoveAll(marketplaceDir); err != nil {
				return applyResult{}, err
			}
		}
	}
	for _, item := range []struct {
		path string
		doc  map[string]any
	}{{settingsPath, settings}, {knownPath, known}, {installedPath, installed}} {
		if len(item.doc) == 0 && enabled {
			continue
		}
		if len(item.doc) == 0 && !enabled {
			if err := os.Remove(item.path); err != nil && !os.IsNotExist(err) {
				return applyResult{}, err
			}
			continue
		}
		data := append(jsonc.MarshalPretty(item.doc), '\n')
		old, _ := os.ReadFile(item.path)
		if bytes.Equal(old, data) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(item.path), 0755); err != nil {
			return applyResult{}, err
		}
		if err := os.WriteFile(item.path, data, 0644); err != nil {
			return applyResult{}, err
		}
		changed = true
	}
	return applyResult{changed: changed}, nil
}

func mustAsset(name string) []byte {
	data, err := jevPluginAssets.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return data
}

func renderVersionAsset(name string) []byte {
	return []byte(strings.ReplaceAll(string(mustAsset(name)), "{{VERSION}}", harnez.Version))
}

func readJSONMap(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}
	}
	var result map[string]any
	if json.Unmarshal(data, &result) != nil || result == nil {
		return map[string]any{}
	}
	return result
}

func jsonEquivalent(left, right any) bool {
	a, _ := json.Marshal(left)
	b, _ := json.Marshal(right)
	return bytes.Equal(a, b)
}

func validateJevSpecSchema(source fs.FS) error {
	data, err := source.Open("spec/jev_compaction.yaml")
	if err != nil {
		return err
	}
	defer data.Close()
	var spec jevCompactionSpec
	decoder := yaml.NewDecoder(data)
	decoder.KnownFields(true)
	if err := decoder.Decode(&spec); err != nil {
		return err
	}
	if spec.AutoTriggerPercent < 1 || spec.AutoTriggerPercent > 100 || spec.MinimumReductionPercent < 1 || spec.MinimumReductionPercent > 100 || spec.PinnedRecentMessages < 0 || spec.ProcessTimeoutSeconds < 1 || spec.MaxTranscriptBytes < 1024 {
		return fmt.Errorf("jev compaction spec values violate schema bounds")
	}
	schema, err := source.Open("spec/schemas/jev_compaction.schema.json")
	if err != nil {
		return err
	}
	defer schema.Close()
	var document map[string]any
	if err := json.NewDecoder(schema).Decode(&document); err != nil {
		return err
	}
	properties, ok := document["properties"].(map[string]any)
	if !ok {
		return fmt.Errorf("jev compaction schema has no properties")
	}
	for _, key := range []string{"auto_trigger_percent", "minimum_reduction_percent", "pinned_recent_messages", "process_timeout_seconds", "max_transcript_bytes"} {
		if _, ok := properties[key]; !ok {
			return fmt.Errorf("jev compaction schema missing %s", key)
		}
	}
	return nil
}
