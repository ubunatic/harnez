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
func BuildHooksDoc() map[string]any {
	return map[string]any{
		"hooks": map[string]any{
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
		},
	}
}

// mergeHooksDoc replaces only the HookName entry under "hooks", leaving
// every other top-level key and every other named hook in existing intact.
func mergeHooksDoc(existing, incoming map[string]any) map[string]any {
	out := make(map[string]any, len(existing)+1)
	for k, v := range existing {
		out[k] = v
	}
	mergedHooks := map[string]any{}
	if existingHooks, ok := out["hooks"].(map[string]any); ok {
		for k, v := range existingHooks {
			mergedHooks[k] = v
		}
	}
	if incomingHooks, ok := incoming["hooks"].(map[string]any); ok {
		for k, v := range incomingHooks {
			mergedHooks[k] = v
		}
	}
	out["hooks"] = mergedHooks
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
// write — the drift-detection surface issues/196's plan calls for
// alongside `harnez status`, unlike issue 195's unlintable PATH-shim.
func Status(path string) (installed bool, drifted bool) {
	existing := jsonc.Read(path)
	hooks, ok := existing["hooks"].(map[string]any)
	if !ok {
		return false, false
	}
	entry, ok := hooks[HookName]
	if !ok {
		return false, false
	}

	want := BuildHooksDoc()["hooks"].(map[string]any)[HookName]
	wantData, _ := json.Marshal(want)
	gotData, _ := json.Marshal(entry)
	return true, string(wantData) != string(gotData)
}

// Remove deletes the HookName entry from hooks.json at path, leaving any
// other named hooks and top-level keys intact. If the file ends up with
// no remaining top-level keys, it is removed entirely (mirroring
// internal/claude's cleanSettingsJSON). A missing file or missing entry
// is a no-op, not an error.
func Remove(path string) (changed bool, err error) {
	existing := jsonc.Read(path)
	hooks, ok := existing["hooks"].(map[string]any)
	if !ok {
		return false, nil
	}
	if _, ok := hooks[HookName]; !ok {
		return false, nil
	}
	delete(hooks, HookName)
	if len(hooks) == 0 {
		delete(existing, "hooks")
	} else {
		existing["hooks"] = hooks
	}

	if len(existing) == 0 {
		return true, os.Remove(path)
	}
	data := append(jsonc.MarshalPretty(existing), '\n')
	return true, os.WriteFile(path, data, 0644)
}
