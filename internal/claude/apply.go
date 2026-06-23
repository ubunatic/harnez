package claude

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ubunatic.com/claudeconfig/internal/fsutil"
	"ubunatic.com/claudeconfig/internal/jsonc"
	"ubunatic.com/claudeconfig/internal/markdown"
)

// applyResult is returned by every apply-level operation.
// Callers own printing; nothing inside prints directly.
type applyResult struct {
	changed bool
	notes   []string // detail lines printed indented beneath the action line
}

// applySectionMD updates or inserts a managed section inside a markdown file.
func applySectionMD(path, section, content string) (applyResult, error) {
	changed, existed, err := markdown.Apply(path, section, content)
	if err != nil {
		return applyResult{}, err
	}
	var notes []string
	if changed {
		verb := "added"
		if existed {
			verb = "updated"
		}
		notes = append(notes, section+": "+verb)
	}
	return applyResult{changed: changed, notes: notes}, nil
}

// diffSectionMD prints unified diff of a managed markdown section.
func diffSectionMD(path, section, content string) (bool, error) {
	return markdown.Diff(path, section, content)
}

// cleanSectionMD cleans/removes a managed section and prints status.
func cleanSectionMD(path, section string) error {
	removed, cleaned, err := markdown.Clean(path, section)
	if err != nil {
		return err
	}
	if removed {
		fmt.Printf("  removed %s\n", path)
	} else if cleaned {
		fmt.Printf("  cleaned %s\n", path)
	}
	return nil
}

// mergePermissions unions the allow/deny arrays of existing and incoming.
func mergePermissions(existing, incoming map[string]any) map[string]any {
	result := make(map[string]any, len(existing))
	for k, v := range existing {
		result[k] = v
	}
	for _, field := range []string{"allow", "deny"} {
		result[field] = jsonc.UnionStrings(jsonc.ToStrings(existing[field]), jsonc.ToStrings(incoming[field]))
	}
	return result
}

// applyMerge merges doc into a copy of existing. The permissions.allow and
// permissions.deny arrays are union-merged; all other top-level keys replace.
func applyMerge(existing, doc map[string]any) map[string]any {
	out := make(map[string]any, len(existing)+len(doc))
	for k, v := range existing {
		out[k] = v
	}
	for k, v := range doc {
		if k == "permissions" {
			if ep, ok := out[k].(map[string]any); ok {
				if ip, ok := v.(map[string]any); ok {
					out[k] = mergePermissions(ep, ip)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}

// managedSettingsKeys are the top-level keys claudeconfig writes to settings.json.
var managedSettingsKeys = []string{
	"model", "effortLevel", "permissions", "hooks", "env", "spinnerVerbs", "mcpServers",
}

// Model alias mappings.
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

// buildSettingsDoc creates the settings JSON document from the YAML config.
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

// settingsStats returns human-readable change notes comparing existing → merged
// for the managed keys (permissions arrays counted, other keys flagged as changed/added).
func settingsStats(existing, merged map[string]any) []string {
	var notes []string

	// permissions: report per-array delta
	ep, _ := existing["permissions"].(map[string]any)
	mp, _ := merged["permissions"].(map[string]any)
	if mp != nil {
		for _, field := range []string{"allow", "deny"} {
			oldN := len(jsonc.ToStrings(ep[field]))
			newN := len(jsonc.ToStrings(mp[field]))
			if newN != oldN {
				notes = append(notes, fmt.Sprintf("permissions.%s: %+d entries (%d → %d)", field, newN-oldN, oldN, newN))
			}
		}
	}

	// other managed keys
	for _, k := range managedSettingsKeys {
		if k == "permissions" {
			continue
		}
		if !bytes.Equal(jsonc.MarshalPretty(existing[k]), jsonc.MarshalPretty(merged[k])) {
			if existing[k] == nil {
				notes = append(notes, k+": added")
			} else {
				notes = append(notes, k+": changed")
			}
		}
	}
	return notes
}

func applySettingsJSON(path string, doc map[string]any) (applyResult, error) {
	existing := jsonc.Read(path)
	merged := applyMerge(existing, doc)
	data := append(jsonc.MarshalPretty(merged), '\n')
	if err := jsonc.Validate(data); err != nil {
		return applyResult{}, fmt.Errorf("%s: %w", path, err)
	}

	oldData, _ := os.ReadFile(path)
	if bytes.Equal(oldData, data) {
		return applyResult{changed: false}, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return applyResult{}, err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return applyResult{}, err
	}
	return applyResult{changed: true, notes: settingsStats(existing, merged)}, nil
}

func diffSettingsJSON(path string, doc map[string]any) (bool, error) {
	existing := jsonc.Read(path)
	merged := applyMerge(existing, doc)

	currentManaged := map[string]any{}
	for k := range doc {
		if v, ok := existing[k]; ok {
			currentManaged[k] = v
		}
	}
	newManaged := map[string]any{}
	for k := range doc {
		if v, ok := merged[k]; ok {
			newManaged[k] = v
		}
	}

	oldData := jsonc.MarshalPretty(currentManaged)
	newData := jsonc.MarshalPretty(newManaged)
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
	existing := jsonc.Read(path)
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
	return os.WriteFile(path, append(jsonc.MarshalPretty(existing), '\n'), 0644)
}

func ensureSymlink(linkPath, target string) (applyResult, error) {
	changed, err := fsutil.EnsureSymlink(linkPath, target)
	if err != nil {
		return applyResult{}, err
	}
	return applyResult{changed: changed}, nil
}

func writeFileIfChanged(dst string, data []byte) (applyResult, error) {
	changed, err := fsutil.WriteIfChanged(dst, data)
	if err != nil {
		return applyResult{}, err
	}
	return applyResult{changed: changed}, nil
}

func copyFile(src, dst string) (applyResult, error) {
	changed, err := fsutil.Copy(src, dst)
	if err != nil {
		return applyResult{}, err
	}
	return applyResult{changed: changed}, nil
}

func installDoc(fsys fs.FS, src, dst string, force bool) (applyResult, error) {
	if !force {
		if _, err := os.Stat(dst); err == nil {
			return applyResult{changed: false}, nil
		}
	}
	data, err := fs.ReadFile(fsys, src)
	if err != nil {
		return applyResult{}, err
	}
	return writeFileIfChanged(dst, data)
}

func buildLangConventions(names []string, cfg *Config) string {
	var sb strings.Builder
	sb.WriteString("Adhere to the following conventions.\n\n")
	for _, name := range names {
		lang, ok := cfg.AgentsMD.Languages[name]
		if !ok || lang.Ref == "" {
			continue
		}
		displayName := lang.Name
		if displayName == "" {
			displayName = name
		}
		sb.WriteString("- " + displayName + " " + lang.Ref)
		if lang.Hint != "" {
			hint := strings.TrimRight(lang.Hint, "\n")
			lines := strings.Split(hint, "\n")
			sb.WriteString(",\n")
			for _, line := range lines {
				sb.WriteString("  " + line + "\n")
			}
		} else {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func langDocState(fsys fs.FS, src, dst string) string {
	installed, err := os.ReadFile(dst)
	if err != nil {
		return "not installed"
	}
	bundled, err := fs.ReadFile(fsys, src)
	if err != nil {
		return "installed"
	}
	if bytes.Equal(installed, bundled) {
		return "bundled"
	}
	return "custom"
}

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

func genSkillContent(cmd Command, fsys fs.FS) (string, error) {
	body := cmd.Content
	if cmd.File != "" {
		data, err := fs.ReadFile(fsys, cmd.File)
		if err != nil {
			return "", fmt.Errorf("skill %s: %w", cmd.Name, err)
		}
		body = string(data)
	}
	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "name: %q\n", cmd.Name)
	if cmd.Description != "" {
		fmt.Fprintf(&sb, "description: %q\n", cmd.Description)
	}
	sb.WriteString("---\n")
	sb.WriteString(strings.TrimRight(body, "\n"))
	sb.WriteString("\n")
	return sb.String(), nil
}

func localPath(projectDir, rel string) string {
	if filepath.IsAbs(rel) || projectDir == "" || projectDir == "." {
		return rel
	}
	return filepath.Join(projectDir, rel)
}

func printResult(action, path string, r applyResult) {
	if !r.changed {
		return
	}
	fmt.Printf("  %s %s\n", action, path)
	for _, n := range r.notes {
		fmt.Printf("    %s\n", n)
	}
}

// mergeLangs returns the union of config-declared langs and CLI --lang flags,
// preserving order (config first, then any extras from the flag).
func mergeLangs(fromConfig, fromFlag []string) []string {
	seen := make(map[string]struct{}, len(fromConfig)+len(fromFlag))
	result := make([]string, 0, len(fromConfig)+len(fromFlag))
	for _, l := range fromConfig {
		seen[l] = struct{}{}
		result = append(result, l)
	}
	for _, l := range fromFlag {
		if _, ok := seen[l]; !ok {
			result = append(result, l)
		}
	}
	return result
}

// ApplyAll applies configuration.
func ApplyAll(target string, cfg *Config, langs []string, forceDocs bool) error {
	changes := 0

	type pStat struct{ label, detail string }
	var pStats []pStat

	addStat := func(label, detail string) {
		if detail != "" {
			pStats = append(pStats, pStat{label, detail})
		}
	}

	settingsPath := filepath.Join(target, "settings.json")
	settingsDoc := buildSettingsDoc(cfg)
	sr, err := applySettingsJSON(settingsPath, settingsDoc)
	if err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	if sr.changed {
		changes++
	} else {
		existing := jsonc.Read(settingsPath)
		if p, _ := existing["permissions"].(map[string]any); p != nil {
			allow := jsonc.ToStrings(p["allow"])
			deny := jsonc.ToStrings(p["deny"])
			addStat("permissions", fmt.Sprintf("%d allow, %d deny", len(allow), len(deny)))
		}
		var keys []string
		for _, k := range managedSettingsKeys {
			if k == "permissions" {
				continue
			}
			if _, inDoc := settingsDoc[k]; inDoc {
				if _, inFile := existing[k]; inFile {
					keys = append(keys, k)
				}
			}
		}
		addStat("settings", strings.Join(keys, ", "))
	}
	printResult("wrote", settingsPath, sr)

	if g := cfg.AgentsMD.Global; len(g.Sections) > 0 {
		gTarget := fsutil.ExpandHome(g.Target)
		gr := applyResult{}
		var presentNames []string
		for _, s := range g.Sections {
			r, err := applySectionMD(gTarget, s.Name, s.Content)
			if err != nil {
				return fmt.Errorf("agents_md.global [%s]: %w", s.Name, err)
			}
			if r.changed {
				gr.changed = true
				gr.notes = append(gr.notes, r.notes...)
			} else {
				presentNames = append(presentNames, s.Name)
			}
		}
		if gr.changed {
			changes++
		}
		printResult("wrote", gTarget, gr)
		addStat(filepath.Base(gTarget), strings.Join(presentNames, ", "))

		if g.Symlink != "" {
			link := fsutil.ExpandHome(g.Symlink)
			lr, err := ensureSymlink(link, gTarget)
			if err != nil {
				return fmt.Errorf("agents_md.global symlink: %w", err)
			}
			if lr.changed {
				changes++
				fmt.Printf("  symlink %s → %s\n", link, gTarget)
			}
		}
	}

	if len(cfg.Commands) > 0 {
		cmdDir := filepath.Join(target, "commands")
		if err := os.MkdirAll(cmdDir, 0755); err != nil {
			return fmt.Errorf("commands dir: %w", err)
		}
		var cmdNames []string
		for _, cmd := range cfg.Commands {
			content, err := genCommandContent(cmd, cfg.FS)
			if err != nil {
				return err
			}
			path := filepath.Join(cmdDir, cmd.Name+".md")
			oldContent, _ := os.ReadFile(path)
			cr := applyResult{changed: string(oldContent) != content}
			if cr.changed {
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					return fmt.Errorf("command %s: %w", cmd.Name, err)
				}
				changes++
			}
			cmdNames = append(cmdNames, cmd.Name)
			printResult("wrote", path, cr)
		}
		addStat("commands", strings.Join(cmdNames, ", "))
	}

	if len(cfg.Skills) > 0 {
		skillsRoot := fsutil.ExpandHome(cfg.SkillsTarget)
		if skillsRoot == "" {
			home, _ := os.UserHomeDir()
			skillsRoot = filepath.Join(home, ".gemini", "skills")
		}
		var skillNames []string
		for _, skill := range cfg.Skills {
			content, err := genSkillContent(skill, cfg.FS)
			if err != nil {
				return err
			}
			skillDir := filepath.Join(skillsRoot, skill.Name)
			if err := os.MkdirAll(skillDir, 0755); err != nil {
				return fmt.Errorf("skill %s dir: %w", skill.Name, err)
			}
			path := filepath.Join(skillDir, "SKILL.md")
			oldContent, _ := os.ReadFile(path)
			cr := applyResult{changed: string(oldContent) != content}
			if cr.changed {
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					return fmt.Errorf("skill %s: %w", skill.Name, err)
				}
				changes++
			}
			skillNames = append(skillNames, skill.Name)
			printResult("wrote", path, cr)
		}
		addStat("skills", strings.Join(skillNames, ", "))
	}

	globalLangs := mergeLangs(cfg.Langs, langs)
	for _, name := range globalLangs {
		lang, ok := cfg.AgentsMD.Languages[name]
		if !ok {
			return fmt.Errorf("unknown language: %s", name)
		}
		dst := fsutil.ExpandHome(lang.Target)
		fr, err := installDoc(cfg.FS, lang.Source, dst, forceDocs)
		if err != nil {
			return fmt.Errorf("language %s: install: %w", name, err)
		}
		if fr.changed {
			changes++
			fmt.Printf("  installed %s\n", dst)
		} else {
			state := langDocState(cfg.FS, lang.Source, dst)
			addStat(name, fsutil.ContractHome(dst)+" ["+state+"]")
		}
	}

	if changes == 0 {
		fmt.Println("No changes.")
	} else if changes == 1 {
		fmt.Println("1 change.")
	} else {
		fmt.Printf("%d changes.\n", changes)
	}
	for _, s := range pStats {
		fmt.Printf("  %-14s %s\n", s.label+":", s.detail)
	}
	return nil
}

// DiffAll diffs the config.
func DiffAll(target string, cfg *Config) error {
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
		gTarget := fsutil.ExpandHome(g.Target)
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

// CleanAll cleans the config.
func CleanAll(target string, cfg *Config) error {
	if err := cleanSettingsJSON(filepath.Join(target, "settings.json")); err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	if g := cfg.AgentsMD.Global; len(g.Sections) > 0 {
		gTarget := fsutil.ExpandHome(g.Target)
		for _, s := range g.Sections {
			if err := cleanSectionMD(gTarget, s.Name); err != nil {
				return fmt.Errorf("agents_md.global [%s]: %w", s.Name, err)
			}
		}
	}
	if l := cfg.AgentsMD.Local; l.Target != "" {
		sections := append([]MDSection(nil), l.Sections...)
		sections = append(sections, MDSection{Name: "Language Conventions"})
		for _, s := range sections {
			if err := cleanSectionMD(l.Target, s.Name); err != nil {
				return fmt.Errorf("agents_md.local [%s]: %w", s.Name, err)
			}
		}
	}
	return nil
}

// ExpandTarget expands targets.
func ExpandTarget(flag, cfgTarget string) string {
	if flag != "" {
		return flag
	}
	if cfgTarget != "" {
		return fsutil.ExpandHome(cfgTarget)
	}
	return DefaultTarget()
}

// OpenConfig opens target configuration.
func OpenConfig(configPath string) (*Config, string, error) {
	if configPath == "" {
		cfg, err := LoadConfigEmbedded()
		return cfg, "(embedded)", err
	}
	cfg, err := LoadConfig(configPath)
	return cfg, configPath, err
}
