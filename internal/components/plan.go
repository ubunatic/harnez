// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package components

import (
	"fmt"
	"strings"

	"ubunatic.com/harnez/internal/claude"
)

// Step is one apply phase (docs/HarnezComponents.md §8.7) as the selection
// sees it. Action is "install", "remove", "skip" or "always".
type Step struct {
	Phase     string
	Component Component // empty for always-on phases
	Action    string
	Detail    string
}

// Plan reports, without touching the filesystem, what Apply does for set.
func Plan(cfg *claude.Config, set Set) []Step {
	f := Filter(cfg, set)
	act := func(c Component) string {
		if set.Has(c) {
			return "install"
		}
		return "skip"
	}
	remove := func(c Component) string {
		if set.Has(c) {
			return "install"
		}
		return "remove"
	}

	var harnezHooks, otherHooks []string
	for _, h := range cfg.Hooks {
		name := h.Event
		if h.Matcher != "" {
			name += "/" + h.Matcher
		}
		if IsHarnezCommand(h.Command) {
			harnezHooks = append(harnezHooks, name)
		} else {
			otherHooks = append(otherHooks, name)
		}
	}
	var removedSkills []string
	for _, s := range cfg.Skills {
		if !set.Allows(SkillRequires[s.Name]) || (s.RateFeedback && f.Feedback.DisableRateProtocol) {
			removedSkills = append(removedSkills, s.Name)
		}
	}

	steps := []Step{
		{"telemetry schema init", Telemetry, act(Telemetry), "~/.harnez/tool_catalog.sqlite"},
		{"decommissioned cleanup", "", "always", ""},
		{"settings: model, permissions, env, verbs, mcp", "", "always", ""},
		{"settings: harnez hooks", Telemetry, remove(Telemetry), strings.Join(harnezHooks, ", ")},
		{"settings: other hooks", "", "always", strings.Join(otherHooks, ", ")},
		{"settings: statusLine", Usage, remove(Usage), "harnez statusline"},
		{"commands", Skills, act(Skills), fmt.Sprintf("%d configured", len(cfg.Commands))},
		{"skills", Skills, act(Skills), skillsDetail(cfg, f, set)},
	}
	if len(removedSkills) > 0 {
		steps = append(steps, Step{"skills: unmet requires", "", "remove", strings.Join(removedSkills, ", ")})
	}
	steps = append(steps,
		Step{"codex hooks", Telemetry, remove(Telemetry), cfg.CodexHooksTarget},
		Step{"agy hooks", Telemetry, remove(Telemetry), cfg.AgyHooksTarget},
		Step{"distill adapters", Telemetry, remove(Telemetry), ""},
		Step{"global docs", Docs, act(Docs), strings.Join(cfg.Docs, ", ")},
		Step{"shims, shell rc", "", "always", ""},
		Step{"systemd collector unit", Usage, act(Usage), "only with --systemd"},
	)
	return steps
}

// skillsDetail counts the skills apply installs, or, when the skills
// component is off, the configured skills apply leaves untouched.
func skillsDetail(cfg, f *claude.Config, set Set) string {
	if !set.Has(Skills) {
		return fmt.Sprintf("%d configured", len(cfg.Skills))
	}
	n := 0
	for _, s := range f.Skills {
		if !(s.RateFeedback && f.Feedback.DisableRateProtocol) {
			n++
		}
	}
	return fmt.Sprintf("%d", n)
}
