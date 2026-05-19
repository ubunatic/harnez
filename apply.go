package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func applySection(path, section, content string) error {
	begin := fmt.Sprintf("// claudeconfig:begin %s", section)
	end := fmt.Sprintf("// claudeconfig:end %s", section)
	block := begin + "\n" + strings.TrimRight(content, "\n") + "\n" + end + "\n"

	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	var result string
	bi := strings.Index(existing, begin)
	ei := strings.Index(existing, end)
	if bi >= 0 && ei >= 0 && ei > bi {
		lineStart := strings.LastIndex(existing[:bi], "\n") + 1
		lineEnd := ei + len(end)
		if lineEnd < len(existing) && existing[lineEnd] == '\n' {
			lineEnd++
		}
		result = existing[:lineStart] + block + existing[lineEnd:]
	} else {
		if existing != "" && !strings.HasSuffix(existing, "\n") {
			existing += "\n"
		}
		result = existing + block
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(result), 0644)
}

func diffSection(path, section, content string) (bool, error) {
	begin := fmt.Sprintf("// claudeconfig:begin %s", section)
	end := fmt.Sprintf("// claudeconfig:end %s", section)
	newBlock := begin + "\n" + strings.TrimRight(content, "\n") + "\n" + end + "\n"

	oldBlock := ""
	if data, err := os.ReadFile(path); err == nil {
		existing := string(data)
		bi := strings.Index(existing, begin)
		ei := strings.Index(existing, end)
		if bi >= 0 && ei >= 0 && ei > bi {
			lineStart := strings.LastIndex(existing[:bi], "\n") + 1
			lineEnd := ei + len(end)
			if lineEnd < len(existing) && existing[lineEnd] == '\n' {
				lineEnd++
			}
			oldBlock = existing[lineStart:lineEnd]
		}
	}

	if oldBlock == newBlock {
		return false, nil
	}

	writeTemp := func(s string) (string, error) {
		f, err := os.CreateTemp("", "claudeconfig-diff-*")
		if err != nil {
			return "", err
		}
		_, err = f.WriteString(s)
		f.Close()
		return f.Name(), err
	}

	oldFile, err := writeTemp(oldBlock)
	if err != nil {
		return true, err
	}
	defer os.Remove(oldFile)

	newFile, err := writeTemp(newBlock)
	if err != nil {
		return true, err
	}
	defer os.Remove(newFile)

	label := fmt.Sprintf("%s [%s]", path, section)
	cmd := exec.Command("diff", "-u", "--label", label, "--label", label, oldFile, newFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	return true, nil
}

func cleanSection(path, section string) error {
	begin := fmt.Sprintf("// claudeconfig:begin %s", section)
	end := fmt.Sprintf("// claudeconfig:end %s", section)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	existing := string(data)

	bi := strings.Index(existing, begin)
	ei := strings.Index(existing, end)
	if bi < 0 || ei < 0 || ei <= bi {
		return nil
	}

	lineStart := strings.LastIndex(existing[:bi], "\n") + 1
	lineEnd := ei + len(end)
	if lineEnd < len(existing) && existing[lineEnd] == '\n' {
		lineEnd++
	}
	result := existing[:lineStart] + existing[lineEnd:]

	if strings.TrimSpace(result) == "" {
		fmt.Printf("  removed %s\n", path)
		return os.Remove(path)
	}
	fmt.Printf("  cleaned %s\n", path)
	return os.WriteFile(path, []byte(result), 0644)
}

// genSettings generates the content for the managed block inside settings.json.
// The block contains only the JSON keys claudeconfig owns; the file's outer { } is user territory.
func genSettings(cfg *Config) string {
	// Build hooks grouped by event (Claude Code format: hooks[event] = [{matcher, hooks:[{type,command}]}])
	type rawHook struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}
	type rawMatcher struct {
		Matcher string    `json:"matcher,omitempty"`
		Hooks   []rawHook `json:"hooks"`
	}

	eventMap := map[string][]rawMatcher{}
	for _, h := range cfg.Hooks {
		m := rawMatcher{
			Matcher: h.Matcher,
			Hooks:   []rawHook{{Type: "command", Command: h.Command}},
		}
		eventMap[h.Event] = append(eventMap[h.Event], m)
	}

	doc := map[string]any{}
	if len(cfg.Permissions.Allow) > 0 || len(cfg.Permissions.Deny) > 0 {
		allow := cfg.Permissions.Allow
		if allow == nil {
			allow = []string{}
		}
		deny := cfg.Permissions.Deny
		if deny == nil {
			deny = []string{}
		}
		doc["permissions"] = map[string]any{"allow": allow, "deny": deny}
	}
	if len(eventMap) > 0 {
		doc["hooks"] = eventMap
	}
	if len(cfg.Env) > 0 {
		doc["env"] = cfg.Env
	}
	if len(cfg.Verbs) > 0 {
		doc["spinnerVerbs"] = map[string]any{"mode": "replace", "verbs": cfg.Verbs}
	}

	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(doc)
	s := strings.TrimSpace(buf.String())
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	s = strings.TrimSpace(s)
	return s
}

func applyAll(target string, cfg *Config) error {
	settingsPath := filepath.Join(target, "settings.json")
	if err := applySection(settingsPath, "settings", genSettings(cfg)); err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	fmt.Printf("  wrote %s\n", settingsPath)
	return nil
}

func diffAll(target string, cfg *Config) error {
	settingsPath := filepath.Join(target, "settings.json")
	changed, err := diffSection(settingsPath, "settings", genSettings(cfg))
	if err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	if !changed {
		fmt.Println("No changes.")
	}
	return nil
}

func cleanAll(target string, _ *Config) error {
	settingsPath := filepath.Join(target, "settings.json")
	if err := cleanSection(settingsPath, "settings"); err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	return nil
}
