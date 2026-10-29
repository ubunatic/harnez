// Package picompact derives Pi compaction overrides for small-window models
// (issue 1023). It reads Pi's own models.json and writes
// compaction.modelOverrides["<provider>/<id>"] into Pi's global settings.json.
// Pi resolves each token setting as model override, then plain
// compaction.* field, then built-in default. All tuning values come in through
// Params, which callers load from spec/agent.yaml.
package picompact

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Params are the spec values that decide which models get an override and how large it is.
type Params struct {
	Threshold       int
	ReserveFraction float64
	KeepFraction    float64
}

// Override is one compaction.modelOverrides entry.
type Override struct {
	ReserveTokens    int `json:"reserveTokens"`
	KeepRecentTokens int `json:"keepRecentTokens"`
}

// Derive returns the override for a context window, or false when the window
// is unknown or not below the threshold. Values are floored: Pi requires integers.
func Derive(window int, p Params) (Override, bool) {
	if window <= 0 || window >= p.Threshold {
		return Override{}, false
	}
	return Override{
		ReserveTokens:    int(float64(window) * p.ReserveFraction),
		KeepRecentTokens: int(float64(window) * p.KeepFraction),
	}, true
}

type modelsFile struct {
	Providers map[string]struct {
		Models []struct {
			ID            string `json:"id"`
			ContextWindow int    `json:"contextWindow"`
		} `json:"models"`
	} `json:"providers"`
}

// LoadModels maps "<provider>/<id>" to contextWindow from <dir>/models.json.
// A missing file returns nil and no error.
func LoadModels(dir string) (map[string]int, error) {
	data, err := os.ReadFile(filepath.Join(dir, "models.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f modelsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse models.json: %w", err)
	}
	windows := map[string]int{}
	for provider, entry := range f.Providers {
		for _, m := range entry.Models {
			if m.ID != "" {
				windows[provider+"/"+m.ID] = m.ContextWindow
			}
		}
	}
	return windows, nil
}

// Plan returns the overrides harnez wants for the given model windows.
func Plan(windows map[string]int, p Params) map[string]Override {
	want := map[string]Override{}
	for key, window := range windows {
		if o, ok := Derive(window, p); ok {
			want[key] = o
		}
	}
	return want
}

// Merge adds the wanted overrides to settings.json content. An existing entry
// counts as harnez's own only when it equals what harnez would write now;
// any other entry is kept and its key returned in skipped.
func Merge(settings []byte, want map[string]Override) (out []byte, changed bool, skipped []string, err error) {
	doc := map[string]any{}
	if len(strings.TrimSpace(string(settings))) > 0 {
		if err := json.Unmarshal(settings, &doc); err != nil {
			return nil, false, nil, fmt.Errorf("parse settings.json: %w", err)
		}
		if doc == nil {
			return nil, false, nil, fmt.Errorf("parse settings.json: not an object")
		}
	}
	compaction, err := childObject(doc, "compaction")
	if err != nil {
		return nil, false, nil, err
	}
	overrides, err := childObject(compaction, "modelOverrides")
	if err != nil {
		return nil, false, nil, err
	}
	keys := make([]string, 0, len(want))
	for key := range want {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		existing, ok := overrides[key]
		if !ok {
			overrides[key] = want[key]
			changed = true
			continue
		}
		if !sameOverride(existing, want[key]) {
			skipped = append(skipped, key)
		}
	}
	if !changed {
		return settings, false, skipped, nil
	}
	compaction["modelOverrides"] = overrides
	doc["compaction"] = compaction
	out, err = json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, false, nil, err
	}
	return append(out, '\n'), true, skipped, nil
}

func childObject(parent map[string]any, key string) (map[string]any, error) {
	v, ok := parent[key]
	if !ok || v == nil {
		return map[string]any{}, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("parse settings.json: %s is not an object", key)
	}
	return m, nil
}

func sameOverride(existing any, o Override) bool {
	m, ok := existing.(map[string]any)
	if !ok || len(m) != 2 {
		return false
	}
	return m["reserveTokens"] == float64(o.ReserveTokens) && m["keepRecentTokens"] == float64(o.KeepRecentTokens)
}

// Sync writes the overrides for small-window models in <dir>/models.json into
// <dir>/settings.json. A missing dir or models.json is a silent no-op; a
// malformed file prints one warning and leaves settings.json untouched.
// Kept entries that differ from the derived values are named in one warning line.
func Sync(dir string, p Params, warn io.Writer) error {
	if dir == "" {
		return nil
	}
	windows, err := LoadModels(dir)
	if err != nil {
		fmt.Fprintf(warn, "Warning: Pi compaction overrides not updated: %s: %v\n", dir, err)
		return nil
	}
	want := Plan(windows, p)
	if len(want) == 0 {
		return nil
	}
	path := filepath.Join(dir, "settings.json")
	mode := fs.FileMode(0o600)
	settings, err := os.ReadFile(path)
	if err == nil {
		if info, statErr := os.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(warn, "Warning: Pi compaction overrides not updated: %v\n", err)
		return nil
	}
	out, changed, skipped, err := Merge(settings, want)
	if err != nil {
		fmt.Fprintf(warn, "Warning: Pi compaction overrides not updated: %s: %v\n", path, err)
		return nil
	}
	if len(skipped) > 0 {
		fmt.Fprintf(warn, "Warning: kept existing Pi compaction overrides in %s: %s\n", path, strings.Join(skipped, ", "))
	}
	if !changed {
		return nil
	}
	return writeAtomic(path, out, mode)
}

func writeAtomic(path string, data []byte, mode fs.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings.json.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
