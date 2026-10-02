package codex

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

const statusLineSpecPath = "spec/statusline.yaml"

type codexStatusLineSpec struct {
	Items             []string `yaml:"items"`
	RemoveLegacyItems []string `yaml:"remove_legacy_items"`
}

func loadCodexStatusLineSpec() (codexStatusLineSpec, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, statusLineSpecPath)
	if err != nil {
		return codexStatusLineSpec{}, fmt.Errorf("read embedded %s: %w", statusLineSpecPath, err)
	}
	var spec struct {
		StatusLine struct {
			Claude string              `yaml:"claude"`
			AGY    string              `yaml:"agy"`
			Codex  codexStatusLineSpec `yaml:"codex"`
		} `yaml:"statusline"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&spec); err != nil {
		return codexStatusLineSpec{}, fmt.Errorf("parse embedded %s: %w", statusLineSpecPath, err)
	}
	if len(spec.StatusLine.Codex.Items) == 0 {
		return codexStatusLineSpec{}, fmt.Errorf("%s: statusline.codex.items must not be empty", statusLineSpecPath)
	}
	return spec.StatusLine.Codex, nil
}

// ApplyStatusLine writes exactly the Codex footer items selected in the embedded spec.
func ApplyStatusLine(path string) (bool, error) {
	statusSpec, err := loadCodexStatusLineSpec()
	if err != nil {
		return false, err
	}
	doc := readTOML(path)
	tui := map[string]any{}
	if existing, ok := doc["tui"].(map[string]any); ok {
		for key, value := range existing {
			tui[key] = value
		}
	}
	tui["status_line"] = append([]string(nil), statusSpec.Items...)
	doc["tui"] = tui
	return writeTOML(path, doc)
}

// StatusLineStatus reports whether the configured footer exactly matches the embedded spec.
func StatusLineStatus(path string) (installed, drifted bool) {
	statusSpec, err := loadCodexStatusLineSpec()
	if err != nil {
		return false, false
	}
	doc := readTOML(path)
	tui, ok := doc["tui"].(map[string]any)
	if !ok {
		return false, false
	}
	items := stringArray(tui["status_line"])
	if _, ok := tui["status_line"]; !ok {
		return false, false
	}
	installed = reflect.DeepEqual(items, statusSpec.Items)
	return installed, !installed
}

// RemoveStatusLine removes current and retired Harnez footer items and keeps others.
func RemoveStatusLine(path string) (bool, error) {
	statusSpec, err := loadCodexStatusLineSpec()
	if err != nil {
		return false, err
	}
	doc := readTOML(path)
	tui, ok := doc["tui"].(map[string]any)
	if !ok {
		return false, nil
	}
	items := stringArray(tui["status_line"])
	filtered := make([]string, 0, len(items))
	removed := false
	for _, item := range items {
		if containsString(statusSpec.Items, item) || containsString(statusSpec.RemoveLegacyItems, item) {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !removed {
		return false, nil
	}
	tui["status_line"] = filtered
	doc["tui"] = tui
	return writeTOML(path, doc)
}

func stringArray(value any) []string {
	var items []string
	switch values := value.(type) {
	case []string:
		return append(items, values...)
	case []any:
		for _, value := range values {
			if item, ok := value.(string); ok {
				items = append(items, item)
			}
		}
	}
	return items
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func writeTOML(path string, doc map[string]any) (bool, error) {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(doc); err != nil {
		return false, err
	}
	data := buf.Bytes()
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
