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

//go:embed jevcompaction channelplugin
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

func applyManagedLocalPlugins(target string, enabled bool, channelEnabled bool, preserveFunctionHookFlag ...bool) (applyResult, error) {
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
	installedPlugins, _ := installed["plugins"].(map[string]any)
	if installedPlugins == nil {
		installedPlugins = map[string]any{}
	}
	_, pluginEnabled := plugins[jevPluginID]
	_, installedPlugin := installedPlugins[jevPluginID]
	pluginManaged := pluginEnabled || installedPlugin || fileExists(filepath.Join(target, "plugins", "cache", jevMarketplaceName, jevPluginName))

	// Apply installs the channel independently of compaction; clean disables both.
	type pluginFile struct {
		path string
		data []byte
	}
	managedPlugins := []struct {
		name    string
		enabled bool
		files   []pluginFile
	}{
		{name: "harnez-channel", enabled: channelEnabled, files: []pluginFile{
			{".claude-plugin/plugin.json", renderVersionAsset("channelplugin/plugin.json")},
			{".mcp.json", mustAsset("channelplugin/mcp.json")},
		}},
		{name: jevPluginName, enabled: enabled},
	}
	if enabled {
		spec, err := loadJevCompactionSpec(harnez.DefaultFS)
		if err != nil {
			return applyResult{}, err
		}
		hook, err := renderJevHook(spec)
		if err != nil {
			return applyResult{}, err
		}
		managedPlugins[1].files = []pluginFile{
			{".claude-plugin/plugin.json", renderVersionAsset("jevcompaction/plugin.json")},
			{"hooks/hooks.json", mustAsset("jevcompaction/hooks.json")},
			{"hooks/jev-compact.ts", hook},
		}
		setMapKey(env, "CLAUDE_CODE_ENABLE_FUNCTION_HOOKS", "1")
	} else if pluginManaged && !(len(preserveFunctionHookFlag) > 0 && preserveFunctionHookFlag[0]) {
		removeMapKey(env, "CLAUDE_CODE_ENABLE_FUNCTION_HOOKS")
	}
	for _, plugin := range managedPlugins {
		id := plugin.name + "@" + jevMarketplaceName
		pluginDir := filepath.Join(marketplaceDir, "plugins", plugin.name)
		cacheRoot := filepath.Join(target, "plugins", "cache", jevMarketplaceName, plugin.name)
		cacheDir := filepath.Join(cacheRoot, harnez.Version)
		if plugin.enabled {
			for _, root := range []string{pluginDir, cacheDir} {
				for _, file := range plugin.files {
					result, err := writeFileIfChanged(filepath.Join(root, file.path), file.data)
					if err != nil {
						return applyResult{}, err
					}
					changed = changed || result.changed
				}
			}
			setMapKey(plugins, id, true)
			setMapKey(installedPlugins, id, []any{map[string]any{"scope": "user", "installPath": cacheDir, "version": harnez.Version}})
		} else {
			removeMapKey(plugins, id)
			removeMapKey(installedPlugins, id)
			for _, dir := range []string{pluginDir, cacheRoot} {
				if fileExists(dir) {
					changed = true
				}
				if err := os.RemoveAll(dir); err != nil {
					return applyResult{}, err
				}
			}
		}
	}
	if enabled || channelEnabled {
		// Claude reads directory marketplaces only from .claude-plugin/marketplace.json.
		result, err := writeFileIfChanged(filepath.Join(marketplaceDir, ".claude-plugin", "marketplace.json"), mustAsset("jevcompaction/marketplace.json"))
		if err != nil {
			return applyResult{}, err
		}
		changed = changed || result.changed
		staleManifest := filepath.Join(marketplaceDir, "marketplace.json")
		if fileExists(staleManifest) {
			if err := os.Remove(staleManifest); err != nil {
				return applyResult{}, err
			}
			changed = true
		}
		setMapKey(extraKnown, jevMarketplaceName, map[string]any{
			"source": map[string]any{"source": "directory", "path": marketplaceDir},
		})
		setMapKey(known, jevMarketplaceName, map[string]any{
			"installLocation": marketplaceDir,
			"lastUpdated":     "1970-01-01T00:00:00.000Z",
			"source":          map[string]any{"source": "directory", "path": marketplaceDir},
		})
		setMapKey(installed, "version", 2)
	} else {
		if source, ok := extraKnown[jevMarketplaceName].(map[string]any); ok {
			if details, ok := source["source"].(map[string]any); ok && details["path"] == marketplaceDir {
				removeMapKey(extraKnown, jevMarketplaceName)
			}
		}
		if source, ok := known[jevMarketplaceName].(map[string]any); ok && source["installLocation"] == marketplaceDir {
			removeMapKey(known, jevMarketplaceName)
			if err := os.RemoveAll(marketplaceDir); err != nil {
				return applyResult{}, err
			}
		}
	}
	for key, value := range map[string]map[string]any{"enabledPlugins": plugins, "extraKnownMarketplaces": extraKnown} {
		if len(value) == 0 {
			delete(settings, key)
		} else {
			settings[key] = value
		}
	}
	if len(env) == 0 {
		delete(settings, "env")
	} else {
		settings["env"] = env
	}
	if len(installedPlugins) == 0 {
		delete(installed, "plugins")
	} else {
		installed["plugins"] = installedPlugins
	}
	for _, item := range []struct {
		path string
		doc  map[string]any
	}{{settingsPath, settings}, {knownPath, known}, {installedPath, installed}} {
		if len(item.doc) == 0 {
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
