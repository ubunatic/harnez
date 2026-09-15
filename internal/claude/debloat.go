// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"ubunatic.com/harnez/internal/jsonc"
)

// Debloat presets and flags trim Claude Code's tool surface for token/context
// savings (issue 316). apply --debloat is global-only, same as the rest of
// `apply` (see docs/CLIDesign.md).

const (
	DebloatPresetMinimal    = "minimal"
	DebloatPresetAggressive = "aggressive"
)

// Preset deny-list content (which tool names belong to which preset) lives
// in config.yaml's `debloat:` section (DebloatConfig), not here — see
// docs/Spec.md: config.yaml is this project's single source of truth for
// apply-related settings, and Go code must not shadow it with hardcoded
// lists.

// debloatBoolKeys are the settings.json top-level boolean toggles debloat can
// set, in stable display order.
var debloatBoolKeys = []string{
	"disableBundledSkills",
	"disableWorkflows",
	"disableRemoteControl",
	"disableClaudeAiConnectors",
	"disableArtifact",
}

// DebloatOptions selects what `apply --debloat` denies/toggles this run.
type DebloatOptions struct {
	Preset                    string // "", DebloatPresetMinimal, or DebloatPresetAggressive
	NotebookEdit              bool
	Cron                      bool
	DisableBundledSkills      bool
	DisableWorkflows          bool
	DisableRemoteControl      bool
	DisableClaudeAiConnectors bool
	DisableArtifact           bool
}

// Requested reports whether any debloat behavior was selected.
func (o DebloatOptions) Requested() bool {
	return o.Preset != "" || o.NotebookEdit || o.Cron ||
		o.DisableBundledSkills || o.DisableWorkflows || o.DisableRemoteControl ||
		o.DisableClaudeAiConnectors || o.DisableArtifact
}

func (o DebloatOptions) denyList(cfg DebloatConfig) ([]string, error) {
	var deny []string
	switch o.Preset {
	case "":
		// no preset selected; opt-in flags below may still add entries.
	case DebloatPresetMinimal:
		deny = append(deny, cfg.MinimalDeny...)
	case DebloatPresetAggressive:
		deny = append(deny, cfg.MinimalDeny...)
		deny = append(deny, cfg.AggressiveExtraDeny...)
	default:
		return nil, fmt.Errorf("unknown --debloat-preset %q (want %q or %q)", o.Preset, DebloatPresetMinimal, DebloatPresetAggressive)
	}
	if o.NotebookEdit {
		deny = append(deny, cfg.NotebookDeny...)
	}
	if o.Cron {
		deny = append(deny, cfg.CronDeny...)
	}
	return deny, nil
}

func (o DebloatOptions) boolToggles() map[string]bool {
	return map[string]bool{
		"disableBundledSkills":      o.DisableBundledSkills,
		"disableWorkflows":          o.DisableWorkflows,
		"disableRemoteControl":      o.DisableRemoteControl,
		"disableClaudeAiConnectors": o.DisableClaudeAiConnectors,
		"disableArtifact":           o.DisableArtifact,
	}
}

// debloatRecord is the sidecar ownership record persisted at
// <target>/.harnez-debloat.json. It only ever grows across repeated
// apply --debloat calls: the *first* time a key is touched, its prior value
// is captured and never overwritten, so revert restores the state from
// before harnez ever touched debloat settings — not the state from the most
// recent apply.
type debloatRecord struct {
	// DenyAdded lists deny entries harnez itself introduced (i.e. that were
	// absent from permissions.deny the first time harnez added them).
	// Entries already present before harnez touched them are never listed
	// here, so revert leaves them alone.
	DenyAdded []string `json:"denyAdded"`
	// BoolPrior maps a toggle key to its value before harnez first set it.
	// A nil value means the key was absent before.
	BoolPrior map[string]*bool `json:"boolPrior"`
}

func debloatRecordPath(target string) string {
	return filepath.Join(target, ".harnez-debloat.json")
}

func readDebloatRecord(target string) debloatRecord {
	rec := debloatRecord{BoolPrior: map[string]*bool{}}
	data, err := os.ReadFile(debloatRecordPath(target))
	if err != nil {
		return rec
	}
	_ = json.Unmarshal(data, &rec)
	if rec.BoolPrior == nil {
		rec.BoolPrior = map[string]*bool{}
	}
	return rec
}

func writeDebloatRecord(target string, rec debloatRecord) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(debloatRecordPath(target), append(data, '\n'), 0644)
}

// ApplyDebloat merges the requested deny entries and boolean toggles into
// <target>/settings.json, preserving every other field untouched, and
// records prior values for the keys it touches for the first time so
// RevertDebloat can restore them exactly.
func ApplyDebloat(target string, cfg DebloatConfig, opts DebloatOptions) error {
	deny, err := opts.denyList(cfg)
	if err != nil {
		return err
	}
	settingsPath := filepath.Join(target, "settings.json")
	settings := jsonc.Read(settingsPath)
	rec := readDebloatRecord(target)

	perms, _ := settings["permissions"].(map[string]any)
	if perms == nil {
		perms = map[string]any{}
	}
	existingDeny := jsonc.ToStrings(perms["deny"])

	addedTracked := map[string]struct{}{}
	for _, d := range rec.DenyAdded {
		addedTracked[d] = struct{}{}
	}
	existingSet := map[string]struct{}{}
	for _, d := range existingDeny {
		existingSet[d] = struct{}{}
	}
	for _, d := range deny {
		if _, already := existingSet[d]; already {
			continue
		}
		if _, tracked := addedTracked[d]; tracked {
			continue
		}
		rec.DenyAdded = append(rec.DenyAdded, d)
		addedTracked[d] = struct{}{}
	}
	sort.Strings(rec.DenyAdded)

	if len(deny) > 0 {
		perms["deny"] = jsonc.UnionStrings(existingDeny, deny)
		settings["permissions"] = perms
	}

	for key, want := range opts.boolToggles() {
		if !want {
			continue
		}
		if _, tracked := rec.BoolPrior[key]; !tracked {
			if cur, ok := settings[key].(bool); ok {
				curCopy := cur
				rec.BoolPrior[key] = &curCopy
			} else {
				rec.BoolPrior[key] = nil
			}
		}
		settings[key] = true
	}

	data := append(jsonc.MarshalPretty(settings), '\n')
	if err := jsonc.Validate(data); err != nil {
		return fmt.Errorf("%s: %w", settingsPath, err)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return err
	}
	return writeDebloatRecord(target, rec)
}

// RevertDebloat restores every key harnez's debloat feature has touched to
// its pre-debloat state (from the sidecar ownership record) and removes the
// record. Keys/entries that pre-existed before harnez touched them, or that
// harnez never touched, are left untouched.
func RevertDebloat(target string) error {
	recordPath := debloatRecordPath(target)
	if _, err := os.Stat(recordPath); os.IsNotExist(err) {
		return fmt.Errorf("no debloat record found at %s (nothing to revert)", recordPath)
	}
	rec := readDebloatRecord(target)

	settingsPath := filepath.Join(target, "settings.json")
	settings := jsonc.Read(settingsPath)

	if len(rec.DenyAdded) > 0 {
		if perms, ok := settings["permissions"].(map[string]any); ok {
			existingDeny := jsonc.ToStrings(perms["deny"])
			added := map[string]struct{}{}
			for _, d := range rec.DenyAdded {
				added[d] = struct{}{}
			}
			kept := make([]string, 0, len(existingDeny))
			for _, d := range existingDeny {
				if _, wasAdded := added[d]; wasAdded {
					continue
				}
				kept = append(kept, d)
			}
			if len(kept) == 0 {
				delete(perms, "deny")
			} else {
				perms["deny"] = kept
			}
			if len(perms) == 0 {
				delete(settings, "permissions")
			} else {
				settings["permissions"] = perms
			}
		}
	}

	for key, prior := range rec.BoolPrior {
		if prior == nil {
			delete(settings, key)
		} else {
			settings[key] = *prior
		}
	}

	data := append(jsonc.MarshalPretty(settings), '\n')
	if err := jsonc.Validate(data); err != nil {
		return fmt.Errorf("%s: %w", settingsPath, err)
	}
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return err
	}
	return os.Remove(recordPath)
}

// StatusDebloat prints the currently-active debloat-managed deny entries and
// boolean toggles for <target>/settings.json, naming the file explicitly and
// noting that it is a global, all-projects setting.
func StatusDebloat(target string, cfg DebloatConfig) error {
	settingsPath := filepath.Join(target, "settings.json")
	settings := jsonc.Read(settingsPath)
	rec := readDebloatRecord(target)

	fmt.Printf("debloat status for %s (global — affects every project for this user)\n", settingsPath)

	perms, _ := settings["permissions"].(map[string]any)
	deny := jsonc.ToStrings(perms["deny"])
	denySet := map[string]struct{}{}
	for _, d := range deny {
		denySet[d] = struct{}{}
	}

	allKnownDeny := append(append(append([]string{}, cfg.MinimalDeny...), cfg.AggressiveExtraDeny...), append(cfg.CronDeny, cfg.NotebookDeny...)...)
	sort.Strings(allKnownDeny)
	fmt.Println("permissions.deny:")
	for _, d := range allKnownDeny {
		_, active := denySet[d]
		ownedByHarnez := false
		for _, owned := range rec.DenyAdded {
			if owned == d {
				ownedByHarnez = true
			}
		}
		state := "off"
		if active {
			state = "on"
			if ownedByHarnez {
				state += " (harnez-managed)"
			} else {
				state += " (pre-existing)"
			}
		}
		fmt.Printf("  %-16s %s\n", d, state)
	}

	fmt.Println("toggles:")
	for _, key := range debloatBoolKeys {
		val, _ := settings[key].(bool)
		state := "off"
		if val {
			state = "on"
			if _, tracked := rec.BoolPrior[key]; tracked {
				state += " (harnez-managed)"
			} else {
				state += " (pre-existing)"
			}
		}
		fmt.Printf("  %-26s %s\n", key, state)
	}
	return nil
}
