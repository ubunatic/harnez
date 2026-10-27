package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"ubunatic.com/harnez/internal/jsonc"
)

// mergePermissions unions the allow/deny arrays of existing and incoming.
func mergePermissions(existing, incoming map[string]any) map[string]any {
	result := make(map[string]any, len(existing))
	for k, v := range existing {
		result[k] = v
	}
	for _, field := range []string{"allow", "deny"} {
		result[field] = jsonc.UnionStrings(jsonc.ToStrings(existing[field]), jsonc.ToStrings(incoming[field]))
	}
	return result
}

// applyMerge merges doc into a copy of existing. The permissions.allow and
// permissions.deny arrays are union-merged; all other top-level keys replace.
// A nil value removes only managed, Harnez-owned settings.
func applyMerge(existing, doc map[string]any) map[string]any {
	out := make(map[string]any, len(existing)+len(doc))
	for k, v := range existing {
		out[k] = v
	}
	for k, v := range doc {
		if v == nil {
			if !isManagedSettingsKey(k) {
				continue
			}
			switch k {
			case "hooks":
				keptHooks, valid := removeHarnezHooks(out[k])
				if !valid {
					continue
				}
				if len(keptHooks) == 0 {
					delete(out, k)
				} else {
					out[k] = keptHooks
				}
			case "statusLine":
				if statusLine, ok := out[k].(map[string]any); ok {
					if command, _ := statusLine["command"].(string); isHarnezCommand(command) {
						delete(out, k)
					}
				}
			}
			continue
		}
		if k == "permissions" {
			if ep, ok := out[k].(map[string]any); ok {
				if ip, ok := v.(map[string]any); ok {
					out[k] = mergePermissions(ep, ip)
					continue
				}
			}
		}
		if k == "hooks" {
			kept, valid := removeHarnezHooks(out[k])
			if !valid {
				kept = map[string]any{}
			}
			encoded, err := json.Marshal(v)
			if err == nil {
				var incoming map[string][]any
				if json.Unmarshal(encoded, &incoming) == nil {
					for event, entries := range incoming {
						prior, _ := kept[event].([]any)
						kept[event] = append(prior, entries...)
					}
					out[k] = kept
					continue
				}
			}
		}
		if k == "env" {
			if existingEnv, ok := out[k].(map[string]any); ok {
				if configuredEnv, ok := v.(map[string]string); ok {
					mergedEnv := make(map[string]any, len(existingEnv)+len(configuredEnv))
					for envKey, envValue := range existingEnv {
						mergedEnv[envKey] = envValue
					}
					for envKey, envValue := range configuredEnv {
						mergedEnv[envKey] = envValue
					}
					out[k] = mergedEnv
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}

func isManagedSettingsKey(key string) bool {
	return slices.Contains(managedSettingsKeys, key)
}

func isHarnezCommand(command string) bool {
	command = strings.TrimSpace(command)
	return command == "harnez" || strings.HasPrefix(command, "harnez ")
}

func removeHarnezHooks(value any) (map[string]any, bool) {
	hooks, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	keptHooks := make(map[string]any, len(hooks))
	for event, rawEntries := range hooks {
		entries, ok := rawEntries.([]any)
		if !ok {
			return nil, false
		}
		kept := make([]any, 0, len(entries))
		for _, entry := range entries {
			entryMap, ok := entry.(map[string]any)
			if !ok {
				kept = append(kept, entry)
				continue
			}
			rawHooks, ok := entryMap["hooks"].([]any)
			if !ok {
				kept = append(kept, entry)
				continue
			}
			entryCopy := make(map[string]any, len(entryMap))
			for key, value := range entryMap {
				entryCopy[key] = value
			}
			keptCommands := make([]any, 0, len(rawHooks))
			for _, rawHook := range rawHooks {
				hook, ok := rawHook.(map[string]any)
				command, commandOK := hook["command"].(string)
				if ok && commandOK && isHarnezCommand(command) {
					continue
				}
				keptCommands = append(keptCommands, rawHook)
			}
			if len(keptCommands) > 0 {
				entryCopy["hooks"] = keptCommands
				kept = append(kept, entryCopy)
			}
		}
		if len(kept) > 0 {
			keptHooks[event] = kept
		}
	}
	return keptHooks, true
}

// managedSettingsKeys are the top-level keys harnez writes to settings.json.
var managedSettingsKeys = []string{
	"model", "effortLevel", "permissions", "hooks", "env", "spinnerVerbs", "statusLine",
}

// Model alias mappings.
var modelAliases = map[string]string{
	"sonnet": "claude-sonnet-4-6",
	"opus":   "claude-opus-4-7",
	"haiku":  "claude-haiku-5-5",
}

func resolveModel(s string) string {
	if full, ok := modelAliases[strings.ToLower(s)]; ok {
		return full
	}
	return s
}

// buildSettingsDoc creates the settings JSON document from the YAML config.
func buildSettingsDoc(cfg *Config, selection Set) map[string]any {
	// Use map[string]any throughout so json.Marshal always sorts keys alphabetically,
	// matching what json.Unmarshal produces on read-back (round-trip stable).
	eventMap := map[string][]map[string]any{}
	for _, h := range cfg.Hooks {
		if !selection.HasComponent("telemetry") && isHarnezCommand(h.Command) {
			continue
		}
		m := map[string]any{
			"hooks": []map[string]any{{"command": h.Command, "type": "command"}},
		}
		if h.Matcher != "" {
			m["matcher"] = h.Matcher
		}
		eventMap[h.Event] = append(eventMap[h.Event], m)
	}

	doc := map[string]any{}
	if cfg.Model != "" {
		doc["model"] = resolveModel(cfg.Model)
	}
	if cfg.Effort != "" {
		doc["effortLevel"] = cfg.Effort
	}
	if len(cfg.Permissions.Allow) > 0 || len(cfg.Permissions.Deny) > 0 {
		allow := cfg.Permissions.Allow
		if allow == nil {
			allow = []string{}
		}
		deny := cfg.Permissions.Deny
		if deny == nil {
			deny = []string{}
		}
		doc["permissions"] = map[string]any{"allow": allow, "deny": deny}
	}
	if len(eventMap) > 0 {
		doc["hooks"] = eventMap
	} else if !selection.HasComponent("telemetry") {
		doc["hooks"] = nil
	}
	if len(cfg.Env) > 0 {
		doc["env"] = cfg.Env
	}
	if len(cfg.Verbs) > 0 {
		doc["spinnerVerbs"] = map[string]any{"mode": "replace", "verbs": cfg.Verbs}
	}
	if cfg.StatusLine && selection.HasComponent("usage") {
		// cwd-only MVP; Claude Code renders this on its own row above the
		// built-in footer badges, it cannot share that row (see issue 095).
		doc["statusLine"] = map[string]any{
			"type":    "command",
			"command": "harnez statusline",
		}
	} else if !cfg.StatusLine || !selection.HasComponent("usage") {
		doc["statusLine"] = nil
	}
	return doc
}

// settingsStats returns human-readable change notes comparing existing → merged
// for the managed keys (permissions arrays counted, other keys flagged as changed/added).
func settingsStats(existing, merged map[string]any) []string {
	var notes []string

	// permissions: report per-array delta
	ep, _ := existing["permissions"].(map[string]any)
	mp, _ := merged["permissions"].(map[string]any)
	if mp != nil {
		for _, field := range []string{"allow", "deny"} {
			oldN := len(jsonc.ToStrings(ep[field]))
			newN := len(jsonc.ToStrings(mp[field]))
			if newN != oldN {
				notes = append(notes, fmt.Sprintf("permissions.%s: %+d entries (%d → %d)", field, newN-oldN, oldN, newN))
			}
		}
	}

	// other managed keys
	for _, k := range managedSettingsKeys {
		if k == "permissions" {
			continue
		}
		if !bytes.Equal(jsonc.MarshalPretty(existing[k]), jsonc.MarshalPretty(merged[k])) {
			if existing[k] == nil {
				notes = append(notes, k+": added")
			} else {
				notes = append(notes, k+": changed")
			}
		}
	}
	return notes
}

func applySettingsJSON(path string, doc map[string]any) (applyResult, error) {
	existing := jsonc.Read(path)
	merged := applyMerge(existing, doc)
	data := append(jsonc.MarshalPretty(merged), '\n')
	if err := jsonc.Validate(data); err != nil {
		return applyResult{}, fmt.Errorf("%s: %w", path, err)
	}

	oldData, _ := os.ReadFile(path)
	if bytes.Equal(oldData, data) {
		return applyResult{changed: false}, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return applyResult{}, err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return applyResult{}, err
	}
	return applyResult{changed: true, notes: settingsStats(existing, merged)}, nil
}

func diffSettingsJSON(path string, doc map[string]any) (bool, error) {
	existing := jsonc.Read(path)
	merged := applyMerge(existing, doc)

	currentManaged := map[string]any{}
	for k := range doc {
		if v, ok := existing[k]; ok {
			currentManaged[k] = v
		}
	}
	newManaged := map[string]any{}
	for k := range doc {
		if v, ok := merged[k]; ok {
			newManaged[k] = v
		}
	}

	oldData := jsonc.MarshalPretty(currentManaged)
	newData := jsonc.MarshalPretty(newManaged)
	if bytes.Equal(oldData, newData) {
		return false, nil
	}

	writeTemp := func(data []byte) (string, error) {
		f, err := os.CreateTemp("", "harnez-diff-*")
		if err != nil {
			return "", err
		}
		_, err = f.Write(data)
		f.Close()
		return f.Name(), err
	}

	oldFile, err := writeTemp(oldData)
	if err != nil {
		return false, err
	}
	defer os.Remove(oldFile)

	newFile, err := writeTemp(newData)
	if err != nil {
		return false, err
	}
	defer os.Remove(newFile)

	label := fmt.Sprintf("%s [settings]", path)
	cmd := exec.Command("diff", "-u", "--label", label, "--label", label, oldFile, newFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("diff %s: %w", label, err)
	}
	return true, nil
}

func cleanSettingsJSON(path string) error {
	existing := jsonc.Read(path)
	if len(existing) == 0 {
		return nil
	}
	for _, k := range managedSettingsKeys {
		delete(existing, k)
	}
	if len(existing) == 0 {
		fmt.Printf("  removed %s\n", path)
		return os.Remove(path)
	}
	fmt.Printf("  cleaned %s\n", path)
	return os.WriteFile(path, append(jsonc.MarshalPretty(existing), '\n'), 0644)
}

// applySettings writes global settings and records the unchanged-settings summary.
func applySettings(r *applyReport, target string, cfg *Config, selection Set) error {
	settingsPath := filepath.Join(target, "settings.json")
	settingsDoc := buildSettingsDoc(cfg, selection)
	sr, err := applySettingsJSON(settingsPath, settingsDoc)
	if err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	if sr.changed {
		r.changes++
	} else {
		existing := jsonc.Read(settingsPath)
		if p, _ := existing["permissions"].(map[string]any); p != nil {
			allow := jsonc.ToStrings(p["allow"])
			deny := jsonc.ToStrings(p["deny"])
			r.addStat("permissions", fmt.Sprintf("%d allow, %d deny", len(allow), len(deny)))
		}
		var keys []string
		for _, k := range managedSettingsKeys {
			if k == "permissions" {
				continue
			}
			if _, inDoc := settingsDoc[k]; inDoc {
				if _, inFile := existing[k]; inFile {
					keys = append(keys, k)
				}
			}
		}
		r.addStat("settings", strings.Join(keys, ", "))
	}
	printResult("wrote", settingsPath, sr)
	return nil
}
