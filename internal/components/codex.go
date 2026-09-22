// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package components

import (
	"bytes"
	"os"

	"github.com/BurntSushi/toml"
)

// PruneCodexHooks removes only harnez-owned hook groups from the Codex
// config.toml at path: groups whose handlers all run `harnez ...`. Events
// left empty are dropped; user groups, other keys and `features.hooks` are
// kept. codex.Remove is not used because it deletes every event table it
// manages, including user hooks (it serves the full uninstall path).
// A missing or unparseable file is a no-op.
func PruneCodexHooks(path string) (changed bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, nil
	}
	doc := map[string]any{}
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return false, nil
	}
	hooks, ok := doc["hooks"].(map[string]any)
	if !ok {
		return false, nil
	}
	for event, v := range hooks {
		groups := tomlTables(v)
		if groups == nil {
			continue // named hook tables and scalars are not event lists
		}
		var kept []map[string]any
		for _, g := range groups {
			cmds := entryCommands(map[string]any{"hooks": toAny(tomlTables(g["hooks"]))})
			if len(cmds) > 0 && allHarnez(cmds) {
				changed = true
				continue
			}
			kept = append(kept, g)
		}
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
		delete(doc, "hooks")
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(doc); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, buf.Bytes(), 0644)
}

// tomlTables normalizes a decoded TOML array of tables, which BurntSushi
// returns as []map[string]any, to that type; other values yield nil.
func tomlTables(v any) []map[string]any {
	switch t := v.(type) {
	case []map[string]any:
		return t
	case []any:
		var out []map[string]any
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

func toAny(ms []map[string]any) []any {
	out := make([]any, len(ms))
	for i, m := range ms {
		out[i] = m
	}
	return out
}
