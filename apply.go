package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ── Markdown managed blocks ───────────────────────────────────────────────────

var mdMarkers = struct {
	begin func(string) string
	end   func(string) string
}{
	begin: func(s string) string { return "<!-- claudeconfig:begin " + s + " -->" },
	end:   func(s string) string { return "<!-- claudeconfig:end " + s + " -->" },
}

func sectionBounds(existing, begin, end string) (lineStart, lineEnd int, found bool) {
	bi := strings.Index(existing, begin)
	ei := strings.Index(existing, end)
	if bi < 0 || ei < 0 || ei <= bi {
		return 0, 0, false
	}
	ls := strings.LastIndex(existing[:bi], "\n") + 1
	le := ei + len(end)
	if le < len(existing) && existing[le] == '\n' {
		le++
	}
	return ls, le, true
}

func applySectionMD(path, section, content string) error {
	begin := mdMarkers.begin(section)
	end := mdMarkers.end(section)
	block := begin + "\n" + strings.TrimRight(content, "\n") + "\n" + end + "\n"

	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	var result string
	if ls, le, ok := sectionBounds(existing, begin, end); ok {
		result = existing[:ls] + block + existing[le:]
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

func diffSectionMD(path, section, content string) (bool, error) {
	begin := mdMarkers.begin(section)
	end := mdMarkers.end(section)
	newBlock := begin + "\n" + strings.TrimRight(content, "\n") + "\n" + end + "\n"

	oldBlock := ""
	if data, err := os.ReadFile(path); err == nil {
		existing := string(data)
		if ls, le, ok := sectionBounds(existing, begin, end); ok {
			oldBlock = existing[ls:le]
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

func cleanSectionMD(path, section string) error {
	begin := mdMarkers.begin(section)
	end := mdMarkers.end(section)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	existing := string(data)

	ls, le, ok := sectionBounds(existing, begin, end)
	if !ok {
		return nil
	}
	result := existing[:ls] + existing[le:]

	if strings.TrimSpace(result) == "" {
		fmt.Printf("  removed %s\n", path)
		return os.Remove(path)
	}
	fmt.Printf("  cleaned %s\n", path)
	return os.WriteFile(path, []byte(result), 0644)
}

// ── JSON file helpers ─────────────────────────────────────────────────────────

// stripComments removes // line comments from JSONC for parsing.
func stripComments(data []byte) []byte {
	var result []byte
	inString := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if inString {
			result = append(result, b)
			if b == '\\' && i+1 < len(data) {
				i++
				result = append(result, data[i])
			} else if b == '"' {
				inString = false
			}
			continue
		}
		if b == '"' {
			inString = true
			result = append(result, b)
		} else if b == '/' && i+1 < len(data) && data[i+1] == '/' {
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				result = append(result, '\n')
			}
		} else {
			result = append(result, b)
		}
	}
	return result
}

func readJSONC(path string) map[string]any {
	m := map[string]any{}
	data, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	// Use Decoder so trailing content after the first JSON value is ignored.
	dec := json.NewDecoder(bytes.NewReader(stripComments(data)))
	dec.Decode(&m) //nolint:errcheck
	return m
}

func marshalPretty(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(v)
	return bytes.TrimRight(buf.Bytes(), "\n")
}

// managedSettingsKeys are the top-level keys claudeconfig writes to settings.json.
var managedSettingsKeys = []string{
	"model", "effortLevel", "permissions", "hooks", "env", "spinnerVerbs", "mcpServers",
}

// ── Model / effort resolution ─────────────────────────────────────────────────

var modelAliases = map[string]string{
	"sonnet": "claude-sonnet-4-6",
	"opus":   "claude-opus-4-7",
	"haiku":  "claude-haiku-4-5-20251001",
}

func resolveModel(s string) string {
	if full, ok := modelAliases[strings.ToLower(s)]; ok {
		return full
	}
	return s
}

// ── settings.json ─────────────────────────────────────────────────────────────

func buildSettingsDoc(cfg *Config) map[string]any {
	// Use map[string]any throughout so json.Marshal always sorts keys alphabetically,
	// matching what json.Unmarshal produces on read-back (round-trip stable).
	eventMap := map[string][]map[string]any{}
	for _, h := range cfg.Hooks {
		m := map[string]any{
			"hooks": []map[string]any{{"command": h.Command, "type": "command"}},
		}
		if h.Matcher != "" {
			m["matcher"] = h.Matcher
		}
		eventMap[h.Event] = append(eventMap[h.Event], m)
	}

	doc := map[string]any{}
	if cfg.Model != "" {
		doc["model"] = resolveModel(cfg.Model)
	}
	if cfg.Effort != "" {
		doc["effortLevel"] = cfg.Effort
	}
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
	if len(cfg.MCPServers) > 0 {
		servers := map[string]any{}
		for _, s := range cfg.MCPServers {
			srv := map[string]any{"command": s.Command}
			if len(s.Args) > 0 {
				srv["args"] = s.Args
			}
			if len(s.Env) > 0 {
				srv["env"] = s.Env
			}
			servers[s.Name] = srv
		}
		doc["mcpServers"] = servers
	}
	return doc
}

func validateJSON(data []byte) error {
	if !json.Valid(data) {
		return fmt.Errorf("generated invalid JSON")
	}
	return nil
}

func applySettingsJSON(path string, doc map[string]any) error {
	existing := readJSONC(path)
	for k, v := range doc {
		existing[k] = v
	}
	data := append(marshalPretty(existing), '\n')
	if err := validateJSON(data); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func diffSettingsJSON(path string, doc map[string]any) (bool, error) {
	existing := readJSONC(path)
	currentManaged := map[string]any{}
	for k := range doc {
		if v, ok := existing[k]; ok {
			currentManaged[k] = v
		}
	}

	oldData := marshalPretty(currentManaged)
	newData := marshalPretty(doc)
	if bytes.Equal(oldData, newData) {
		return false, nil
	}

	writeTemp := func(data []byte) (string, error) {
		f, err := os.CreateTemp("", "claudeconfig-diff-*")
		if err != nil {
			return "", err
		}
		_, err = f.Write(data)
		f.Close()
		return f.Name(), err
	}

	oldFile, err := writeTemp(oldData)
	if err != nil {
		return true, err
	}
	defer os.Remove(oldFile)

	newFile, err := writeTemp(newData)
	if err != nil {
		return true, err
	}
	defer os.Remove(newFile)

	label := fmt.Sprintf("%s [settings]", path)
	cmd := exec.Command("diff", "-u", "--label", label, "--label", label, oldFile, newFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	return true, nil
}

func cleanSettingsJSON(path string) error {
	existing := readJSONC(path)
	if len(existing) == 0 {
		return nil
	}
	for _, k := range managedSettingsKeys {
		delete(existing, k)
	}
	if len(existing) == 0 {
		fmt.Printf("  removed %s\n", path)
		return os.Remove(path)
	}
	fmt.Printf("  cleaned %s\n", path)
	return os.WriteFile(path, append(marshalPretty(existing), '\n'), 0644)
}

// ── File helpers ──────────────────────────────────────────────────────────────

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func ensureSymlink(linkPath, target string) error {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		return err
	}
	if rel, err := filepath.Rel(filepath.Dir(linkPath), target); err == nil {
		target = rel
	}
	existing, err := os.Readlink(linkPath)
	if err == nil && existing == target {
		return nil
	}
	os.Remove(linkPath) //nolint:errcheck
	return os.Symlink(target, linkPath)
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// ── Command file generation ───────────────────────────────────────────────────

func genCommandContent(cmd Command, fsys fs.FS) (string, error) {
	body := cmd.Content
	if cmd.File != "" {
		data, err := fs.ReadFile(fsys, cmd.File)
		if err != nil {
			return "", fmt.Errorf("command %s: %w", cmd.Name, err)
		}
		body = string(data)
	}
	var sb strings.Builder
	if cmd.Description != "" {
		sb.WriteString("---\n")
		fmt.Fprintf(&sb, "description: %q\n", cmd.Description)
		sb.WriteString("---\n")
	}
	sb.WriteString(strings.TrimRight(body, "\n"))
	sb.WriteString("\n")
	return sb.String(), nil
}

// ── Orchestrators ─────────────────────────────────────────────────────────────

func localPath(projectDir, rel string) string {
	if filepath.IsAbs(rel) || projectDir == "" || projectDir == "." {
		return rel
	}
	return filepath.Join(projectDir, rel)
}

func applyAll(target, projectDir string, cfg *Config, langs []string) error {
	settingsPath := filepath.Join(target, "settings.json")
	if err := applySettingsJSON(settingsPath, buildSettingsDoc(cfg)); err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	fmt.Printf("  wrote %s\n", settingsPath)

	if g := cfg.AgentsMD.Global; len(g.Sections) > 0 {
		gTarget := expandHome(g.Target)
		for _, s := range g.Sections {
			if err := applySectionMD(gTarget, s.Name, s.Content); err != nil {
				return fmt.Errorf("agents_md.global [%s]: %w", s.Name, err)
			}
		}
		fmt.Printf("  wrote %s\n", gTarget)
		if g.Symlink != "" {
			link := expandHome(g.Symlink)
			if err := ensureSymlink(link, gTarget); err != nil {
				return fmt.Errorf("agents_md.global symlink: %w", err)
			}
			fmt.Printf("  symlink %s → %s\n", link, gTarget)
		}
	}

	if l := cfg.AgentsMD.Local; len(l.Sections) > 0 {
		lTarget := localPath(projectDir, l.Target)
		for _, s := range l.Sections {
			if err := applySectionMD(lTarget, s.Name, s.Content); err != nil {
				return fmt.Errorf("agents_md.local [%s]: %w", s.Name, err)
			}
		}
		fmt.Printf("  wrote %s\n", lTarget)
		if l.Symlink != "" {
			lSymlink := localPath(projectDir, l.Symlink)
			if err := ensureSymlink(lSymlink, lTarget); err != nil {
				return fmt.Errorf("agents_md.local symlink: %w", err)
			}
			fmt.Printf("  symlink %s → %s\n", lSymlink, lTarget)
		}
	}

	if len(cfg.Commands) > 0 {
		cmdDir := filepath.Join(target, "commands")
		if err := os.MkdirAll(cmdDir, 0755); err != nil {
			return fmt.Errorf("commands dir: %w", err)
		}
		for _, cmd := range cfg.Commands {
			content, err := genCommandContent(cmd, cfg.FS)
			if err != nil {
				return err
			}
			path := filepath.Join(cmdDir, cmd.Name+".md")
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return fmt.Errorf("command %s: %w", cmd.Name, err)
			}
			fmt.Printf("  wrote %s\n", path)
		}
	}

	for _, name := range langs {
		lang, ok := cfg.AgentsMD.Languages[name]
		if !ok {
			return fmt.Errorf("unknown language: %s", name)
		}
		src := filepath.Join(cfg.Dir, lang.Source)
		dst := expandHome(lang.Target)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("language %s: copy: %w", name, err)
		}
		fmt.Printf("  copied %s → %s\n", src, dst)
		if lang.Symlink != "" {
			if err := ensureSymlink(lang.Symlink, dst); err != nil {
				return fmt.Errorf("language %s: symlink: %w", name, err)
			}
			fmt.Printf("  symlink %s → %s\n", lang.Symlink, dst)
		}
	}

	return nil
}

func diffAll(target string, cfg *Config) error {
	anyChanged := false
	report := func(changed bool, err error) error {
		if err != nil {
			return err
		}
		if changed {
			anyChanged = true
		}
		return nil
	}

	if err := report(diffSettingsJSON(filepath.Join(target, "settings.json"), buildSettingsDoc(cfg))); err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	if g := cfg.AgentsMD.Global; len(g.Sections) > 0 {
		gTarget := expandHome(g.Target)
		for _, s := range g.Sections {
			if err := report(diffSectionMD(gTarget, s.Name, s.Content)); err != nil {
				return fmt.Errorf("agents_md.global [%s]: %w", s.Name, err)
			}
		}
	}
	if l := cfg.AgentsMD.Local; len(l.Sections) > 0 {
		for _, s := range l.Sections {
			if err := report(diffSectionMD(l.Target, s.Name, s.Content)); err != nil {
				return fmt.Errorf("agents_md.local [%s]: %w", s.Name, err)
			}
		}
	}

	if !anyChanged {
		fmt.Println("No changes.")
	}
	return nil
}

func cleanAll(target string, cfg *Config) error {
	if err := cleanSettingsJSON(filepath.Join(target, "settings.json")); err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	if g := cfg.AgentsMD.Global; len(g.Sections) > 0 {
		gTarget := expandHome(g.Target)
		for _, s := range g.Sections {
			if err := cleanSectionMD(gTarget, s.Name); err != nil {
				return fmt.Errorf("agents_md.global [%s]: %w", s.Name, err)
			}
		}
	}
	if l := cfg.AgentsMD.Local; len(l.Sections) > 0 {
		for _, s := range l.Sections {
			if err := cleanSectionMD(l.Target, s.Name); err != nil {
				return fmt.Errorf("agents_md.local [%s]: %w", s.Name, err)
			}
		}
	}
	return nil
}
