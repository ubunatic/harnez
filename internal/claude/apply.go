package claude

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"ubunatic.com/harnez/internal/agy"
	"ubunatic.com/harnez/internal/codex"
	"ubunatic.com/harnez/internal/fsutil"
	"ubunatic.com/harnez/internal/jsonc"
	"ubunatic.com/harnez/internal/markdown"
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
// A nil value removes only managed, Harnez-owned settings.
func applyMerge(existing, doc map[string]any) map[string]any {
	out := make(map[string]any, len(existing)+len(doc))
	for k, v := range existing {
		out[k] = v
	}
	for k, v := range doc {
		if v == nil {
			if !isManagedSettingsKey(k) {
				continue
			}
			switch k {
			case "hooks":
				keptHooks, valid := removeHarnezHooks(out[k])
				if !valid {
					continue
				}
				if len(keptHooks) == 0 {
					delete(out, k)
				} else {
					out[k] = keptHooks
				}
			case "statusLine":
				if statusLine, ok := out[k].(map[string]any); ok {
					if command, _ := statusLine["command"].(string); isHarnezCommand(command) {
						delete(out, k)
					}
				}
			}
			continue
		}
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

func isManagedSettingsKey(key string) bool {
	return slices.Contains(managedSettingsKeys, key)
}

func isHarnezCommand(command string) bool {
	command = strings.TrimSpace(command)
	return command == "harnez" || strings.HasPrefix(command, "harnez ")
}

func removeHarnezHooks(value any) (map[string]any, bool) {
	hooks, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	keptHooks := make(map[string]any, len(hooks))
	for event, rawEntries := range hooks {
		entries, ok := rawEntries.([]any)
		if !ok {
			return nil, false
		}
		kept := make([]any, 0, len(entries))
		for _, entry := range entries {
			entryMap, ok := entry.(map[string]any)
			if !ok {
				kept = append(kept, entry)
				continue
			}
			rawHooks, ok := entryMap["hooks"].([]any)
			if !ok {
				kept = append(kept, entry)
				continue
			}
			entryCopy := make(map[string]any, len(entryMap))
			for key, value := range entryMap {
				entryCopy[key] = value
			}
			keptCommands := make([]any, 0, len(rawHooks))
			for _, rawHook := range rawHooks {
				hook, ok := rawHook.(map[string]any)
				command, commandOK := hook["command"].(string)
				if ok && commandOK && isHarnezCommand(command) {
					continue
				}
				keptCommands = append(keptCommands, rawHook)
			}
			if len(keptCommands) > 0 {
				entryCopy["hooks"] = keptCommands
				kept = append(kept, entryCopy)
			}
		}
		if len(kept) > 0 {
			keptHooks[event] = kept
		}
	}
	return keptHooks, true
}

// managedSettingsKeys are the top-level keys harnez writes to settings.json.
var managedSettingsKeys = []string{
	"model", "effortLevel", "permissions", "hooks", "env", "spinnerVerbs", "mcpServers", "statusLine",
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
func buildSettingsDoc(cfg *Config, selection Set) map[string]any {
	// Use map[string]any throughout so json.Marshal always sorts keys alphabetically,
	// matching what json.Unmarshal produces on read-back (round-trip stable).
	eventMap := map[string][]map[string]any{}
	for _, h := range cfg.Hooks {
		if !selection.HasComponent("telemetry") && isHarnezCommand(h.Command) {
			continue
		}
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
	} else if !selection.HasComponent("telemetry") {
		doc["hooks"] = nil
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
	if cfg.StatusLine && selection.HasComponent("usage") {
		// cwd-only MVP; Claude Code renders this on its own row above the
		// built-in footer badges, it cannot share that row (see issue 095).
		doc["statusLine"] = map[string]any{
			"type":    "command",
			"command": "harnez statusline",
		}
	} else if !selection.HasComponent("usage") {
		doc["statusLine"] = nil
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
		f, err := os.CreateTemp("", "harnez-diff-*")
		if err != nil {
			return "", err
		}
		_, err = f.Write(data)
		f.Close()
		return f.Name(), err
	}

	oldFile, err := writeTemp(oldData)
	if err != nil {
		return false, err
	}
	defer os.Remove(oldFile)

	newFile, err := writeTemp(newData)
	if err != nil {
		return false, err
	}
	defer os.Remove(newFile)

	label := fmt.Sprintf("%s [settings]", path)
	cmd := exec.Command("diff", "-u", "--label", label, "--label", label, oldFile, newFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("diff %s: %w", label, err)
	}
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

// systemdUnitSource/systemdUnitName locate the bundled harnez-agent-collector
// unit template (issue 082) within cfg.FS, and name it on disk.
const systemdUnitSource = "systemd/harnez-agent-collector.service"
const systemdUnitName = "harnez-agent-collector.service"

// systemdUnitExecPlaceholder is substituted in the bundled unit template
// with the absolute path to the currently running harnez binary.
const systemdUnitExecPlaceholder = "{{HARNEZ_BIN}}"

// systemdUserUnitDir resolves the standard systemd --user unit directory:
// ~/.config/systemd/user/.
func systemdUserUnitDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

// installSystemdUnit writes (or refreshes) the harnez-agent-collector
// systemd --user unit file (issue 082). It substitutes the unit's
// ExecStart with the absolute path to the currently running harnez binary
// (os.Executable()) rather than a bare `harnez` command name, because a
// systemd --user session does not reliably inherit the interactive shell's
// PATH — a bare command name would silently fail to start regardless of
// whether harnez was installed via `go install` (~/go/bin) or
// `make install-system` (/usr/local/bin).
func installSystemdUnit(fsys fs.FS) (string, applyResult, error) {
	unitDir, err := systemdUserUnitDir()
	if err != nil {
		return "", applyResult{}, err
	}
	exe, err := os.Executable()
	if err != nil {
		return "", applyResult{}, fmt.Errorf("resolve harnez binary path: %w", err)
	}
	tmpl, err := fs.ReadFile(fsys, systemdUnitSource)
	if err != nil {
		return "", applyResult{}, fmt.Errorf("read %s: %w", systemdUnitSource, err)
	}
	content := strings.ReplaceAll(string(tmpl), systemdUnitExecPlaceholder, exe)
	dst := filepath.Join(unitDir, systemdUnitName)
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return "", applyResult{}, fmt.Errorf("create %s: %w", unitDir, err)
	}
	r, err := writeFileIfChanged(dst, []byte(content))
	return dst, r, err
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
	return writeManagedDoc(dst, data)
}

func buildLangConventions(names []string, cfg *Config) string {
	var sb strings.Builder
	sb.WriteString("Adhere to the following conventions.\n\n")
	sb.WriteString("Docs in `./docs/` are managed by harnez. <!-- harnez:bundled -->\n\n")
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

type skillResourceContent struct {
	target string
	data   []byte
}

func validateSkillResourceTarget(skillName, target string) (string, error) {
	cleanTarget := path.Clean(target)
	if target == "" || path.IsAbs(target) || cleanTarget == "." || cleanTarget == ".." || strings.HasPrefix(cleanTarget, "../") {
		return "", fmt.Errorf("skill %s: resource target %q must stay inside the skill directory", skillName, target)
	}
	if cleanTarget == "SKILL.md" {
		return "", fmt.Errorf("skill %s: resource target %q is reserved for the generated skill body", skillName, target)
	}
	return filepath.FromSlash(cleanTarget), nil
}

func validateSkillResourceTargets(skill Command) ([]string, error) {
	targets := make([]string, 0, len(skill.Resources))
	for _, resource := range skill.Resources {
		if resource.Source == "" {
			return nil, fmt.Errorf("skill %s: resource source is required", skill.Name)
		}
		target, err := validateSkillResourceTarget(skill.Name, resource.Target)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	sortedTargets := append([]string(nil), targets...)
	sort.Strings(sortedTargets)
	for i := 1; i < len(sortedTargets); i++ {
		if sortedTargets[i] == sortedTargets[i-1] || strings.HasPrefix(sortedTargets[i], sortedTargets[i-1]+string(filepath.Separator)) {
			return nil, fmt.Errorf("skill %s: resource targets overlap at %q", skill.Name, sortedTargets[i])
		}
	}
	return targets, nil
}

func genSkillResources(skill Command, fsys fs.FS) ([]skillResourceContent, error) {
	targets, err := validateSkillResourceTargets(skill)
	if err != nil {
		return nil, err
	}
	resources := make([]skillResourceContent, 0, len(skill.Resources))
	for i, resource := range skill.Resources {
		data, err := fs.ReadFile(fsys, resource.Source)
		if err != nil {
			return nil, fmt.Errorf("skill %s resource %s: %w", skill.Name, resource.Source, err)
		}
		resources = append(resources, skillResourceContent{target: targets[i], data: data})
	}
	return resources, nil
}

func safeSkillPath(skillDir, relativePath string) (string, error) {
	if info, err := os.Lstat(skillDir); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("skill path %s is a symlink", skillDir)
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	current := skillDir
	for _, part := range strings.Split(relativePath, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("skill path %s is a symlink", current)
		}
	}
	return filepath.Join(skillDir, relativePath), nil
}

func primeAgentRoot(cfg *Config) string {
	return fsutil.ExpandHome(cfg.PrimeAgentTarget)
}

// sortedAgentIDs returns the keys of an agents_md.agents map in stable
// (alphabetical) order, so repeated applies iterate agent profiles in a
// deterministic sequence despite Go's randomized map iteration.
func sortedAgentIDs(agents map[string]AgentsMDTarget) []string {
	ids := make([]string, 0, len(agents))
	for id := range agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func appendUniquePath(paths []string, path string) []string {
	if path == "" {
		return paths
	}
	for _, existing := range paths {
		if existing == path {
			return paths
		}
	}
	return append(paths, path)
}

// skillDisabled reports whether skill must be actively removed rather than
// installed: rate_feedback-gated and disabled, or its `requires:` component
// isn't in the resolved selection (issue 142, generalized for issue 491 M5).
func skillDisabled(skill Command, selection Set, disableRateFeedback bool) bool {
	if skill.RateFeedback && disableRateFeedback {
		return true
	}
	for _, req := range skill.Requires {
		if !selection.HasComponent(req) {
			return true
		}
	}
	return false
}

func skillTargets(cfg *Config) []string {
	var targets []string

	if root := fsutil.ExpandHome(cfg.SkillsTarget); root != "" {
		targets = appendUniquePath(targets, root)
	} else if home, err := os.UserHomeDir(); err == nil {
		targets = appendUniquePath(targets, filepath.Join(home, ".gemini", "skills"))
	}

	targets = appendUniquePath(targets, fsutil.ExpandHome(cfg.CodexSkillsTarget))

	if root := fsutil.ExpandHome(cfg.ClaudeSkillsTarget); root != "" {
		targets = appendUniquePath(targets, root)
	} else if home, err := os.UserHomeDir(); err == nil {
		targets = appendUniquePath(targets, filepath.Join(home, ".claude", "skills"))
	}

	if root := primeAgentRoot(cfg); root != "" {
		targets = appendUniquePath(targets, filepath.Join(root, "skills"))
	}

	return targets
}

func docTargets(target string, lang Language, cfg *Config) []string {
	var targets []string
	if target != "" {
		targets = append(targets, filepath.Join(target, "docs", filepath.Base(lang.Target)))
	} else if lang.Target != "" {
		targets = append(targets, fsutil.ExpandHome(lang.Target))
	}
	if root := primeAgentRoot(cfg); root != "" && lang.Target != "" {
		targets = appendUniquePath(targets, filepath.Join(root, "docs", filepath.Base(lang.Target)))
	}
	return targets
}

// CleanUnmanagedDocs removes docs in ~/.claude/docs/ and ~/.prime/agent/docs/
// that are not present in keepDocs.
func CleanUnmanagedDocs(target string, cfg *Config, keepDocs []string) (int, error) {
	keepSet := make(map[string]struct{}, len(keepDocs))
	for _, name := range expandDocNames(cfg, keepDocs) {
		keepSet[name] = struct{}{}
	}
	removed := 0
	for name, lang := range cfg.AgentsMD.Languages {
		if _, keep := keepSet[name]; keep {
			continue
		}
		for _, dst := range docTargets(target, lang, cfg) {
			if err := os.Remove(dst); err == nil {
				removed++
				fmt.Printf("  removed %s\n", dst)
				_ = os.Remove(filepath.Dir(dst))
			} else if !os.IsNotExist(err) {
				return removed, fmt.Errorf("remove doc %s [%s]: %w", name, dst, err)
			}
		}
	}
	return removed, nil
}

func commandTargets(target string, cfg *Config) []string {
	targets := []string{filepath.Join(target, "commands")}
	if root := primeAgentRoot(cfg); root != "" {
		targets = appendUniquePath(targets, filepath.Join(root, "prompts"))
	}
	return targets
}

func decommissionedName(name string) error {
	if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return fmt.Errorf("invalid decommissioned artifact name %q", name)
	}
	return nil
}

func removeDecommissionedArtifacts(target string, cfg *Config) (int, error) {
	changes := 0
	for _, name := range cfg.Decommissioned.Commands {
		if err := decommissionedName(name); err != nil {
			return changes, err
		}
		for _, root := range commandTargets(target, cfg) {
			artifact := filepath.Join(root, name+".md")
			if err := os.Remove(artifact); err == nil {
				changes++
				fmt.Printf("  removed %s\n", artifact)
			} else if !os.IsNotExist(err) {
				return changes, fmt.Errorf("remove decommissioned command %s: %w", artifact, err)
			}
		}
	}
	for _, name := range cfg.Decommissioned.Skills {
		if err := decommissionedName(name); err != nil {
			return changes, err
		}
		for _, root := range skillTargets(cfg) {
			artifact := filepath.Join(root, name)
			if _, err := os.Stat(artifact); os.IsNotExist(err) {
				continue
			} else if err != nil {
				return changes, fmt.Errorf("inspect decommissioned skill %s: %w", artifact, err)
			}
			if err := os.RemoveAll(artifact); err != nil {
				return changes, fmt.Errorf("remove decommissioned skill %s: %w", artifact, err)
			}
			changes++
			fmt.Printf("  removed %s\n", artifact)
		}
	}
	return changes, nil
}

func gearExecutable() string {
	if exe, err := os.Executable(); err == nil && exe != "" {
		return exe
	}
	return "harnez"
}

// GearSymlinkTargets returns the list of candidate paths where the ⚙ multicall
// symlink should be provisioned for agent environments and PATH execution.
func GearSymlinkTargets(target string, cfg *Config) []string {
	var targets []string
	if target != "" {
		targets = appendUniquePath(targets, filepath.Join(target, "bin", "⚙"))
	}
	if cfg != nil {
		defaultTarget := ExpandTarget("", cfg.TargetDir)
		if target == defaultTarget {
			if home, err := os.UserHomeDir(); err == nil {
				goBin := filepath.Join(home, "go", "bin")
				if fi, err := os.Stat(goBin); err == nil && fi.IsDir() {
					targets = appendUniquePath(targets, filepath.Join(goBin, "⚙"))
				}
				localBin := filepath.Join(home, ".local", "bin")
				if fi, err := os.Stat(localBin); err == nil && fi.IsDir() {
					targets = appendUniquePath(targets, filepath.Join(localBin, "⚙"))
				}
			}
		}
		if root := primeAgentRoot(cfg); root != "" {
			targets = appendUniquePath(targets, filepath.Join(root, "bin", "⚙"))
		}
	}
	return targets
}

// BashShimContent is the guarded bash PATH-shim script (issue 271).
const BashShimContent = `#!/bin/sh
if test "$HARNEZ_INTERCEPTED" = "1"
then exec /bin/bash "$@"
fi
export HARNEZ_INTERCEPTED=1
exec harnez exec -- /bin/bash "$@"
`

// BashShimPath returns the standard location of the guarded bash shim (~/.harnez/shims/bash).
func BashShimPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".harnez", "shims", "bash")
}

// EnsureBashShim writes or refreshes the managed guarded bash shim using the
// same executable-file policy as `harnez apply`.
func EnsureBashShim(home string) (path string, changed bool, err error) {
	if home == "" {
		return "", false, errors.New("empty home directory")
	}
	path = filepath.Join(home, ".harnez", "shims", "bash")
	changed, err = fsutil.WriteExecutableIfChanged(path, []byte(BashShimContent))
	if err != nil {
		return path, false, fmt.Errorf("bash shim %s: %w", path, err)
	}
	return path, changed, nil
}

// HarnezAgyLauncherContent installs harnez-agy, which resolves agy using the
// caller's PATH before adding the guarded bash shim directory.
const HarnezAgyLauncherContent = `#!/bin/sh
set -f
meter=0
if test "${1-}" = "--meter"
then
    meter=1
    shift
fi
home=$HOME
shim_dir="$home/.harnez/shims"
old_path=${PATH-}
agy_path=
old_ifs=$IFS
IFS=:
for entry in $old_path
do
    test -n "$entry" || entry=.
    candidate="$entry/agy"
    if test -x "$candidate" && test ! -d "$candidate" && test "$candidate" != "$0"
    then
        agy_path=$candidate
        break
    fi
done
IFS=$old_ifs
if test -z "$agy_path"
then
    printf '%s\n' 'harnez-agy: agy not found on the original PATH' >&2
    exit 127
fi
if test "${old_path%%:*}" = "$shim_dir"
then
    export ANTIGRAVITY_AGENT=1
    if test "$meter" = 1
    then
        harnez_bin=$(command -v harnez 2>/dev/null || true)
        if test -n "$harnez_bin"
        then
            exec "$harnez_bin" agy-meter-run -- "$agy_path" "$@"
        fi
        printf '%s\n' 'harnez-agy: harnez not found; starting agy without metering' >&2
    fi
    exec "$agy_path" "$@"
fi
if test -n "$old_path"
then
    export PATH="$shim_dir:$old_path"
    export ANTIGRAVITY_AGENT=1
fi
if test -z "$old_path"
then
    export PATH="$shim_dir"
    export ANTIGRAVITY_AGENT=1
fi
if test "$meter" = 1
then
    harnez_bin=$(command -v harnez 2>/dev/null || true)
    if test -n "$harnez_bin"
    then
        exec "$harnez_bin" agy-meter-run -- "$agy_path" "$@"
    fi
    printf '%s\n' 'harnez-agy: harnez not found; starting agy without metering' >&2
fi
exec "$agy_path" "$@"
`

// HarnezAgyLauncherPath returns the managed harnez-agy launcher path.
func HarnezAgyLauncherPath(home string) string {
	return filepath.Join(home, ".local", "bin", "harnez-agy")
}

// EnsureHarnezAgyLauncher installs the managed harnez-agy launcher.
func EnsureHarnezAgyLauncher(home string) (string, bool, error) {
	if home == "" {
		return "", false, errors.New("empty home directory")
	}
	path := HarnezAgyLauncherPath(home)
	changed, err := fsutil.WriteExecutableIfChanged(path, []byte(HarnezAgyLauncherContent))
	if err != nil {
		return path, false, fmt.Errorf("harnez-agy launcher %s: %w", path, err)
	}
	return path, changed, nil
}

const HarnezEnvContent = `# harnez:begin env
# Shell environment and helper functions for harnez-managed tools and agents.

# Antigravity (AGY) wrapper with guarded shims PATH and agent indicator
agy() {
    PATH="$HOME/.harnez/shims:$PATH" ANTIGRAVITY_AGENT=1 command agy "$@"
}
# harnez:end env
`

const HarnezShellRCSnippet = `if test -f "$HOME/.harnez/env.sh"
then source "$HOME/.harnez/env.sh"
fi
`

// HarnezEnvPath returns the standard location of the harnez environment script (~/.harnez/env.sh).
func HarnezEnvPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".harnez", "env.sh")
}

// ShellRCPaths returns potential shell rc targets (~/.bashrc, ~/.zshrc) that exist,
// or ~/.bashrc if neither exists.
func ShellRCPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	candidates := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
	}
	var existing []string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			existing = append(existing, p)
		}
	}
	if len(existing) == 0 {
		return []string{candidates[0]}
	}
	return existing
}

// SwitchDocVariant re-installs a single already project-installed doc through
// the same installDoc/MergeManagedDoc path init.go uses, swapping which
// source variant supplies the managed content while preserving any
// harnez:stop-delimited local section. It touches nothing else — no other
// doc, no Makefile, no AGENTS.md section. name must be a valid entry in
// cfg.AgentsMD.Languages with Local set; variant must be "lite" or "full".
func SwitchDocVariant(dir string, cfg *Config, name, variant string) (changed bool, err error) {
	if variant != "lite" && variant != "full" {
		return false, fmt.Errorf("invalid variant %q: must be lite or full", variant)
	}
	lang, ok := cfg.AgentsMD.Languages[name]
	if !ok {
		return false, fmt.Errorf("unknown doc: %s", name)
	}
	if lang.Local == "" {
		return false, fmt.Errorf("doc %q has no project-local target (local: unset)", name)
	}
	resolveVariant := variant
	if variant == "full" {
		resolveVariant = ""
	}
	if variant == "lite" && lang.LiteSource == "" {
		return false, fmt.Errorf("doc %q has no lite_source configured; nothing to switch to", name)
	}
	dst := localPath(dir, lang.Local)
	r, err := installDoc(cfg.FS, lang.SourceFor(resolveVariant), dst, true)
	return r.changed, err
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

// mergeDocs returns the union of config-declared docs and CLI --doc flags,
// preserving order (config first, then any extras from the flag).
func mergeDocs(fromConfig, fromFlag []string) []string {
	return jsonc.UnionStrings(fromConfig, fromFlag)
}

// ApplyAll applies configuration. installSystemd additionally installs the
// harnez-agent-collector systemd --user unit (issue 082) to
// ~/.config/systemd/user/. installShell (opt-in) additionally injects the
// harnez environment source block into shell rc files (~/.bashrc, ~/.zshrc).
// ApplyAll installs the managed configuration into target, always using the
// full doc source. See ApplyAllVariant to select a lite_source variant.
func ApplyAll(target string, cfg *Config, docs []string, forceDocs bool, installSystemd bool, installShell ...bool) error {
	return ApplyAllVariant(target, cfg, nil, docs, forceDocs, installSystemd, "", installShell...)
}

// ApplyAllVariant is ApplyAll with an explicit doc variant ("" or "lite")
// selecting which source (Language.SourceFor) is installed for docs that
// declare a lite_source. selection is the resolved component set for this
// apply operation (nil = full, issue 491 M6); cfg is passed unfiltered —
// selection narrows what gets installed and drives the active-removal
// branches (Codex/AGY hooks, distill adapters, skills with unmet requires)
// entirely inside this package now.
func ApplyAllVariant(target string, cfg *Config, selection Set, docs []string, forceDocs bool, installSystemd bool, docVariant string, installShell ...bool) error {
	shellOpt := len(installShell) > 0 && installShell[0]
	docs = expandDocNames(cfg, docs)
	if err := validateDocNames(cfg, docs); err != nil {
		return err
	}
	changes := 0
	removed, err := removeDecommissionedArtifacts(target, cfg)
	if err != nil {
		return err
	}
	changes += removed

	type pStat struct{ label, detail string }
	var pStats []pStat

	addStat := func(label, detail string) {
		if detail != "" {
			pStats = append(pStats, pStat{label, detail})
		}
	}

	settingsPath := filepath.Join(target, "settings.json")
	settingsDoc := buildSettingsDoc(cfg, selection)
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

	disableRateFeedback := RateFeedbackDisabled(cfg, nil)

	// issue 491 M6: with cfg unfiltered, the "skills" component itself (as
	// opposed to a skill's own `requires:`) only gates installing new
	// content — skip semantics, matching a disabled skill's own requires
	// check below, which still actively removes it regardless of this gate.
	if len(cfg.Commands) > 0 && selection.HasComponent("skills") {
		cmdDirs := commandTargets(target, cfg)
		for _, cmdDir := range cmdDirs {
			if err := os.MkdirAll(cmdDir, 0755); err != nil {
				return fmt.Errorf("commands dir %s: %w", cmdDir, err)
			}
		}
		var cmdNames []string
		for _, cmd := range cfg.Commands {
			content, err := genCommandContent(cmd, cfg.FS)
			if err != nil {
				return err
			}
			for _, cmdDir := range cmdDirs {
				path := filepath.Join(cmdDir, cmd.Name+".md")
				cr, err := writeFileIfChanged(path, []byte(content))
				if err != nil {
					return fmt.Errorf("command %s: %w", cmd.Name, err)
				}
				if cr.changed {
					changes++
				}
				printResult("wrote", path, cr)
			}
			cmdNames = append(cmdNames, cmd.Name)
		}
		addStat("commands", strings.Join(cmdNames, ", "))
	}

	if len(cfg.Skills) > 0 {
		targets := skillTargets(cfg)
		var skillNames []string
		for _, skill := range cfg.Skills {
			// issue 142/491: a skill that is rate_feedback-gated and disabled,
			// or whose `requires:` component isn't selected, is actively
			// removed (not merely skipped) — same rationale as the
			// section-clean branch above.
			if skillDisabled(skill, selection, disableRateFeedback) {
				for _, skillsRoot := range targets {
					skillDir := filepath.Join(skillsRoot, skill.Name)
					path, err := safeSkillPath(skillDir, "SKILL.md")
					if err != nil {
						return err
					}
					if _, err := os.Stat(path); err == nil {
						if err := os.Remove(path); err != nil {
							return fmt.Errorf("skill %s [%s]: %w", skill.Name, path, err)
						}
						changes++
						fmt.Printf("  removed %s\n", path)
						_ = os.Remove(skillDir) // best-effort: drop now-empty dir
					} else if !os.IsNotExist(err) {
						return fmt.Errorf("skill %s [%s]: %w", skill.Name, path, err)
					}
					resourceTargets, err := validateSkillResourceTargets(skill)
					if err != nil {
						return err
					}
					for _, resourceTarget := range resourceTargets {
						resourcePath, err := safeSkillPath(skillDir, resourceTarget)
						if err != nil {
							return err
						}
						if err := os.Remove(resourcePath); err != nil && !os.IsNotExist(err) {
							return fmt.Errorf("skill %s resource [%s]: %w", skill.Name, resourcePath, err)
						}
						_ = os.Remove(filepath.Dir(resourcePath))
					}
					_ = os.Remove(skillDir)
				}
				skillNames = append(skillNames, skill.Name+" (disabled)")
				continue
			}
			if !selection.HasComponent("skills") {
				// skip semantics: skills component off, but this skill's own
				// requires: (if any) are met, so an existing install is left
				// untouched rather than updated or removed.
				continue
			}
			content, err := genSkillContent(skill, cfg.FS)
			if err != nil {
				return err
			}
			resources, err := genSkillResources(skill, cfg.FS)
			if err != nil {
				return err
			}
			for _, skillsRoot := range targets {
				skillDir := filepath.Join(skillsRoot, skill.Name)
				path, err := safeSkillPath(skillDir, "SKILL.md")
				if err != nil {
					return err
				}
				if err := os.MkdirAll(skillDir, 0755); err != nil {
					return fmt.Errorf("skill %s dir: %w", skill.Name, err)
				}
				oldContent, _ := os.ReadFile(path)
				cr := applyResult{changed: string(oldContent) != content}
				if cr.changed {
					if err := os.WriteFile(path, []byte(content), 0644); err != nil {
						return fmt.Errorf("skill %s: %w", skill.Name, err)
					}
					changes++
				}
				printResult("wrote", path, cr)
				for _, resource := range resources {
					resourcePath, err := safeSkillPath(skillDir, resource.target)
					if err != nil {
						return err
					}
					if err := os.MkdirAll(filepath.Dir(resourcePath), 0755); err != nil {
						return fmt.Errorf("skill %s resource dir: %w", skill.Name, err)
					}
					rr, err := writeFileIfChanged(resourcePath, resource.data)
					if err != nil {
						return fmt.Errorf("skill %s resource: %w", skill.Name, err)
					}
					if rr.changed {
						changes++
					}
					printResult("wrote", resourcePath, rr)
				}
			}
			skillNames = append(skillNames, skill.Name)
		}
		addStat("skills", strings.Join(skillNames, ", "))
	}

	// Codex's own native hooks.json-equivalent (config.toml's [hooks.<name>]
	// tables) is a distinct config format from Claude's settings.json, so it
	// isn't folded into buildSettingsDoc/applySettingsJSON above — it gets
	// its own merge (internal/codex.Apply, preserving unrelated hooks/keys
	// the same way applySettingsJSON preserves unmanaged settings keys). See
	// issues/199 (research) and issues/200 (this wiring, originally a
	// standalone `harnez codex-hooks apply` command, folded into `apply`
	// per user direction rather than staying a separate command).
	// issue 491 M5: Codex/AGY hooks are installed only under the telemetry
	// component; without it they're actively removed (ownership-aware,
	// 02c9a0b), not merely skipped, so a selected apply that drops telemetry
	// leaves no dangling harnez-owned hook behind.
	if cfg.CodexHooksTarget != "" {
		hooksPath := fsutil.ExpandHome(cfg.CodexHooksTarget)
		if selection.HasComponent("telemetry") {
			changed, err := codex.Apply(hooksPath)
			if err != nil {
				return fmt.Errorf("codex hooks: %w", err)
			}
			if changed {
				changes++
				fmt.Printf("  wrote %s\n", hooksPath)
				fmt.Println("  note: Codex will prompt for hook-trust review before this hook becomes active (see its /hooks panel).")
			}
			addStat("codex hooks", codex.Summary(hooksPath))
		} else {
			changed, err := codex.Remove(hooksPath)
			if err != nil {
				return fmt.Errorf("codex hooks: %w", err)
			}
			if changed {
				changes++
				fmt.Printf("  removed %s\n", hooksPath)
			}
		}
	}

	// Antigravity native tool observation hook (issue 373):
	// Install/update wildcard PreToolUse hook in ~/.gemini/config/hooks.json if ~/.gemini exists.
	if cfg.AgyHooksTarget != "" {
		hooksPath := fsutil.ExpandHome(cfg.AgyHooksTarget)
		agentHome := filepath.Dir(filepath.Dir(hooksPath))
		if _, err := os.Stat(agentHome); err == nil {
			if selection.HasComponent("telemetry") {
				changed, err := agy.Apply(hooksPath)
				if err != nil {
					return fmt.Errorf("agy hooks: %w", err)
				}
				if changed {
					changes++
					fmt.Printf("  wrote %s\n", hooksPath)
				}
				addStat("agy hooks", agy.Summary(hooksPath))
			} else {
				changed, err := agy.Remove(hooksPath)
				if err != nil {
					return fmt.Errorf("agy hooks: %w", err)
				}
				if changed {
					changes++
					fmt.Printf("  removed %s\n", hooksPath)
				}
			}
		}
	}

	// issue 491 M6: like Codex/AGY hooks above, distill adapters are only
	// installed under the telemetry component; without it they're actively
	// removed, since cfg is unfiltered here (selection decides, not Filter).
	if adapters := distillAdapters(cfg); len(adapters) > 0 {
		if selection.HasComponent("telemetry") {
			for _, adapter := range adapters {
				ar, err := writeFileIfChanged(adapter.path, []byte(adapter.content))
				if err != nil {
					return fmt.Errorf("%s %s: %w", adapter.label, adapter.path, err)
				}
				if ar.changed {
					changes++
				}
				printResult("wrote", adapter.path, ar)
			}
			addStat("distill", distillAdapterSummary(cfg))
		} else {
			for _, adapter := range adapters {
				if err := os.Remove(adapter.path); err == nil {
					changes++
					fmt.Printf("  removed %s\n", adapter.path)
				} else if !os.IsNotExist(err) {
					return fmt.Errorf("%s %s: %w", adapter.label, adapter.path, err)
				}
			}
		}
	}

	docNames := docs
	configDocs := cfg.Docs
	if !selection.HasComponent("docs") {
		docNames = nil
		configDocs = nil
	}
	globalDocs := mergeDocs(configDocs, docNames)
	for _, name := range globalDocs {
		lang, ok := cfg.AgentsMD.Languages[name]
		if !ok {
			return fmt.Errorf("unknown doc: %s", name)
		}
		for _, dst := range docTargets(target, lang, cfg) {
			fr, err := installDoc(cfg.FS, lang.SourceFor(docVariant), dst, forceDocs)
			if err != nil {
				return fmt.Errorf("language %s: install %s: %w", name, dst, err)
			}
			if fr.changed {
				changes++
				fmt.Printf("  installed %s\n", dst)
			} else {
				state := langDocState(cfg.FS, lang.SourceFor(docVariant), dst)
				addStat(name, fsutil.ContractHome(dst)+" ["+state+"]")
			}
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		shimPath, changed, err := EnsureBashShim(home)
		if err != nil {
			return err
		}
		if changed {
			changes++
			fmt.Printf("  wrote %s\n", shimPath)
		} else {
			addStat("bash shim", fsutil.ContractHome(shimPath))
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		launcherPath, changed, err := EnsureHarnezAgyLauncher(home)
		if err != nil {
			return err
		}
		if changed {
			changes++
			fmt.Printf("  wrote %s\n", launcherPath)
		} else {
			addStat("harnez-agy launcher", fsutil.ContractHome(launcherPath))
		}
	}

	gearExe := gearExecutable()
	var gearStats []string
	for _, link := range GearSymlinkTargets(target, cfg) {
		lr, err := ensureSymlink(link, gearExe)
		if err != nil {
			return fmt.Errorf("gear symlink %s: %w", link, err)
		}
		if lr.changed {
			changes++
			fmt.Printf("  symlink %s → %s\n", link, gearExe)
		} else {
			gearStats = append(gearStats, fsutil.ContractHome(link))
		}
	}
	if len(gearStats) > 0 {
		addStat("⚙ symlink", strings.Join(gearStats, ", "))
	}

	if envPath := HarnezEnvPath(); envPath != "" {
		changed, err := fsutil.WriteIfChanged(envPath, []byte(HarnezEnvContent))
		if err != nil {
			return fmt.Errorf("harnez env %s: %w", envPath, err)
		}
		if changed {
			changes++
			fmt.Printf("  wrote %s\n", envPath)
		} else {
			addStat("env script", fsutil.ContractHome(envPath))
		}
	}

	if shellOpt {
		for _, rcPath := range ShellRCPaths() {
			changed, existed, err := markdown.ApplyMK(rcPath, "env", HarnezShellRCSnippet)
			if err != nil {
				return fmt.Errorf("shell rc %s: %w", rcPath, err)
			}
			if changed {
				changes++
				if existed {
					fmt.Printf("  updated %s [env]\n", rcPath)
				} else {
					fmt.Printf("  wrote %s [env]\n", rcPath)
				}
			} else {
				addStat("shell rc", fsutil.ContractHome(rcPath))
			}
		}
	}

	if installSystemd {
		unitPath, ur, err := installSystemdUnit(cfg.FS)
		if err != nil {
			return fmt.Errorf("systemd unit: %w", err)
		}
		if ur.changed {
			changes++
		}
		printResult("wrote", unitPath, ur)
		if ur.changed {
			fmt.Println("  enable with: systemctl --user enable --now harnez-agent-collector.service")
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

// DiffAll diffs the config and reports whether changes/drift were detected.
func DiffAll(target string, cfg *Config, selection Set) (bool, error) {
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

	if err := report(diffSettingsJSON(filepath.Join(target, "settings.json"), buildSettingsDoc(cfg, selection))); err != nil {
		return false, fmt.Errorf("settings: %w", err)
	}
	disableRateFeedback := RateFeedbackDisabled(cfg, nil)

	if len(cfg.Skills) > 0 {
		for _, skill := range cfg.Skills {
			if skillDisabled(skill, selection, disableRateFeedback) {
				for _, skillsRoot := range skillTargets(cfg) {
					path := filepath.Join(skillsRoot, skill.Name, "SKILL.md")
					if _, err := os.Stat(path); err == nil {
						anyChanged = true
					} else if !os.IsNotExist(err) {
						return false, fmt.Errorf("skill %s [%s]: %w", skill.Name, path, err)
					}
				}
				continue
			}
			if !selection.HasComponent("skills") {
				continue // skip semantics: not managed, so never drifted
			}
			content, err := genSkillContent(skill, cfg.FS)
			if err != nil {
				return false, err
			}
			resources, err := genSkillResources(skill, cfg.FS)
			if err != nil {
				return false, err
			}
			for _, skillsRoot := range skillTargets(cfg) {
				skillDir := filepath.Join(skillsRoot, skill.Name)
				path, err := safeSkillPath(skillDir, "SKILL.md")
				if err != nil {
					return false, err
				}
				existing, readErr := os.ReadFile(path)
				if readErr != nil {
					if os.IsNotExist(readErr) {
						anyChanged = true
						continue
					}
					return false, fmt.Errorf("skill %s [%s]: %w", skill.Name, path, readErr)
				}
				if string(existing) != content {
					anyChanged = true
				}
				for _, resource := range resources {
					resourcePath, err := safeSkillPath(skillDir, resource.target)
					if err != nil {
						return false, err
					}
					existing, readErr := os.ReadFile(resourcePath)
					if readErr != nil {
						if os.IsNotExist(readErr) {
							anyChanged = true
							continue
						}
						return false, fmt.Errorf("skill %s resource [%s]: %w", skill.Name, resourcePath, readErr)
					}
					if !bytes.Equal(existing, resource.data) {
						anyChanged = true
					}
				}
			}
		}
	}
	// issue 491 M5: check Codex/AGY under the resolved selection directly
	// rather than skipping them because a filtered config blanked their
	// targets — without telemetry, drift means a stale hook still installed.
	if cfg.CodexHooksTarget != "" {
		hooksPath := fsutil.ExpandHome(cfg.CodexHooksTarget)
		installed, drifted := codex.Status(hooksPath)
		if selection.HasComponent("telemetry") {
			if !installed || drifted {
				anyChanged = true
			}
		} else if installed {
			anyChanged = true
		}
	}
	if cfg.AgyHooksTarget != "" {
		hooksPath := fsutil.ExpandHome(cfg.AgyHooksTarget)
		agentHome := filepath.Dir(filepath.Dir(hooksPath))
		if _, err := os.Stat(agentHome); err == nil {
			installed, drifted := agy.Status(hooksPath)
			if selection.HasComponent("telemetry") {
				if !installed || drifted {
					anyChanged = true
				}
			} else if installed {
				anyChanged = true
			}
		}
	}

	if shimPath := BashShimPath(); shimPath != "" {
		if selection.HasComponent("telemetry") {
			if fi, err := os.Stat(shimPath); err != nil {
				anyChanged = true
			} else {
				data, readErr := os.ReadFile(shimPath)
				if readErr != nil || string(data) != BashShimContent || fi.Mode().Perm() != 0755 {
					anyChanged = true
				}
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		launcherPath := HarnezAgyLauncherPath(home)
		if selection.HasComponent("telemetry") {
			if fi, err := os.Stat(launcherPath); err != nil {
				anyChanged = true
			} else {
				data, readErr := os.ReadFile(launcherPath)
				if readErr != nil || string(data) != HarnezAgyLauncherContent || fi.Mode().Perm() != 0755 {
					anyChanged = true
				}
			}
		}
	}

	if envPath := HarnezEnvPath(); envPath != "" {
		if _, err := os.Stat(envPath); err != nil {
			anyChanged = true
		} else {
			data, readErr := os.ReadFile(envPath)
			if readErr != nil || string(data) != HarnezEnvContent {
				anyChanged = true
			}
		}
	}
	for _, rcPath := range ShellRCPaths() {
		if markdown.ContainsSectionMK(rcPath, "env") {
			if changed, err := markdown.DiffMK(rcPath, "env", HarnezShellRCSnippet); err != nil || changed {
				anyChanged = true
			}
		}
	}

	gearExe := gearExecutable()
	for _, link := range GearSymlinkTargets(target, cfg) {
		if _, err := os.Lstat(link); err != nil {
			anyChanged = true
		} else if targetDst, err := os.Readlink(link); err != nil {
			anyChanged = true
		} else {
			expected := gearExe
			if rel, err := filepath.Rel(filepath.Dir(link), gearExe); err == nil {
				expected = rel
			}
			if targetDst != expected && targetDst != gearExe {
				anyChanged = true
			}
		}
	}

	docsForDiff := cfg.Docs
	if !selection.HasComponent("docs") {
		docsForDiff = nil
	}
	for _, name := range docsForDiff {
		lang, ok := cfg.AgentsMD.Languages[name]
		if !ok {
			continue
		}
		for _, dst := range docTargets(target, lang, cfg) {
			state := langDocState(cfg.FS, lang.Source, dst)
			if state != "bundled" {
				anyChanged = true
			}
		}
	}

	if !anyChanged {
		fmt.Println("No changes.")
	}
	return anyChanged, nil
}

// CleanAll cleans the config.
func CleanAll(target string, cfg *Config) error {
	if err := cleanSettingsJSON(filepath.Join(target, "settings.json")); err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	for _, lang := range cfg.AgentsMD.Languages {
		for _, dst := range docTargets(target, lang, cfg) {
			if err := os.Remove(dst); err == nil {
				fmt.Printf("  removed %s\n", dst)
				_ = os.Remove(filepath.Dir(dst))
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
	if len(cfg.Skills) > 0 {
		for _, skill := range cfg.Skills {
			for _, skillsRoot := range skillTargets(cfg) {
				skillDir := filepath.Join(skillsRoot, skill.Name)
				path, err := safeSkillPath(skillDir, "SKILL.md")
				if err != nil {
					return err
				}
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("skill %s [%s]: %w", skill.Name, path, err)
				}
				resourceTargets, err := validateSkillResourceTargets(skill)
				if err != nil {
					return err
				}
				for _, resourceTarget := range resourceTargets {
					resourcePath, err := safeSkillPath(skillDir, resourceTarget)
					if err != nil {
						return err
					}
					if err := os.Remove(resourcePath); err != nil && !os.IsNotExist(err) {
						return fmt.Errorf("skill %s resource [%s]: %w", skill.Name, resourcePath, err)
					}
					_ = os.Remove(filepath.Dir(resourcePath))
				}
				// Best-effort: drop the now-empty per-skill directory. Ignore
				// errors (e.g. directory holds other files) — never clean
				// content harnez didn't write.
				_ = os.Remove(skillDir)
			}
		}
	}
	if cfg.CodexHooksTarget != "" {
		hooksPath := fsutil.ExpandHome(cfg.CodexHooksTarget)
		if _, err := codex.Remove(hooksPath); err != nil {
			return fmt.Errorf("codex hooks [%s]: %w", hooksPath, err)
		}
	}
	if cfg.AgyHooksTarget != "" {
		hooksPath := fsutil.ExpandHome(cfg.AgyHooksTarget)
		if _, err := agy.Remove(hooksPath); err != nil {
			return fmt.Errorf("agy hooks [%s]: %w", hooksPath, err)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		launcherPath := HarnezAgyLauncherPath(home)
		if err := os.Remove(launcherPath); err == nil {
			fmt.Printf("  removed %s\n", launcherPath)
		}
	}
	if shimPath := BashShimPath(); shimPath != "" {
		if err := os.Remove(shimPath); err == nil {
			fmt.Printf("  removed %s\n", shimPath)
			_ = os.Remove(filepath.Dir(shimPath))
		}
	}
	if envPath := HarnezEnvPath(); envPath != "" {
		if err := os.Remove(envPath); err == nil {
			fmt.Printf("  removed %s\n", envPath)
			_ = os.Remove(filepath.Dir(envPath))
		}
	}
	for _, rcPath := range ShellRCPaths() {
		if removed, _, err := markdown.CleanMK(rcPath, "env"); err == nil && removed {
			fmt.Printf("  cleaned %s [env]\n", rcPath)
		}
	}
	for _, link := range GearSymlinkTargets(target, cfg) {
		if err := os.Remove(link); err == nil {
			fmt.Printf("  removed %s\n", link)
			_ = os.Remove(filepath.Dir(link)) // best-effort clean empty bin dir
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
