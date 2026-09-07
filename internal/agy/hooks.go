// Package agy manages harnez's integration with agy's (Antigravity CLI)
// native hooks.json lifecycle-hook system — the "tell agy" complement to
// the quiet PATH-shim in issues/195. See
// issues/196-agy-native-hooks-plan-alongside-claude-hooks.md for the plan
// this implements, and issues/193's Findings for the hooks.json contract
// this mirrors (PreToolUse stdin/stdout schema, config file locations).
package agy

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ubunatic.com/harnez/internal/jsonc"
)

// HookName is the named hook entry harnez owns inside hooks.json. Only
// this entry is ever written or deleted; any other named hooks a user has
// hand-authored are preserved untouched, mirroring how internal/claude's
// settingsJSON merge only touches managedSettingsKeys.
const HookName = "harnez"

// HooksPath returns the global, shared agy hooks.json path used when no
// workspace-local override applies (~/.gemini/config/hooks.json per
// issue 193's Findings) — this mirrors `apply`'s global-only scope
// (docs/CLIDesign.md) rather than init's project-local one, since agy's
// hooks.json is itself a global/shared config file by default.
func HooksPath(home string) string {
	return filepath.Join(home, ".gemini", "config", "hooks.json")
}

// BuildHooksDoc returns the harnez-managed "harnez" named hook entry:
// a PreToolUse hook matching agy's "run_command" tool-step type, whose
// command handler is `harnez agy-hooks hook` (the PreToolUse handshake
// implemented in cmd/harnez/agyhooks.go), mirroring the shape of
// internal/claude's own PreToolUse/Bash wiring for Claude Code.
//
// Per agy's hook schema specification (agy-customizations/docs/hooks.md),
// named hooks are defined as top-level keys in hooks.json rather than
// nested under a redundant "hooks" root object.
func BuildHooksDoc() map[string]any {
	return map[string]any{
		HookName: map[string]any{
			"enabled": true,
			"PreToolUse": []map[string]any{
				{
					"matcher": "run_command",
					"hooks": []map[string]any{
						{"type": "command", "command": "harnez agy-hooks hook"},
					},
				},
			},
		},
	}
}

// extractHook extracts the HookName configuration from doc. It looks for
// doc[HookName] at the top level first. If the hook is only found nested
// under legacy doc["hooks"][HookName], or if both exist, legacy is returned
// as true so Status and mergeHooksDoc can flag or clean the drift.
func extractHook(doc map[string]any) (entry any, legacy bool, found bool) {
	legacyHook := false
	if hooks, ok := doc["hooks"].(map[string]any); ok {
		if _, exists := hooks[HookName]; exists {
			legacyHook = true
		}
	}
	if entry, ok := doc[HookName]; ok {
		return entry, legacyHook, true
	}
	if legacyHook {
		hooks := doc["hooks"].(map[string]any)
		return hooks[HookName], true, true
	}
	return nil, false, false
}

// mergeHooksDoc replaces HookName at the root level, cleans up any legacy
// HookName under "hooks", and leaves every other top-level key and other named
// hooks intact.
func mergeHooksDoc(existing, incoming map[string]any) map[string]any {
	out := make(map[string]any, len(existing)+1)
	for k, v := range existing {
		out[k] = v
	}
	// Clean up legacy "hooks" entry if it exists
	if existingHooks, ok := out["hooks"].(map[string]any); ok {
		mergedHooks := make(map[string]any, len(existingHooks))
		for k, v := range existingHooks {
			if k != HookName {
				mergedHooks[k] = v
			}
		}
		if len(mergedHooks) == 0 {
			delete(out, "hooks")
		} else {
			out["hooks"] = mergedHooks
		}
	}
	if hookEntry, ok := incoming[HookName]; ok {
		out[HookName] = hookEntry
	}
	return out
}

// Apply merges BuildHooksDoc into the hooks.json file at path, creating
// the file and its parent directory if absent, and reports whether the
// file's contents changed. Any hand-authored named hooks other than
// HookName, and any other top-level keys, are preserved verbatim.
func Apply(path string) (changed bool, err error) {
	existing := jsonc.Read(path)
	merged := mergeHooksDoc(existing, BuildHooksDoc())
	data := append(jsonc.MarshalPretty(merged), '\n')
	if err := jsonc.Validate(data); err != nil {
		return false, err
	}

	old, _ := os.ReadFile(path)
	if string(old) == string(data) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return false, err
	}
	return true, nil
}

// Status reports whether HookName is present in the hooks.json at path,
// and whether its content has drifted from what BuildHooksDoc would
// write (including legacy nested "hooks" wrapping) — the drift-detection
// surface issues/196's plan calls for alongside `harnez status`, unlike
// issue 195's unlintable PATH-shim.
func Status(path string) (installed bool, drifted bool) {
	existing := jsonc.Read(path)
	entry, legacy, found := extractHook(existing)
	if !found {
		return false, false
	}
	if legacy {
		return true, true
	}
	want := BuildHooksDoc()[HookName]
	wantData, _ := json.Marshal(want)
	gotData, _ := json.Marshal(entry)
	return true, string(wantData) != string(gotData)
}

// Remove deletes the HookName entry from hooks.json at path, cleaning up
// both top-level and legacy nested entries, leaving any other named hooks
// and top-level keys intact. If the file ends up with no remaining top-level
// keys, it is removed entirely (mirroring internal/claude's cleanSettingsJSON).
// A missing file or missing entry is a no-op, not an error.
func Remove(path string) (changed bool, err error) {
	existing := jsonc.Read(path)
	if len(existing) == 0 {
		return false, nil
	}

	found := false
	if _, ok := existing[HookName]; ok {
		delete(existing, HookName)
		found = true
	}
	if hooks, ok := existing["hooks"].(map[string]any); ok {
		if _, ok := hooks[HookName]; ok {
			delete(hooks, HookName)
			found = true
			if len(hooks) == 0 {
				delete(existing, "hooks")
			} else {
				existing["hooks"] = hooks
			}
		}
	}
	if !found {
		return false, nil
	}

	if len(existing) == 0 {
		return true, os.Remove(path)
	}
	data := append(jsonc.MarshalPretty(existing), '\n')
	return true, os.WriteFile(path, data, 0644)
}

// Delete is an alias for Remove.
func Delete(path string) (changed bool, err error) {
	return Remove(path)
}
