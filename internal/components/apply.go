// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package components

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"ubunatic.com/harnez/internal/agy"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/codex"
	"ubunatic.com/harnez/internal/fsutil"
	"ubunatic.com/harnez/internal/jsonc"
)

// Filter returns a copy of cfg narrowed to what set enables. Disabled docs,
// commands and skills are dropped (skip semantics: apply stops managing
// them). Harness entry points of disabled components are dropped here too;
// Apply's removal pass then deletes their installed copies.
func Filter(cfg *claude.Config, set Set) *claude.Config {
	out := *cfg
	if set == nil {
		return &out
	}
	if !set.Has(Docs) {
		out.Docs = nil
	}
	if !set.Has(Skills) {
		out.Commands = nil
		out.Skills = nil
	} else {
		// Rate-feedback skills stay: DisableRateProtocol (below) makes apply
		// remove them itself, resources included.
		out.Skills = slices.DeleteFunc(slices.Clone(cfg.Skills), func(s claude.Command) bool {
			return !s.RateFeedback && !set.Allows(SkillRequires[s.Name])
		})
	}
	if !set.Has(Telemetry) {
		out.Hooks = slices.DeleteFunc(slices.Clone(cfg.Hooks), func(h claude.Hook) bool {
			return IsHarnezCommand(h.Command)
		})
		out.Feedback.DisableRateProtocol = true
		out.DistillAutopipe = claude.DistillAutopipe{}
		out.CodexHooksTarget = ""
		out.AgyHooksTarget = ""
	}
	if !set.Has(Usage) {
		out.StatusLine = false
	}
	return &out
}

// Options mirrors the apply flags that ApplyAllVariant takes.
type Options struct {
	Docs           []string
	ForceDocs      bool
	InstallSystemd bool
	DocVariant     string
	InstallShell   bool
}

// Apply runs `harnez apply` for the selection: filtered install, then the
// removal pass (§8.4). It returns the paths or entries it removed.
func Apply(target string, cfg *claude.Config, set Set, opts Options) ([]string, error) {
	docs := opts.Docs
	if !set.Has(Docs) {
		docs = nil
	}
	variant := opts.DocVariant
	if variant == "" {
		variant = "full"
	}
	filtered := Filter(cfg, set)
	if err := claude.ApplyAllVariant(target, filtered, docs, opts.ForceDocs,
		opts.InstallSystemd && set.Has(Usage), variant, opts.InstallShell); err != nil {
		return nil, err
	}
	if set == nil {
		return nil, nil
	}
	return removeDisabled(target, cfg, set)
}

// removeDisabled deletes installed harness entry points of disabled
// components. It uses the unfiltered cfg to know where they were installed.
func removeDisabled(target string, cfg *claude.Config, set Set) ([]string, error) {
	var removed []string

	settingsPath := filepath.Join(target, "settings.json")
	if _, err := os.Stat(settingsPath); err == nil {
		existing := jsonc.Read(settingsPath)
		pruned, notes := PruneSettings(existing, set)
		if len(notes) > 0 {
			data := append(jsonc.MarshalPretty(pruned), '\n')
			if err := os.WriteFile(settingsPath, data, 0644); err != nil {
				return removed, fmt.Errorf("settings: %w", err)
			}
			for _, n := range notes {
				removed = append(removed, settingsPath+": "+n)
			}
		}
	}

	if !set.Has(Telemetry) {
		if cfg.CodexHooksTarget != "" {
			p := fsutil.ExpandHome(cfg.CodexHooksTarget)
			changed, err := codex.Remove(p)
			if err != nil {
				return removed, fmt.Errorf("codex hooks: %w", err)
			}
			if changed {
				removed = append(removed, p+": harnez hook")
			}
		}
		if cfg.AgyHooksTarget != "" {
			p := fsutil.ExpandHome(cfg.AgyHooksTarget)
			changed, err := agy.Remove(p)
			if err != nil {
				return removed, fmt.Errorf("agy hooks: %w", err)
			}
			if changed {
				removed = append(removed, p+": harnez hook")
			}
		}
		// Distill adapters are harnez-generated files that route tool
		// output through harnez; remove them like hooks.
		for _, target := range []string{cfg.DistillAutopipe.PiExtensionTarget, cfg.DistillAutopipe.OpenCodePluginTarget} {
			p := fsutil.ExpandHome(target)
			if p == "" {
				continue
			}
			if err := os.Remove(p); err == nil {
				removed = append(removed, p)
			} else if !os.IsNotExist(err) {
				return removed, fmt.Errorf("distill adapter: %w", err)
			}
		}
	}

	for _, skill := range cfg.Skills {
		if set.Allows(SkillRequires[skill.Name]) {
			continue
		}
		for _, root := range SkillTargets(cfg) {
			dir := filepath.Join(root, skill.Name)
			path := filepath.Join(dir, "SKILL.md")
			if err := os.Remove(path); err == nil {
				removed = append(removed, path)
			} else if !os.IsNotExist(err) {
				return removed, fmt.Errorf("skill %s: %w", skill.Name, err)
			}
			_ = os.Remove(dir) // best effort: only succeeds when empty
		}
	}
	return removed, nil
}

// PruneSettings removes harnez-owned hooks (telemetry off) and a harnez-owned
// status line (usage off) from a settings.json document. Hook entries whose
// commands are not all harnez-owned are kept. It returns the new document and
// one note per removal; the input is not modified.
func PruneSettings(doc map[string]any, set Set) (map[string]any, []string) {
	out := make(map[string]any, len(doc))
	for k, v := range doc {
		out[k] = v
	}
	var notes []string

	if !set.Has(Usage) {
		if sl, ok := out["statusLine"].(map[string]any); ok {
			if cmd, _ := sl["command"].(string); IsHarnezCommand(cmd) {
				delete(out, "statusLine")
				notes = append(notes, "statusLine "+cmd)
			}
		}
	}

	if !set.Has(Telemetry) {
		if hooks, ok := out["hooks"].(map[string]any); ok {
			newHooks := map[string]any{}
			for event, v := range hooks {
				entries, _ := v.([]any)
				var kept []any
				for _, e := range entries {
					if cmds := entryCommands(e); len(cmds) > 0 && allHarnez(cmds) {
						notes = append(notes, fmt.Sprintf("hook %s %s", event, cmds[0]))
						continue
					}
					kept = append(kept, e)
				}
				if len(kept) > 0 {
					newHooks[event] = kept
				}
			}
			if len(newHooks) == 0 {
				delete(out, "hooks")
			} else {
				out["hooks"] = newHooks
			}
		}
	}
	return out, notes
}

func entryCommands(entry any) []string {
	m, _ := entry.(map[string]any)
	list, _ := m["hooks"].([]any)
	var cmds []string
	for _, h := range list {
		hm, _ := h.(map[string]any)
		if cmd, ok := hm["command"].(string); ok {
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

func allHarnez(cmds []string) bool {
	for _, c := range cmds {
		if !IsHarnezCommand(c) {
			return false
		}
	}
	return true
}

// SkillTargets mirrors claude's unexported skillTargets for the removal pass.
func SkillTargets(cfg *claude.Config) []string {
	home, _ := os.UserHomeDir()
	orHome := func(configured string, fallback ...string) string {
		if p := fsutil.ExpandHome(configured); p != "" {
			return p
		}
		if home == "" {
			return ""
		}
		return filepath.Join(append([]string{home}, fallback...)...)
	}
	candidates := []string{
		orHome(cfg.SkillsTarget, ".gemini", "skills"),
		fsutil.ExpandHome(cfg.CodexSkillsTarget),
		orHome(cfg.ClaudeSkillsTarget, ".claude", "skills"),
	}
	if prime := fsutil.ExpandHome(cfg.PrimeAgentTarget); prime != "" {
		candidates = append(candidates, filepath.Join(prime, "skills"))
	}
	var out []string
	for _, p := range candidates {
		if p != "" && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}
