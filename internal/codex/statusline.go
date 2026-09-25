package codex

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

var statusLineItems = []string{
	"model-with-reasoning",
	"current-dir",
	"thread-name",
	"context-remaining",
	"model-context",
	"five-hour-limit",
	"weekly-limit",
}

// ApplyStatusLine enables Codex's native footer items while preserving any
// unrelated TUI settings and user-selected status-line items.
func ApplyStatusLine(path string) (bool, error) {
	doc := readTOML(path)
	tui := map[string]any{}
	if existing, ok := doc["tui"].(map[string]any); ok {
		for key, value := range existing {
			tui[key] = value
		}
	}
	items := stringArray(tui["status_line"])
	for _, required := range statusLineItems {
		if !containsString(items, required) {
			items = append(items, required)
		}
	}
	tui["status_line"] = items
	doc["tui"] = tui
	return writeTOML(path, doc)
}

// StatusLineStatus reports whether all Harnez-selected native footer items
// are present. Additional user-selected items are allowed.
func StatusLineStatus(path string) (installed, drifted bool) {
	doc := readTOML(path)
	tui, ok := doc["tui"].(map[string]any)
	if !ok {
		return false, false
	}
	items := stringArray(tui["status_line"])
	for _, required := range statusLineItems {
		if !containsString(items, required) {
			return false, false
		}
	}
	return true, false
}

// RemoveStatusLine removes Harnez-selected footer items and keeps all others.
func RemoveStatusLine(path string) (bool, error) {
	doc := readTOML(path)
	tui, ok := doc["tui"].(map[string]any)
	if !ok {
		return false, nil
	}
	items := stringArray(tui["status_line"])
	filtered := make([]string, 0, len(items))
	removed := false
	for _, item := range items {
		if containsString(statusLineItems, item) {
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
