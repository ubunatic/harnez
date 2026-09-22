// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"fmt"
	"slices"
	"strings"
)

// Component names one separable part of what `harnez apply` installs
// (issue 490, docs/HarnezComponents.md §8; moved from internal/components
// into internal/claude, issue 491 M6).
type Component string

const (
	Docs      Component = "docs"      // global copyable docs
	Skills    Component = "skills"    // commands and skills
	Telemetry Component = "telemetry" // harnez hooks (Claude, Codex, AGY), distill adapters, rate protocol, telemetry store
	Agents    Component = "agents"    // skills that require `harnez agent`
	Usage     Component = "usage"     // status line and collector unit (transitional, moving to loom)
)

// All lists every component in canonical order.
var All = []Component{Docs, Skills, Telemetry, Agents, Usage}

// presets are the named selections from issue 490.
var presets = map[string][]Component{
	"full":           All,
	"docs-only":      {Docs, Skills},
	"telemetry-only": {Telemetry},
	"agents-only":    {Agents},
}

// PresetNames lists the preset names in a stable order.
func PresetNames() []string {
	return []string{"full", "docs-only", "telemetry-only", "agents-only"}
}

// Set is a resolved component selection. A nil Set means no selection was
// made and enables everything, so a config without `components:` behaves as
// today (nil = full).
type Set map[Component]bool

// Has reports whether c is enabled.
func (s Set) Has(c Component) bool {
	return s == nil || s[c]
}

// HasComponent is Has by string name, for call sites (skillDisabled,
// buildSettingsDoc) that only have a `requires:` name, not a typed Component.
func (s Set) HasComponent(name string) bool {
	return s.Has(Component(name))
}

// Allows reports whether every required component is enabled.
func (s Set) Allows(requires []Component) bool {
	for _, r := range requires {
		if !s.Has(r) {
			return false
		}
	}
	return true
}

// String renders the set in canonical order; "full" when unfiltered.
func (s Set) String() string {
	if s == nil {
		return "full"
	}
	var names []string
	for _, c := range All {
		if s[c] {
			names = append(names, string(c))
		}
	}
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ",")
}

// Parse resolves preset and component names (mixable, comma-separated or
// repeated) into a Set. Empty input returns nil (unfiltered).
func Parse(names ...string) (Set, error) {
	var parts []string
	for _, n := range names {
		for p := range strings.SplitSeq(n, ",") {
			if p = strings.TrimSpace(p); p != "" {
				parts = append(parts, p)
			}
		}
	}
	if len(parts) == 0 {
		return nil, nil
	}
	set := Set{}
	for _, p := range parts {
		if preset, ok := presets[p]; ok {
			for _, c := range preset {
				set[c] = true
			}
			continue
		}
		if !slices.Contains(All, Component(p)) {
			return nil, fmt.Errorf("unknown component %q (components: %s; presets: %s)",
				p, joinComponents(All), strings.Join(PresetNames(), ", "))
		}
		set[Component(p)] = true
	}
	return set, nil
}

func joinComponents(cs []Component) string {
	names := make([]string, len(cs))
	for i, c := range cs {
		names[i] = string(c)
	}
	return strings.Join(names, ", ")
}

// Resolve applies the precedence from §8.5: flag > config > local config.
// Each argument is a list of names; the first non-empty one wins.
func Resolve(flag, config, local []string) (Set, error) {
	for _, names := range [][]string{flag, config, local} {
		if len(names) > 0 {
			return Parse(names...)
		}
	}
	return nil, nil
}

// ValidateConfig rejects component names that cannot be resolved by apply.
func ValidateConfig(cfg *Config) error {
	if _, err := Parse(cfg.ComponentNames...); err != nil {
		return fmt.Errorf("config components: %w", err)
	}
	for _, skill := range cfg.Skills {
		for _, name := range skill.Requires {
			if !slices.Contains(All, Component(name)) {
				return fmt.Errorf("skill %q requires unknown component %q", skill.Name, name)
			}
		}
	}
	return nil
}
