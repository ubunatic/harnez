package claude

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/agy"
	"ubunatic.com/harnez/internal/codex"
	"ubunatic.com/harnez/internal/fsutil"
)

// applyReport accumulates the change count and the per-phase summary lines
// that ApplyAllVariant prints at the end of an apply run.
type applyReport struct {
	changes int
	stats   []applyStat
}

type applyStat struct{ label, detail string }

// addStat records a summary line; an empty detail adds nothing.
func (r *applyReport) addStat(label, detail string) {
	if detail != "" {
		r.stats = append(r.stats, applyStat{label, detail})
	}
}

// applyCommands installs configured slash commands into every command target.
func applyCommands(r *applyReport, cfg *Config, selection Set, target string) error {
	// issue 491 M6: with cfg unfiltered, the "skills" component itself (as
	// opposed to a skill's own `requires:`) only gates installing new
	// content — skip semantics, matching a disabled skill's own requires
	// check below, which still actively removes it regardless of this gate.
	if len(cfg.Commands) > 0 && selection.HasComponent("skills") {
		cmdDirs := commandTargets(target, cfg)
		for _, cmdDir := range cmdDirs {
			if err := os.MkdirAll(cmdDir, 0755); err != nil {
				return fmt.Errorf("commands dir %s: %w", cmdDir, err)
			}
		}
		var cmdNames []string
		for _, cmd := range cfg.Commands {
			content, err := genCommandContent(cmd, cfg.FS)
			if err != nil {
				return err
			}
			for _, cmdDir := range cmdDirs {
				path := filepath.Join(cmdDir, cmd.Name+".md")
				cr, err := writeFileIfChanged(path, []byte(content))
				if err != nil {
					return fmt.Errorf("command %s: %w", cmd.Name, err)
				}
				if cr.changed {
					r.changes++
				}
				printResult("wrote", path, cr)
			}
			cmdNames = append(cmdNames, cmd.Name)
		}
		r.addStat("commands", strings.Join(cmdNames, ", "))
	}
	return nil
}

// applySkills installs, updates, and removes configured skills in every skill target.
func applySkills(r *applyReport, cfg *Config, selection Set, disableRateFeedback bool) error {
	if len(cfg.Skills) > 0 {
		targets := skillTargets(cfg)
		var skillNames []string
		for _, skill := range cfg.Skills {
			// issue 142/491: a skill that is rate_feedback-gated and disabled,
			// or whose `requires:` component isn't selected, is actively
			// removed (not merely skipped) — same rationale as the
			// section-clean branch above.
			if skillDisabled(skill, selection, disableRateFeedback) {
				for _, skillsRoot := range targets {
					skillDir := filepath.Join(skillsRoot, skill.Name)
					if externalSkillClash(skillDir) || !isManagedSkill(skillDir) {
						continue
					}
					path, err := safeSkillPath(skillDir, "SKILL.md")
					if err != nil {
						return err
					}
					if _, err := os.Stat(path); err == nil {
						if err := os.Remove(path); err != nil {
							return fmt.Errorf("skill %s [%s]: %w", skill.Name, path, err)
						}
						r.changes++
						fmt.Printf("  removed %s\n", path)
						_ = os.Remove(skillDir) // best-effort: drop now-empty dir
					} else if !os.IsNotExist(err) {
						return fmt.Errorf("skill %s [%s]: %w", skill.Name, path, err)
					}
					resourceTargets, err := validateSkillResourceTargets(skill)
					if err != nil {
						return err
					}
					for _, resourceTarget := range resourceTargets {
						resourcePath, err := safeSkillPath(skillDir, resourceTarget)
						if err != nil {
							return err
						}
						if err := os.Remove(resourcePath); err != nil && !os.IsNotExist(err) {
							return fmt.Errorf("skill %s resource [%s]: %w", skill.Name, resourcePath, err)
						}
						_ = os.Remove(filepath.Dir(resourcePath))
					}
					markerPath := filepath.Join(skillDir, ManagedSkillMarker)
					if err := os.Remove(markerPath); err == nil {
						r.changes++
					} else if !os.IsNotExist(err) {
						return fmt.Errorf("skill %s marker [%s]: %w", skill.Name, markerPath, err)
					}
					_ = os.Remove(skillDir)
				}
				skillNames = append(skillNames, skill.Name+" (disabled)")
				continue
			}
			if !selection.HasComponent("skills") {
				// skip semantics: skills component off, but this skill's own
				// requires: (if any) are met, so an existing install is left
				// untouched rather than updated or removed.
				continue
			}
			content, err := genSkillContent(skill, cfg.FS)
			if err != nil {
				return err
			}
			resources, err := genSkillResources(skill, cfg.FS)
			if err != nil {
				return err
			}
			for _, skillsRoot := range targets {
				skillDir := filepath.Join(skillsRoot, skill.Name)
				if externalSkillClash(skillDir) {
					continue
				}
				if _, err := os.Lstat(skillDir); err == nil && !isManagedSkill(skillDir) {
					continue // never overwrite a skill directory harnez does not own
				} else if err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("skill %s dir: %w", skill.Name, err)
				}
				path, err := safeSkillPath(skillDir, "SKILL.md")
				if err != nil {
					return err
				}
				if err := os.MkdirAll(skillDir, 0755); err != nil {
					return fmt.Errorf("skill %s dir: %w", skill.Name, err)
				}
				markerPath := filepath.Join(skillDir, ManagedSkillMarker)
				markerResult, err := writeFileIfChanged(markerPath, []byte("managed: config\nskill: "+skill.Name+"\n"))
				if err != nil {
					return fmt.Errorf("skill %s marker: %w", skill.Name, err)
				}
				if markerResult.changed {
					r.changes++
				}
				oldContent, _ := os.ReadFile(path)
				cr := applyResult{changed: string(oldContent) != content}
				if cr.changed {
					if err := os.WriteFile(path, []byte(content), 0644); err != nil {
						return fmt.Errorf("skill %s: %w", skill.Name, err)
					}
					r.changes++
				}
				printResult("wrote", path, cr)
				for _, resource := range resources {
					resourcePath, err := safeSkillPath(skillDir, resource.target)
					if err != nil {
						return err
					}
					if err := os.MkdirAll(filepath.Dir(resourcePath), 0755); err != nil {
						return fmt.Errorf("skill %s resource dir: %w", skill.Name, err)
					}
					rr, err := writeFileIfChanged(resourcePath, resource.data)
					if err != nil {
						return fmt.Errorf("skill %s resource: %w", skill.Name, err)
					}
					if rr.changed {
						r.changes++
					}
					printResult("wrote", resourcePath, rr)
				}
			}
			skillNames = append(skillNames, skill.Name)
		}
		if selection.HasComponent("skills") {
			n, bundled, err := syncBundledSkills(cfg)
			if err != nil {
				return err
			}
			r.changes += n
			for _, b := range bundled {
				skillNames = append(skillNames, b+" (bundled)")
			}
		}
		r.addStat("skills", strings.Join(skillNames, ", "))
	}
	return nil
}

// applyCodexHooks merges or removes harnez-owned Codex hooks in config.toml.
func applyCodexHooks(r *applyReport, cfg *Config, selection Set) error {
	// Codex's own native hooks.json-equivalent (config.toml's [hooks.<name>]
	// tables) is a distinct config format from Claude's settings.json, so it
	// isn't folded into buildSettingsDoc/applySettingsJSON above — it gets
	// its own merge (internal/codex.Apply, preserving unrelated hooks/keys
	// the same way applySettingsJSON preserves unmanaged settings keys). See
	// issues/199 (research) and issues/200 (this wiring, originally a
	// standalone `harnez codex-hooks apply` command, folded into `apply`
	// per user direction rather than staying a separate command).
	// issue 491 M5: Codex/AGY hooks are installed only under the telemetry
	// component; without it they're actively removed (ownership-aware,
	// 02c9a0b), not merely skipped, so a selected apply that drops telemetry
	// leaves no dangling harnez-owned hook behind.
	if cfg.CodexHooksTarget != "" {
		hooksPath := fsutil.ExpandHome(cfg.CodexHooksTarget)
		if selection.HasComponent("telemetry") {
			changed, err := codex.Apply(hooksPath)
			if err != nil {
				return fmt.Errorf("codex hooks: %w", err)
			}
			if changed {
				r.changes++
				fmt.Printf("  wrote %s\n", hooksPath)
				fmt.Println("  note: Codex will prompt for hook-trust review before this hook becomes active (see its /hooks panel).")
			}
			r.addStat("codex hooks", codex.Summary(hooksPath))
		} else {
			changed, err := codex.Remove(hooksPath)
			if err != nil {
				return fmt.Errorf("codex hooks: %w", err)
			}
			if changed {
				r.changes++
				fmt.Printf("  removed %s\n", hooksPath)
			}
		}
	}
	return nil
}

// applyAgyHooks installs or removes the Antigravity tool observation hook.
func applyAgyHooks(r *applyReport, cfg *Config, selection Set) error {
	// Antigravity native tool observation hook (issue 373):
	// Install/update wildcard PreToolUse hook in ~/.gemini/config/hooks.json if ~/.gemini exists.
	if cfg.AgyHooksTarget != "" {
		hooksPath := fsutil.ExpandHome(cfg.AgyHooksTarget)
		agentHome := filepath.Dir(filepath.Dir(hooksPath))
		if _, err := os.Stat(agentHome); err == nil {
			if selection.HasComponent("telemetry") {
				changed, err := agy.Apply(hooksPath)
				if err != nil {
					return fmt.Errorf("agy hooks: %w", err)
				}
				if changed {
					r.changes++
					fmt.Printf("  wrote %s\n", hooksPath)
				}
				r.addStat("agy hooks", agy.Summary(hooksPath))
			} else {
				changed, err := agy.Remove(hooksPath)
				if err != nil {
					return fmt.Errorf("agy hooks: %w", err)
				}
				if changed {
					r.changes++
					fmt.Printf("  removed %s\n", hooksPath)
				}
			}
		}
	}
	return nil
}

// applyDistillAdapters installs or removes distill adapters (telemetry component).
func applyDistillAdapters(r *applyReport, cfg *Config, selection Set) error {
	// issue 491 M6: like Codex/AGY hooks above, distill adapters are only
	// installed under the telemetry component; without it they're actively
	// removed, since cfg is unfiltered here (selection decides, not Filter).
	if adapters := distillAdapters(cfg); len(adapters) > 0 {
		if selection.HasComponent("telemetry") {
			for _, adapter := range adapters {
				ar, err := writeFileIfChanged(adapter.path, []byte(adapter.content))
				if err != nil {
					return fmt.Errorf("%s %s: %w", adapter.label, adapter.path, err)
				}
				if ar.changed {
					r.changes++
				}
				printResult("wrote", adapter.path, ar)
			}
			r.addStat("distill", distillAdapterSummary(cfg))
		} else {
			for _, adapter := range adapters {
				if err := os.Remove(adapter.path); err == nil {
					r.changes++
					fmt.Printf("  removed %s\n", adapter.path)
				} else if !os.IsNotExist(err) {
					return fmt.Errorf("%s %s: %w", adapter.label, adapter.path, err)
				}
			}
		}
	}
	return nil
}

// applyGlobalDocs installs the selected global docs.
func applyGlobalDocs(r *applyReport, cfg *Config, docVariant string, docs []string, forceDocs bool, selection Set, target string) error {
	docNames := docs
	configDocs := cfg.Docs
	if !selection.HasComponent("docs") {
		docNames = nil
		configDocs = nil
	}
	globalDocs := mergeDocs(configDocs, docNames)
	for _, name := range globalDocs {
		lang, ok := cfg.AgentsMD.Languages[name]
		if !ok {
			return fmt.Errorf("unknown doc: %s", name)
		}
		for _, dst := range docTargets(target, lang, cfg) {
			fr, err := installDoc(cfg.FS, lang.SourceFor(docVariant), dst, forceDocs)
			if err != nil {
				return fmt.Errorf("language %s: install %s: %w", name, dst, err)
			}
			if fr.changed {
				r.changes++
				fmt.Printf("  installed %s\n", dst)
			} else {
				state := langDocState(cfg.FS, lang.SourceFor(docVariant), dst)
				r.addStat(name, fsutil.ContractHome(dst)+" ["+state+"]")
			}
		}
	}
	return nil
}
