package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// debloatRecord captures the original values on the first debloat apply.
// A nil feature value means the key did not exist before harnez touched it.
type debloatRecord struct {
	FileExisted     bool             `json:"fileExisted"`
	FeaturesExisted bool             `json:"featuresExisted"`
	Prior           map[string]*bool `json:"prior"`
}

func debloatRecordPath(path string) string {
	return path + ".harnez-debloat.json"
}

func readDebloatRecord(path string) (debloatRecord, bool, error) {
	data, err := os.ReadFile(debloatRecordPath(path))
	if errors.Is(err, os.ErrNotExist) {
		return debloatRecord{Prior: map[string]*bool{}}, false, nil
	}
	if err != nil {
		return debloatRecord{}, false, err
	}
	var rec debloatRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return rec, true, fmt.Errorf("decode Codex debloat record: %w", err)
	}
	if rec.Prior == nil {
		return rec, true, fmt.Errorf("Codex debloat record has no prior values")
	}
	return rec, true, nil
}

func readDebloatConfig(path string) (map[string]any, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	doc := map[string]any{}
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return nil, true, fmt.Errorf("decode %s: %w", path, err)
	}
	return doc, true, nil
}

func writeDebloatConfig(path string, doc map[string]any) error {
	data, err := encodeTOML(doc)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ApplyDebloat sets only Codex feature keys listed in the spec. It records
// original values once, and restores keys removed from the spec on reapply.
func ApplyDebloat(path string, wanted map[string]bool) (bool, error) {
	doc, fileExisted, err := readDebloatConfig(path)
	if err != nil {
		return false, err
	}
	rec, recorded, err := readDebloatRecord(path)
	if err != nil {
		return false, err
	}
	if len(wanted) == 0 && !recorded {
		return false, nil
	}
	features, featuresExisted := doc["features"].(map[string]any)
	if raw, exists := doc["features"]; exists && !featuresExisted {
		return false, fmt.Errorf("%s: features is %T, want TOML table", path, raw)
	}
	if !recorded {
		rec.FileExisted = fileExisted
		rec.FeaturesExisted = featuresExisted
	}
	if features == nil {
		features = map[string]any{}
	}
	changed := false
	recordChanged := !recorded
	for key, prior := range rec.Prior {
		if _, included := wanted[key]; included {
			continue
		}
		if prior == nil {
			if _, present := features[key]; present {
				changed = true
			}
			delete(features, key)
		} else {
			current, present := features[key].(bool)
			if !present || current != *prior {
				changed = true
			}
			features[key] = *prior
		}
		delete(rec.Prior, key)
		recordChanged = true
	}
	for key, want := range wanted {
		if key == "" {
			return false, fmt.Errorf("Codex debloat spec contains an empty feature name")
		}
		if _, tracked := rec.Prior[key]; !tracked {
			recordChanged = true
			if cur, present := features[key]; present {
				prior, ok := cur.(bool)
				if !ok {
					return false, fmt.Errorf("%s: features.%s is %T, want boolean", path, key, cur)
				}
				priorCopy := prior
				rec.Prior[key] = &priorCopy
			} else {
				rec.Prior[key] = nil
			}
		}
		if cur, present := features[key].(bool); !present || cur != want {
			changed = true
		}
		features[key] = want
	}
	if len(features) == 0 && !rec.FeaturesExisted {
		delete(doc, "features")
	} else {
		doc["features"] = features
	}
	if !changed && !recordChanged {
		return false, nil
	}
	if len(rec.Prior) == 0 {
		if len(doc) == 0 && !rec.FileExisted {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return false, err
			}
		} else if err := writeDebloatConfig(path, doc); err != nil {
			return false, err
		}
		return changed, os.Remove(debloatRecordPath(path))
	}
	recordData, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, err
	}
	if err := os.WriteFile(debloatRecordPath(path), append(recordData, '\n'), 0644); err != nil {
		return false, err
	}
	if err := writeDebloatConfig(path, doc); err != nil {
		return false, err
	}
	return changed, nil
}

// RevertDebloat restores only features recorded by ApplyDebloat. A missing
// ownership record is a no-op, allowing older Claude-only debloat installs.
func RevertDebloat(path string) (bool, error) {
	rec, recorded, err := readDebloatRecord(path)
	if err != nil || !recorded {
		return false, err
	}
	doc, _, err := readDebloatConfig(path)
	if err != nil {
		return false, err
	}
	features, ok := doc["features"].(map[string]any)
	if !ok {
		if raw, exists := doc["features"]; exists {
			return false, fmt.Errorf("%s: features is %T, want TOML table", path, raw)
		}
		features = map[string]any{}
	}
	for key, prior := range rec.Prior {
		if prior == nil {
			delete(features, key)
		} else {
			features[key] = *prior
		}
	}
	if len(features) == 0 && !rec.FeaturesExisted {
		delete(doc, "features")
	} else {
		doc["features"] = features
	}
	if len(doc) == 0 && !rec.FileExisted {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	} else if err := writeDebloatConfig(path, doc); err != nil {
		return false, err
	}
	return true, os.Remove(debloatRecordPath(path))
}

// StatusDebloat prints every spec-managed Codex feature and its current
// value. All unlisted settings are outside debloat's scope.
func StatusDebloat(path string, wanted map[string]bool) error {
	doc, _, err := readDebloatConfig(path)
	if err != nil {
		return err
	}
	rec, _, err := readDebloatRecord(path)
	if err != nil {
		return err
	}
	features, ok := doc["features"].(map[string]any)
	if raw, exists := doc["features"]; exists && !ok {
		return fmt.Errorf("%s: features is %T, want TOML table", path, raw)
	}
	keys := make([]string, 0, len(wanted))
	for key := range wanted {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Printf("Codex debloat status for %s (global)\n", path)
	for _, key := range keys {
		value := "unset"
		if current, present := features[key]; present {
			value = fmt.Sprint(current)
		}
		owner := ""
		if _, tracked := rec.Prior[key]; tracked {
			owner = " (harnez-managed)"
		}
		fmt.Printf("  features.%-18s %s%s (preset: %v)\n", key, value, owner, wanted[key])
	}
	previouslyManaged := make([]string, 0, len(rec.Prior))
	for key := range rec.Prior {
		if _, included := wanted[key]; !included {
			previouslyManaged = append(previouslyManaged, key)
		}
	}
	sort.Strings(previouslyManaged)
	for _, key := range previouslyManaged {
		fmt.Printf("  features.%s: previously managed; excluded from current spec (reapply to restore)\n", key)
	}
	fmt.Println("  All other unlisted Codex settings: unmanaged by debloat")
	return nil
}
