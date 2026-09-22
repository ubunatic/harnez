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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// HookName is the legacy named hook entry ([hooks.harnez]) earlier harnez
// versions wrote; Apply and Remove delete it. Current hooks live in the
// per-event arrays ([[hooks.PreToolUse]] etc.), where harnez owns exactly
// the groups whose handlers all run `harnez ...` (see isHarnezGroup). User
// groups on the same events, other named hooks, and other top-level keys are
// preserved.
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
		"features": map[string]any{"hooks": true},
		"hooks": map[string]any{
			"PreToolUse": []map[string]any{
				{
					"matcher": "Bash",
					"hooks": []map[string]any{
						{"type": "command", "command": "harnez codex-hook"},
					},
				},
			},
			"PostToolUse": []map[string]any{
				{
					"matcher": "*",
					"hooks":   []map[string]any{{"type": "command", "command": "harnez codex-telemetry"}},
				},
			},
			"PreCompact": []map[string]any{{
				"hooks": []map[string]any{{"type": "command", "command": "harnez codex-telemetry"}},
			}},
			"PostCompact": []map[string]any{{
				"hooks": []map[string]any{{"type": "command", "command": "harnez codex-telemetry"}},
			}},
			"SessionStart": []map[string]any{{
				"matcher": "startup|resume|clear|compact",
				"hooks":   []map[string]any{{"type": "command", "command": "harnez codex-telemetry"}},
			}},
			"Stop": []map[string]any{{
				"hooks": []map[string]any{{"type": "command", "command": "harnez codex-telemetry"}},
			}},
			"SessionEnd": []map[string]any{{
				"matcher": "other",
				"hooks":   []map[string]any{{"type": "command", "command": "harnez codex-telemetry"}},
			}},
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

// mergeHooksDoc replaces harnez-owned groups in each event array that
// incoming defines and deletes the legacy HookName entry. User groups on
// the same events, every other named hook, and every other top-level key in
// existing stay intact. This is the TOML analog of internal/agy's
// mergeHooksDoc — same preserve-everything-else contract, different
// serialization.
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
		delete(mergedHooks, HookName)
		for event, v := range incomingHooks {
			groups := userGroups(mergedHooks[event])
			mergedHooks[event] = append(groups, tableArray(v)...)
		}
	}
	out["hooks"] = mergedHooks
	features := map[string]any{}
	if existingFeatures, ok := out["features"].(map[string]any); ok {
		for k, v := range existingFeatures {
			features[k] = v
		}
	}
	if incomingFeatures, ok := incoming["features"].(map[string]any); ok {
		for k, v := range incomingFeatures {
			features[k] = v
		}
	}
	out["features"] = features
	return out
}

// Apply merges BuildHooksDoc into the config.toml file at path, creating
// the file and its parent directory if absent, and reports whether the
// file's contents changed. User hook groups on the events harnez uses,
// hand-authored named hooks, and any other top-level config.toml keys
// (model settings, MCP server entries, etc.) are preserved.
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
	features, featuresOK := existing["features"].(map[string]any)
	if !featuresOK || features["hooks"] != true {
		return false, true
	}
	owned := harnezHooks(hooks)
	if len(owned) == 0 {
		return false, false
	}

	wantData, _ := json.Marshal(BuildHooksDoc()["hooks"])
	gotData, _ := json.Marshal(owned)
	return true, string(wantData) != string(gotData)
}

// Summary returns a compact operator-facing description of the managed Codex hooks.
func Summary(path string) string {
	doc := readTOML(path)
	features, _ := doc["features"].(map[string]any)
	hooks, _ := doc["hooks"].(map[string]any)
	entry := hooks
	state := "disabled"
	if features["hooks"] == true {
		state = "enabled"
	}
	return fmt.Sprintf("%s (PreToolUse %d, PostToolUse %d, PreCompact %d, PostCompact %d, SessionStart %d, Stop %d, SessionEnd %d)",
		state,
		hookCount(entry, "PreToolUse"),
		hookCount(entry, "PostToolUse"),
		hookCount(entry, "PreCompact"),
		hookCount(entry, "PostCompact"),
		hookCount(entry, "SessionStart"),
		hookCount(entry, "Stop"),
		hookCount(entry, "SessionEnd"),
	)
}

func hookCount(entry map[string]any, name string) int {
	groups, _ := entry[name].([]map[string]any)
	count := 0
	for _, group := range groups {
		if handlers, ok := group["hooks"].([]map[string]any); ok {
			count += len(handlers)
		}
	}
	return count
}

// Remove deletes harnez-owned groups from every event array and the legacy
// HookName entry, leaving user groups, other named hooks and top-level keys
// intact. features.hooks is dropped only when no hooks remain. If the file
// ends up with no remaining top-level keys, it is removed entirely
// (mirroring internal/agy's Remove/internal/claude's cleanSettingsJSON). A
// missing file or missing entry is a no-op, not an error.
func Remove(path string) (changed bool, err error) {
	existing := readTOML(path)
	hooks, ok := existing["hooks"].(map[string]any)
	if !ok {
		return false, nil
	}
	if _, ok := hooks[HookName]; ok {
		delete(hooks, HookName)
		changed = true
	}
	for event, v := range hooks {
		all := tableArray(v)
		if all == nil {
			continue
		}
		kept := userGroups(v)
		if len(kept) == len(all) {
			continue
		}
		changed = true
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}
	if !changed {
		return false, nil
	}
	if len(hooks) == 0 {
		delete(existing, "hooks")
		if features, ok := existing["features"].(map[string]any); ok {
			delete(features, "hooks")
			if len(features) == 0 {
				delete(existing, "features")
			}
		}
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

// tableArray normalizes a decoded TOML array of tables (BurntSushi returns
// []map[string]any) or a built one; any other value yields nil.
func tableArray(v any) []map[string]any {
	switch t := v.(type) {
	case []map[string]any:
		return t
	case []any:
		out := make([]map[string]any, 0, len(t))
		for _, e := range t {
			m, ok := e.(map[string]any)
			if !ok {
				return nil
			}
			out = append(out, m)
		}
		return out
	}
	return nil
}

// isHarnezGroup reports whether every handler in a hook group runs harnez.
func isHarnezGroup(group map[string]any) bool {
	handlers := tableArray(group["hooks"])
	if len(handlers) == 0 {
		return false
	}
	for _, h := range handlers {
		cmd, _ := h["command"].(string)
		cmd = strings.TrimSpace(cmd)
		if cmd != "harnez" && !strings.HasPrefix(cmd, "harnez ") {
			return false
		}
	}
	return true
}

// userGroups returns the groups of an event array that harnez does not own.
func userGroups(v any) []map[string]any {
	var out []map[string]any
	for _, g := range tableArray(v) {
		if !isHarnezGroup(g) {
			out = append(out, g)
		}
	}
	return out
}

// harnezHooks projects a hooks table onto the harnez-owned groups, dropping
// events without any, so it compares equal to BuildHooksDoc()["hooks"].
func harnezHooks(hooks map[string]any) map[string]any {
	out := map[string]any{}
	for event, v := range hooks {
		var owned []map[string]any
		for _, g := range tableArray(v) {
			if isHarnezGroup(g) {
				owned = append(owned, g)
			}
		}
		if len(owned) > 0 {
			out[event] = owned
		}
	}
	return out
}
