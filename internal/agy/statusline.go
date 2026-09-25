// Package agy manages Harnez's optional Antigravity CLI status line.
package agy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"ubunatic.com/harnez/internal/jsonc"
)

const statusLineCommand = "harnez agy-statusline"
const sharedStatusLineCommand = "harnez statusline --agent agy"

// StatusLinePath returns Antigravity CLI's global settings path.
func StatusLinePath(home string) string {
	return StatusLineSettingsPath(filepath.Join(home, ".gemini", "antigravity-cli"))
}

// StatusLineSettingsPath returns the statusLine settings file under configDir.
func StatusLineSettingsPath(configDir string) string {
	return filepath.Join(configDir, "settings.json")
}

func buildStatusLine() map[string]any {
	return map[string]any{
		"type":               "command",
		"command":            sharedStatusLineCommand,
		"enabled":            true,
		"stack_with_default": true,
	}
}

// ApplyStatusLine installs Harnez's status line and preserves other settings.
// It refuses to replace a non-Harnez command already configured by the user.
func ApplyStatusLine(path string) (bool, error) {
	doc := jsonc.Read(path)
	if raw, exists := doc["statusLine"]; exists {
		existing, ok := raw.(map[string]any)
		if !ok {
			return false, fmt.Errorf("agy status line: existing statusLine in %s is not an object", path)
		}
		command, _ := existing["command"].(string)
		if command != "" && command != statusLineCommand && command != sharedStatusLineCommand {
			return false, fmt.Errorf("agy status line: %q is already configured in %s", command, path)
		}
	}
	doc["statusLine"] = buildStatusLine()
	data := append(jsonc.MarshalPretty(doc), '\n')
	if err := jsonc.Validate(data); err != nil {
		return false, err
	}
	old, _ := os.ReadFile(path)
	if string(old) == string(data) {
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

// StatusLineStatus reports whether Harnez's status line is installed and drifted.
func StatusLineStatus(path string) (installed, drifted bool) {
	doc := jsonc.Read(path)
	existing, ok := doc["statusLine"].(map[string]any)
	if !ok {
		return false, false
	}
	command, _ := existing["command"].(string)
	if command != statusLineCommand && command != sharedStatusLineCommand {
		return false, false
	}
	want, _ := json.Marshal(buildStatusLine())
	got, _ := json.Marshal(existing)
	return true, command != sharedStatusLineCommand || string(want) != string(got)
}

// RemoveStatusLine removes Harnez's status line without touching a user-owned one.
func RemoveStatusLine(path string) (bool, error) {
	if installed, _ := StatusLineStatus(path); !installed {
		return false, nil
	}
	doc := jsonc.Read(path)
	delete(doc, "statusLine")
	data := append(jsonc.MarshalPretty(doc), '\n')
	if err := jsonc.Validate(data); err != nil {
		return false, err
	}
	if len(doc) == 0 {
		return true, os.Remove(path)
	}
	return true, os.WriteFile(path, data, 0644)
}
