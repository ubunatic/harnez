// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package agy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ubunatic.com/harnez/internal/jsonc"
)

// DefaultTarget is the default Antigravity CLI config directory.
const DefaultTarget = "~/.gemini/antigravity-cli"

// DebloatConfig defines the Antigravity debloat tool lists from config.yaml.
type DebloatConfig struct {
	MinimalDeny         []string `yaml:"minimal_deny"`
	AggressiveExtraDeny []string `yaml:"aggressive_extra_deny"`
}

// DenyList returns the tools to deny for the given preset.
func (cfg DebloatConfig) DenyList(preset string) ([]string, error) {
	switch preset {
	case "", "minimal":
		return cfg.MinimalDeny, nil
	case "aggressive":
		deny := make([]string, 0, len(cfg.MinimalDeny)+len(cfg.AggressiveExtraDeny))
		deny = append(deny, cfg.MinimalDeny...)
		deny = append(deny, cfg.AggressiveExtraDeny...)
		return deny, nil
	default:
		return nil, fmt.Errorf("unknown Antigravity debloat preset %q (want %q or %q)", preset, "minimal", "aggressive")
	}
}

// debloatRecord captures the deny entries harnez introduced for Antigravity.
type debloatRecord struct {
	DenyAdded []string `json:"denyAdded"`
}

// IsAgyTarget reports whether target refers to an Antigravity directory or settings file.
func IsAgyTarget(target string) bool {
	lower := strings.ToLower(target)
	return strings.Contains(lower, "antigravity") || strings.Contains(lower, ".gemini")
}

func resolvePaths(target string) (settingsPath, recordPath string) {
	if strings.HasSuffix(target, ".json") {
		settingsPath = target
		recordPath = filepath.Join(filepath.Dir(target), ".harnez-debloat.json")
	} else {
		settingsPath = filepath.Join(target, "settings.json")
		recordPath = filepath.Join(target, ".harnez-debloat.json")
	}
	return settingsPath, recordPath
}

func readDebloatRecord(path string) debloatRecord {
	rec := debloatRecord{}
	data, err := os.ReadFile(path)
	if err != nil {
		return rec
	}
	_ = json.Unmarshal(data, &rec)
	return rec
}

func writeDebloatRecord(path string, rec debloatRecord) error {
	if len(rec.DenyAdded) == 0 {
		_ = os.Remove(path)
		return nil
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// ApplyDebloat merges the requested Antigravity deny entries into settings.json
// and records them in .harnez-debloat.json for clean revert.
func ApplyDebloat(target string, cfg DebloatConfig, preset string) (bool, error) {
	wantedDeny, err := cfg.DenyList(preset)
	if err != nil {
		return false, err
	}

	settingsPath, recordPath := resolvePaths(target)
	settings := jsonc.Read(settingsPath)
	rec := readDebloatRecord(recordPath)

	perms, _ := settings["permissions"].(map[string]any)
	if perms == nil {
		perms = map[string]any{}
	}
	existingDeny := jsonc.ToStrings(perms["deny"])

	wantedSet := map[string]bool{}
	for _, d := range wantedDeny {
		wantedSet[d] = true
	}

	// Remove previously added tools that are no longer wanted
	keptAdded := make([]string, 0, len(rec.DenyAdded))
	removedTools := map[string]bool{}
	for _, d := range rec.DenyAdded {
		if wantedSet[d] {
			keptAdded = append(keptAdded, d)
		} else {
			removedTools[d] = true
		}
	}
	rec.DenyAdded = keptAdded

	if len(removedTools) > 0 {
		filteredExisting := make([]string, 0, len(existingDeny))
		for _, d := range existingDeny {
			if !removedTools[d] {
				filteredExisting = append(filteredExisting, d)
			}
		}
		existingDeny = filteredExisting
	}

	existingSet := map[string]bool{}
	for _, d := range existingDeny {
		existingSet[d] = true
	}
	addedTracked := map[string]bool{}
	for _, d := range rec.DenyAdded {
		addedTracked[d] = true
	}

	for _, d := range wantedDeny {
		if !existingSet[d] && !addedTracked[d] {
			rec.DenyAdded = append(rec.DenyAdded, d)
			addedTracked[d] = true
		}
	}
	sort.Strings(rec.DenyAdded)

	mergedDeny := jsonc.UnionStrings(existingDeny, wantedDeny)
	if len(mergedDeny) > 0 {
		perms["deny"] = mergedDeny
		settings["permissions"] = perms
	} else {
		delete(perms, "deny")
		if len(perms) == 0 {
			delete(settings, "permissions")
		} else {
			settings["permissions"] = perms
		}
	}

	data := append(jsonc.MarshalPretty(settings), '\n')
	if err := jsonc.Validate(data); err != nil {
		return false, fmt.Errorf("%s: %w", settingsPath, err)
	}

	old, _ := os.ReadFile(settingsPath)
	changed := string(old) != string(data)

	if changed {
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
			return false, err
		}
		if err := os.WriteFile(settingsPath, data, 0644); err != nil {
			return false, err
		}
	}

	if err := writeDebloatRecord(recordPath, rec); err != nil {
		return false, err
	}
	return changed, nil
}

// RevertDebloat restores permissions.deny in Antigravity settings.json by removing
// all deny entries recorded in .harnez-debloat.json, and deletes the record.
func RevertDebloat(target string) (bool, error) {
	settingsPath, recordPath := resolvePaths(target)
	if _, err := os.Stat(recordPath); os.IsNotExist(err) {
		return false, nil
	}
	rec := readDebloatRecord(recordPath)
	settings := jsonc.Read(settingsPath)

	if perms, ok := settings["permissions"].(map[string]any); ok {
		existingDeny := jsonc.ToStrings(perms["deny"])
		added := map[string]bool{}
		for _, d := range rec.DenyAdded {
			added[d] = true
		}
		kept := make([]string, 0, len(existingDeny))
		for _, d := range existingDeny {
			if !added[d] {
				kept = append(kept, d)
			}
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

	data := append(jsonc.MarshalPretty(settings), '\n')
	if err := jsonc.Validate(data); err != nil {
		return false, fmt.Errorf("%s: %w", settingsPath, err)
	}
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return false, err
	}
	return true, os.Remove(recordPath)
}

// StatusDebloat prints the debloat-managed deny entries for Antigravity settings.json.
func StatusDebloat(target string, cfg DebloatConfig) error {
	settingsPath, recordPath := resolvePaths(target)
	settings := jsonc.Read(settingsPath)
	rec := readDebloatRecord(recordPath)

	fmt.Printf("Antigravity debloat status for %s (global)\n", settingsPath)

	perms, _ := settings["permissions"].(map[string]any)
	deny := jsonc.ToStrings(perms["deny"])
	denySet := map[string]bool{}
	for _, d := range deny {
		denySet[d] = true
	}
	addedSet := map[string]bool{}
	for _, d := range rec.DenyAdded {
		addedSet[d] = true
	}

	allKnownDeny := append(append([]string{}, cfg.MinimalDeny...), cfg.AggressiveExtraDeny...)
	sort.Strings(allKnownDeny)
	fmt.Println("permissions.deny:")
	for _, d := range allKnownDeny {
		state := "off"
		if denySet[d] {
			state = "on"
			if addedSet[d] {
				state += " (harnez-managed)"
			} else {
				state += " (pre-existing)"
			}
		}
		fmt.Printf("  %-18s %s\n", d, state)
	}
	return nil
}
