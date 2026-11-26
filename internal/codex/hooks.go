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
	"ubunatic.com/harnez/internal/hookdispatch"
)

// HookName is the legacy named hook entry ([hooks.harnez]) earlier harnez
// versions wrote; Apply and Remove delete it. Current hooks live in the
// per-event arrays ([[hooks.PreToolUse]] etc.), where harnez owns the handlers
// that run `harnez ...` (see isHarnezHandler). A group that also holds foreign
// handlers keeps them and its matcher. User groups on the same events, other
// named hooks, and other top-level keys are preserved.
const HookName = "harnez"

// HooksPath returns the global Codex config.toml path (~/.codex/config.toml
// per issue 199's Findings — Codex has no standalone hooks.json at the
// user level; hooks live inside the main config file's [hooks.<name>]
// tables instead).
func HooksPath(home string) string {
	return filepath.Join(home, ".codex", "config.toml")
}

// BuildHooksDoc returns the harnez-managed hook groups for config.toml: one
// group per event of the codex agent in spec/hooks.yaml, each running
// `harnez hook codex <Event>` (issue 1304 M3). PreToolUse runs codex-hook
// for the Bash matcher; the other events run codex-telemetry. The embedded
// spec is validated by tests, so a failure here is a build defect and panics.
//
// timeoutSec is intentionally omitted: Codex applies its own built-in default
// rather than harnez guessing a value that could be wrong in either direction
// (see issues/200-codex-native-hooks-preTooluse-wiring.md Scope point 4).
func BuildHooksDoc() map[string]any {
	manifest, err := hookdispatch.LoadManifest("codex")
	if err != nil {
		panic(err)
	}
	entries, err := manifest.Entries()
	if err != nil {
		panic(err)
	}
	hooks := map[string]any{}
	for _, e := range entries {
		group := map[string]any{
			"hooks": []map[string]any{{"type": "command", "command": e.Command}},
		}
		if e.Matcher != "" {
			group["matcher"] = e.Matcher
		}
		hooks[e.Event] = []map[string]any{group}
	}
	return map[string]any{
		"features": map[string]any{"hooks": true},
		"hooks":    hooks,
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

// mergeHooksDoc removes harnez-owned handlers from every event array of
// existing, appends the incoming groups, and deletes the legacy HookName
// entry. Foreign handlers, including those in a group that also holds harnez
// handlers, every other named hook, and every other top-level key stay intact. This is the TOML analog of internal/agy's
// mergeHooksDoc — same preserve-everything-else contract, different
// serialization.
func mergeHooksDoc(existing, incoming map[string]any) map[string]any {
	out := make(map[string]any, len(existing)+1)
	for k, v := range existing {
		out[k] = v
	}
	mergedHooks := map[string]any{}
	if existingHooks, ok := out["hooks"].(map[string]any); ok {
		// Strip harnez handlers from every event, not only the events incoming
		// defines, so a command harnez no longer writes cannot linger.
		for k, v := range existingHooks {
			if tableArray(v) == nil {
				mergedHooks[k] = v
				continue
			}
			if kept := userGroups(v); len(kept) > 0 {
				mergedHooks[k] = kept
			}
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
	owned, mixed := harnezHooks(hooks)
	if len(owned) == 0 {
		return false, false
	}
	if mixed {
		// A harnez handler shares a group with foreign handlers; Apply would split it.
		return true, true
	}

	wantData, _ := json.Marshal(BuildHooksDoc()["hooks"])
	gotData, _ := json.Marshal(owned)
	return true, string(wantData) != string(gotData)
}

// TrustStatus reports whether every harnez-owned hook handler at path has a
// trusted_hash recorded under [hooks.state] (issue 724).
func TrustStatus(path string) (trusted bool, untrusted int, total int) {
	doc := readTOML(path)
	hooks, ok := doc["hooks"].(map[string]any)
	if !ok {
		return false, 0, 0
	}
	state, _ := hooks["state"].(map[string]any)
	for event, v := range hooks {
		if event == "state" || event == HookName {
			continue
		}
		snake := codexHookEventKey(event)
		for gi, group := range tableArray(v) {
			for hi, h := range tableArray(group["hooks"]) {
				if !isHarnezHandler(h) {
					continue
				}
				total++
				if !hasHookTrustHash(state, path, snake, gi, hi) {
					untrusted++
				}
			}
		}
	}
	return total > 0 && untrusted == 0, untrusted, total
}

func codexHookEventKey(event string) string {
	var b strings.Builder
	for i, r := range event {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteByte(byte(r + ('a' - 'A')))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func hasHookTrustHash(state map[string]any, path, snakeEvent string, groupIdx, hookIdx int) bool {
	if len(state) == 0 {
		return false
	}
	suffix := fmt.Sprintf(":%s:%d:%d", snakeEvent, groupIdx, hookIdx)
	if entry, ok := state[path+suffix].(map[string]any); ok {
		if h, _ := entry["trusted_hash"].(string); strings.TrimSpace(h) != "" {
			return true
		}
	}
	for k, raw := range state {
		if !strings.HasSuffix(k, suffix) {
			continue
		}
		if entry, ok := raw.(map[string]any); ok {
			if h, _ := entry["trusted_hash"].(string); strings.TrimSpace(h) != "" {
				return true
			}
		}
	}
	return false
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
		if all == nil || ownedCount(all) == 0 {
			continue
		}
		changed = true
		kept := userGroups(v)
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

// isHarnezHandler reports whether one hook handler runs harnez, by the rule
// shared with apply's Claude side (hookdispatch.RunsHarnez). That covers the
// bare legacy `harnez codex-hook`, absolute-path and env-prefixed forms.
func isHarnezHandler(h map[string]any) bool {
	cmd, _ := h["command"].(string)
	return hookdispatch.RunsHarnez(cmd)
}

// splitGroup separates a hook group's handlers into the harnez-owned ones and
// the foreign ones, keeping their order.
func splitGroup(group map[string]any) (owned, kept []map[string]any) {
	for _, h := range tableArray(group["hooks"]) {
		if isHarnezHandler(h) {
			owned = append(owned, h)
		} else {
			kept = append(kept, h)
		}
	}
	return owned, kept
}

// withHandlers returns a copy of group with its handlers replaced, so the
// group's matcher and any other keys stay as they were.
func withHandlers(group map[string]any, handlers []map[string]any) map[string]any {
	out := make(map[string]any, len(group))
	for k, v := range group {
		out[k] = v
	}
	out["hooks"] = handlers
	return out
}

// ownedCount counts the harnez-owned handlers across groups.
func ownedCount(groups []map[string]any) int {
	n := 0
	for _, g := range groups {
		owned, _ := splitGroup(g)
		n += len(owned)
	}
	return n
}

// userGroups returns the groups of an event array without the harnez handlers.
// A group that held only harnez handlers is dropped. A mixed group keeps its
// foreign handlers and its matcher, so the user's handlers are never lost.
func userGroups(v any) []map[string]any {
	var out []map[string]any
	for _, g := range tableArray(v) {
		owned, kept := splitGroup(g)
		switch {
		case len(owned) == 0:
			out = append(out, g)
		case len(kept) > 0:
			out = append(out, withHandlers(g, kept))
		}
	}
	return out
}

// harnezHooks projects a hooks table onto the harnez-owned handlers: per event,
// each group that holds a harnez handler, reduced to those handlers and the
// group's matcher. Events without any are dropped, so the projection compares
// equal to BuildHooksDoc()["hooks"]. mixed reports a group that also holds
// foreign handlers, which Apply would split.
func harnezHooks(hooks map[string]any) (owned map[string]any, mixed bool) {
	owned = map[string]any{}
	for event, v := range hooks {
		var groups []map[string]any
		for _, g := range tableArray(v) {
			ownedHandlers, kept := splitGroup(g)
			if len(ownedHandlers) == 0 {
				continue
			}
			if len(kept) > 0 {
				mixed = true
			}
			groups = append(groups, withHandlers(g, ownedHandlers))
		}
		if len(groups) > 0 {
			owned[event] = groups
		}
	}
	return owned, mixed
}
