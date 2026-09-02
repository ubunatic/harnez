// Package codex manages harnez's integration with Codex CLI's native,
// stable, always-on hooks system — a TOML analog of internal/agy's
// hooks.json integration, and the native-hook counterpart to
// internal/claude's own PreToolUse/Bash wiring. See
// issues/199-research-codex-hook-surface-for-transparent-exec-distill.md's
// Findings for the config.toml [hooks.<name>] schema and PreToolUse
// stdin/stdout contract this mirrors, and
// issues/200-codex-native-hooks-preTooluse-wiring.md for the plan this
// implements.
package codex

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// HookName is the named hook entry harnez owns inside config.toml's
// [hooks.<name>] tables. Only this entry is ever written or deleted; any
// other named hook (hand-authored or plugin-installed) and any other
// top-level config.toml key are preserved untouched, mirroring
// internal/agy's mergeHooksDoc contract.
const HookName = "harnez"

// HooksPath returns the global Codex config.toml path (~/.codex/config.toml
// per issue 199's Findings — Codex has no standalone hooks.json at the
// user level; hooks live inside the main config file's [hooks.<name>]
// tables instead).
func HooksPath(home string) string {
	return filepath.Join(home, ".codex", "config.toml")
}

// BuildHooksDoc returns the harnez-managed "harnez" named hook entry: a
// PreToolUse hook matching Codex's "Bash" shell-tool matcher (per 199's
// Q2 — real installed plugin hooks.json files on the research machine use
// "Bash" as a matcher value, and Codex's PreToolUse schema is otherwise a
// near-exact mirror of Claude Code's own, which also matches "Bash"),
// whose command handler is `harnez codex-hook` (the hidden PreToolUse
// handshake command implemented in cmd/harnez/codexhooks.go; installation
// itself happens via `harnez apply`, not a separate management command).
//
// timeoutSec is intentionally omitted: issue 199's Q5 confirmed a
// per-hook timeoutSec field exists, but the real default numeric value
// was not recoverable from binary strings alone, and pinning it down
// would require either upstream Codex docs or a live hook-trust
// experiment out of scope for this ticket (see
// issues/200-codex-native-hooks-preTooluse-wiring.md Scope point 4).
// Omitting the field lets Codex apply its own built-in default rather
// than harnez guessing a value that could be wrong in either direction
// (too short: spurious "hook timed out" failures on slow harnez exec
// calls; too long: a stalled hook blocks the tool call longer than
// necessary).
func BuildHooksDoc() map[string]any {
	return map[string]any{
		"hooks": map[string]any{
			HookName: map[string]any{
				"enabled": true,
				"PreToolUse": []map[string]any{
					{
						"matcher": "Bash",
						"hooks": []map[string]any{
							{"type": "command", "command": "harnez codex-hook"},
						},
					},
				},
			},
		},
	}
}

// readTOML decodes the TOML document at path into a generic map, mirroring
// internal/jsonc.Read's silent-fallback-to-empty behavior on a missing or
// unparseable file — Apply/Status/Remove all treat "file absent" and "file
// present but broken" the same way (no entry found), never erroring.
func readTOML(path string) map[string]any {
	m := map[string]any{}
	data, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	if _, err := toml.Decode(string(data), &m); err != nil {
		return map[string]any{}
	}
	if m == nil {
		m = map[string]any{}
	}
	return m
}

// encodeTOML renders doc as TOML text via BurntSushi/toml, which emits
// proper [[array.of.tables]] header syntax for nested []map[string]any
// values (verified against issue 199's Q2 example config) rather than
// collapsing them into inline tables — important since the generated file
// must parse cleanly under `codex --strict-config doctor`.
func encodeTOML(doc map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// mergeHooksDoc replaces only the HookName entry under "hooks", leaving
// every other top-level key and every other named hook in existing intact.
// This is the TOML analog of internal/agy's mergeHooksDoc — same
// preserve-everything-else contract, different serialization.
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

// Apply merges BuildHooksDoc into the config.toml file at path, creating
// the file and its parent directory if absent, and reports whether the
// file's contents changed. Any hand-authored named hooks other than
// HookName, and any other top-level config.toml keys (model settings,
// MCP server entries, etc.), are preserved verbatim.
func Apply(path string) (changed bool, err error) {
	existing := readTOML(path)
	merged := mergeHooksDoc(existing, BuildHooksDoc())
	data, err := encodeTOML(merged)
	if err != nil {
		return false, err
	}

	old, _ := os.ReadFile(path)
	if bytes.Equal(old, data) {
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

// Status reports whether HookName is present in the config.toml at path,
// and whether its content has drifted from what BuildHooksDoc would
// write. Comparison goes through encoding/json rather than the raw TOML
// bytes so it's insensitive to key-ordering/formatting differences
// between what BurntSushi's decoder hands back and what BuildHooksDoc
// constructs directly.
func Status(path string) (installed bool, drifted bool) {
	existing := readTOML(path)
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

// Remove deletes the HookName entry from config.toml at path, leaving any
// other named hooks and top-level keys intact. If the file ends up with
// no remaining top-level keys, it is removed entirely (mirroring
// internal/agy's Remove/internal/claude's cleanSettingsJSON). A missing
// file or missing entry is a no-op, not an error.
func Remove(path string) (changed bool, err error) {
	existing := readTOML(path)
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
	data, err := encodeTOML(existing)
	if err != nil {
		return true, err
	}
	return true, os.WriteFile(path, data, 0644)
}
