package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ── Apply result ──────────────────────────────────────────────────────────────

// applyResult is returned by every apply-level operation.
// Callers own printing; nothing inside prints directly.
type applyResult struct {
	changed bool
	notes   []string // detail lines printed indented beneath the action line
}

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

func applySectionMD(path, section, content string) (applyResult, error) {
	begin := mdMarkers.begin(section)
	end := mdMarkers.end(section)
	block := begin + "\n" + strings.TrimRight(content, "\n") + "\n" + end + "\n"

	existingContent := ""
	if data, err := os.ReadFile(path); err == nil {
		existingContent = string(data)
	}

	var newContent string
	_, _, existed := sectionBounds(existingContent, begin, end)
	if ls, le, ok := sectionBounds(existingContent, begin, end); ok {
		newContent = existingContent[:ls] + block + existingContent[le:]
	} else {
		tail := existingContent
		if tail != "" && !strings.HasSuffix(tail, "\n") {
			tail += "\n"
		}
		newContent = tail + block
	}

	if newContent == existingContent {
		return applyResult{changed: false}, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return applyResult{}, err
	}
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return applyResult{}, err
	}

	verb := "added"
	if existed {
		verb = "updated"
	}
	return applyResult{changed: true, notes: []string{section + ": " + verb}}, nil
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

// ── Permissions merge helpers ─────────────────────────────────────────────────

// toStrings converts a []any (from JSON unmarshal) or []string to []string.
func toStrings(v any) []string {
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		ss := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				ss = append(ss, s)
			}
		}
		return ss
	}
	return nil
}

// unionStrings returns a∪b, preserving order (a first, then new items from b).
func unionStrings(a, b []string) []string {
	seen := make(map[string]struct{}, len(a))
	result := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		seen[s] = struct{}{}
		result = append(result, s)
	}
	for _, s := range b {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}

// mergePermissions unions the allow/deny arrays of existing and incoming.
func mergePermissions(existing, incoming map[string]any) map[string]any {
	result := make(map[string]any, len(existing))
	for k, v := range existing {
		result[k] = v
	}
	for _, field := range []string{"allow", "deny"} {
		result[field] = unionStrings(toStrings(existing[field]), toStrings(incoming[field]))
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

// settingsStats returns human-readable change notes comparing existing → merged
// for the managed keys (permissions arrays counted, other keys flagged as changed/added).
func settingsStats(existing, merged map[string]any) []string {
	var notes []string

	// permissions: report per-array delta
	ep, _ := existing["permissions"].(map[string]any)
	mp, _ := merged["permissions"].(map[string]any)
	if mp != nil {
		for _, field := range []string{"allow", "deny"} {
			oldN := len(toStrings(ep[field]))
			newN := len(toStrings(mp[field]))
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
		if !bytes.Equal(marshalPretty(existing[k]), marshalPretty(merged[k])) {
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
	existing := readJSONC(path)
	merged := applyMerge(existing, doc)
	data := append(marshalPretty(merged), '\n')
	if err := validateJSON(data); err != nil {
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
	existing := readJSONC(path)
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

	oldData := marshalPretty(currentManaged)
	newData := marshalPretty(newManaged)
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

func contractHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home+"/") {
		return "~/" + path[len(home)+1:]
	}
	return path
}

func ensureSymlink(linkPath, target string) (applyResult, error) {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		return applyResult{}, err
	}
	if rel, err := filepath.Rel(filepath.Dir(linkPath), target); err == nil {
		target = rel
	}
	existing, err := os.Readlink(linkPath)
	if err == nil && existing == target {
		return applyResult{changed: false}, nil
	}
	os.Remove(linkPath) //nolint:errcheck
	if err := os.Symlink(target, linkPath); err != nil {
		return applyResult{}, err
	}
	return applyResult{changed: true}, nil
}

func copyFile(src, dst string) (applyResult, error) {
	srcData, err := os.ReadFile(src)
	if err != nil {
		return applyResult{}, err
	}
	if dstData, err := os.ReadFile(dst); err == nil && bytes.Equal(srcData, dstData) {
		return applyResult{changed: false}, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return applyResult{}, err
	}
	if err := os.WriteFile(dst, srcData, 0644); err != nil {
		return applyResult{}, err
	}
	return applyResult{changed: true}, nil
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

// printResult prints the action line and any notes for a changed item.
// Silent for unchanged items — the summary line covers those.
func printResult(action, path string, r applyResult) {
	if !r.changed {
		return
	}
	fmt.Printf("  %s %s\n", action, path)
	for _, n := range r.notes {
		fmt.Printf("    %s\n", n)
	}
}

func applyAll(target, projectDir string, cfg *Config, langs []string) error {
	changes := 0

	// pStat records what was already present, shown in the summary.
	type pStat struct{ label, detail string }
	var pStats []pStat

	addStat := func(label, detail string) {
		if detail != "" {
			pStats = append(pStats, pStat{label, detail})
		}
	}

	// settings.json
	settingsPath := filepath.Join(target, "settings.json")
	settingsDoc := buildSettingsDoc(cfg)
	sr, err := applySettingsJSON(settingsPath, settingsDoc)
	if err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	if sr.changed {
		changes++
	} else {
		existing := readJSONC(settingsPath)
		// permissions: individual entry counts
		if p, _ := existing["permissions"].(map[string]any); p != nil {
			allow := toStrings(p["allow"])
			deny := toStrings(p["deny"])
			addStat("permissions", fmt.Sprintf("%d allow, %d deny", len(allow), len(deny)))
		}
		// other managed keys present in both doc and file
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

	// global CLAUDE.md / AGENTS.md — aggregate all sections into one file result
	if g := cfg.AgentsMD.Global; len(g.Sections) > 0 {
		gTarget := expandHome(g.Target)
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
			link := expandHome(g.Symlink)
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

	// local AGENTS.md / CLAUDE.md — same aggregation
	if l := cfg.AgentsMD.Local; len(l.Sections) > 0 {
		lTarget := localPath(projectDir, l.Target)
		lr := applyResult{}
		var presentNames []string
		for _, s := range l.Sections {
			r, err := applySectionMD(lTarget, s.Name, s.Content)
			if err != nil {
				return fmt.Errorf("agents_md.local [%s]: %w", s.Name, err)
			}
			if r.changed {
				lr.changed = true
				lr.notes = append(lr.notes, r.notes...)
			} else {
				presentNames = append(presentNames, s.Name)
			}
		}
		if lr.changed {
			changes++
		}
		printResult("wrote", lTarget, lr)
		addStat(filepath.Base(lTarget), strings.Join(presentNames, ", "))

		if l.Symlink != "" {
			lSymlink := localPath(projectDir, l.Symlink)
			slr, err := ensureSymlink(lSymlink, lTarget)
			if err != nil {
				return fmt.Errorf("agents_md.local symlink: %w", err)
			}
			if slr.changed {
				changes++
				fmt.Printf("  symlink %s → %s\n", lSymlink, lTarget)
			}
		}
	}

	// command files
	if len(cfg.Commands) > 0 {
		cmdDir := filepath.Join(target, "commands")
		if err := os.MkdirAll(cmdDir, 0755); err != nil {
			return fmt.Errorf("commands dir: %w", err)
		}
		var presentCmds []string
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
			} else {
				presentCmds = append(presentCmds, cmd.Name)
			}
			printResult("wrote", path, cr)
		}
		addStat("commands", strings.Join(presentCmds, ", "))
	}

	// language files
	for _, name := range langs {
		lang, ok := cfg.AgentsMD.Languages[name]
		if !ok {
			return fmt.Errorf("unknown language: %s", name)
		}
		src := filepath.Join(cfg.Dir, lang.Source)
		dst := expandHome(lang.Target)
		fr, err := copyFile(src, dst)
		if err != nil {
			return fmt.Errorf("language %s: copy: %w", name, err)
		}
		langChanged := false
		if fr.changed {
			changes++
			langChanged = true
			fmt.Printf("  copied %s → %s\n", src, dst)
		}
		if lang.Symlink != "" {
			slr, err := ensureSymlink(lang.Symlink, dst)
			if err != nil {
				return fmt.Errorf("language %s: symlink: %w", name, err)
			}
			if slr.changed {
				changes++
				langChanged = true
				fmt.Printf("  symlink %s → %s\n", lang.Symlink, dst)
			}
		}
		if !langChanged {
			detail := contractHome(dst)
			if lang.Symlink != "" {
				detail += " → " + lang.Symlink
			}
			addStat(name, detail)
		}
	}

	// summary
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
