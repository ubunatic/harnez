package claude

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/fsutil"
	"ubunatic.com/harnez/internal/jsonc"
	"ubunatic.com/harnez/internal/markdown"
)

var markdownRelativeLinkRE = regexp.MustCompile(`\[[^]]*\]\(([^)#]+)(?:#[^)]*)?\)`)
var unavailableProjectDocRE = regexp.MustCompile(`docs/(?:studies|feedback)/[^\s` + "`" + `)]+\.md`)
var projectDestinationRE = regexp.MustCompile(`docs/(?:studies|feedback)/`)

func markdownOutsideFences(data []byte) string {
	var b strings.Builder
	inFence := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

const summarySection = "Project Summary"

const quota1SectionName = "Quota-1 Guardrails"

const summaryPrompt = `Summarize this project for a coding agent in plain markdown.
Cover: what it does, the main components and their roles, key conventions, and anything
important to know before making changes.
Start your response with the first line of content. No preamble, no trailing commentary.`

const reviewPrompt = `Review the following project summary for an AI coding agent (AGENTS.md).
Produce a refined version that is:
- Precise and actionable — an agent can act on it directly
- Grounded in the code — no assumptions beyond what is shown
- Concise — no redundancy, no padding
- Well-structured — key conventions and entry points front and centre

Start your response with the first line of the summary. No preamble, no analysis, no trailing commentary.
Do not include HTML comment markers (<!-- ... -->) in your output.`

// sanitizeContent strips artifacts that LLMs sometimes inject.
func sanitizeContent(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if nl := strings.Index(s, "\n"); nl > 0 {
			if end := strings.LastIndex(s, "\n```"); end > nl {
				s = strings.TrimSpace(s[nl+1 : end])
			}
		}
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<!-- harnez:") || strings.HasPrefix(trimmed, "<!-- claudeconfig:") {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// stripMetaCommentary removes meta-commentary separated from the actual content by a "---" rule.
func stripMetaCommentary(s string) string {
	const sep = "\n---\n"
	idx := strings.LastIndex(s, sep)
	if idx < 0 {
		return s
	}
	before := strings.TrimSpace(s[:idx])
	after := strings.TrimSpace(s[idx+len(sep):])
	bl := strings.Count(before, "\n") + 1
	al := strings.Count(after, "\n") + 1
	switch {
	case al >= 4 && bl < 4:
		return after
	case bl >= 4 && al < 4:
		return before
	}
	return s
}

type claudeJSONResult struct {
	Result       string  `json:"result"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

func claudeP(dir, prompt string) (string, error) {
	cmd := exec.Command("claude", "-p", "--output-format", "json", prompt)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude -p: %w", err)
	}
	var res claudeJSONResult
	if err := json.Unmarshal(out, &res); err != nil {
		return string(out), nil
	}
	u := res.Usage
	parts := fmt.Sprintf("in=%d out=%d", u.InputTokens, u.OutputTokens)
	if u.CacheReadInputTokens > 0 {
		parts += fmt.Sprintf(" read=%d", u.CacheReadInputTokens)
	}
	if u.CacheCreationInputTokens > 0 {
		parts += fmt.Sprintf(" write=%d", u.CacheCreationInputTokens)
	}
	fmt.Printf("  tokens  %s  $%.4f\n", parts, res.TotalCostUSD)
	return res.Result, nil
}

func fetchSummary(dir string) (string, error) {
	return claudeP(dir, summaryPrompt)
}

func reviewSummary(dir, draft string) (string, error) {
	context := "```markdown\n" + draft + "\n```"
	return claudeP(dir, reviewPrompt+"\n\n"+context)
}

// autoDetectDocs returns doc names from cfg whose Default is "true" or "auto"
// (with a positive signal in dir), excluding any already in explicit.
// Names are returned in the config's docs: list order (issue 011: map
// iteration made the Language Conventions section non-deterministic).
func autoDetectDocs(dir string, cfg *Config, explicit []string) []string {
	inExplicit := make(map[string]struct{}, len(explicit))
	for _, name := range explicit {
		inExplicit[name] = struct{}{}
	}
	var detected []string
	for _, name := range docNamesInOrder(cfg) {
		if _, ok := inExplicit[name]; ok {
			continue
		}
		lang := cfg.AgentsMD.Languages[name]
		switch lang.Default {
		case "true":
			detected = append(detected, name)
		case "auto":
			if detectDoc(dir, name) {
				detected = append(detected, name)
			}
		}
	}
	return detected
}

// existingLangDocs returns doc names whose @docs/*.md ref already appears in
// the target file's "Language Conventions" managed section, so a plain
// re-init preserves optional (default: false) docs a prior --docs run added
// instead of silently dropping them (issue 341-followup, harnez self-init).
func existingLangDocs(agentsPath string, cfg *Config) []string {
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		return nil
	}
	content := string(data)
	_, _, ok := markdown.SectionBounds(content,
		markdown.MDMarkers.Begin("Language Conventions"), markdown.MDMarkers.End("Language Conventions"))
	if !ok {
		return nil
	}
	var found []string
	for _, name := range docNamesInOrder(cfg) {
		lang := cfg.AgentsMD.Languages[name]
		if lang.Ref != "" && strings.Contains(content, lang.Ref) {
			found = append(found, name)
		}
	}
	return found
}

// docNamesInOrder returns all language doc names, following the top-level
// docs: list order first, then any remaining names sorted.
func docNamesInOrder(cfg *Config) []string {
	seen := make(map[string]struct{}, len(cfg.Docs))
	var names []string
	for _, name := range cfg.Docs {
		if _, ok := cfg.AgentsMD.Languages[name]; ok {
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	var rest []string
	for name := range cfg.AgentsMD.Languages {
		if _, ok := seen[name]; !ok {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	return append(names, rest...)
}

// orderDocNames puts selected docs in the canonical config order used by
// existingLangDocs and plain re-init, so the first generated list is stable.
func orderDocNames(cfg *Config, selected []string) []string {
	wanted := make(map[string]struct{}, len(selected))
	for _, name := range selected {
		wanted[name] = struct{}{}
	}
	ordered := make([]string, 0, len(selected))
	for _, name := range docNamesInOrder(cfg) {
		if _, ok := wanted[name]; ok {
			ordered = append(ordered, name)
			delete(wanted, name)
		}
	}
	// Preserve unknown names defensively; validation normally rejects them.
	for _, name := range selected {
		if _, ok := wanted[name]; ok {
			ordered = append(ordered, name)
			delete(wanted, name)
		}
	}
	return ordered
}

// expandDocNames expands profile names and "all" or "full" to their constituent doc lists,
// and returns deduplicated names.
func expandDocNames(cfg *Config, names []string) []string {
	if cfg == nil {
		return names
	}
	var expanded []string
	visitedProfiles := make(map[string]bool)

	var expand func(name string)
	expand = func(name string) {
		if name == "all" {
			expanded = append(expanded, docNamesInOrder(cfg)...)
			return
		}
		if cfg.DocsProfiles != nil {
			if profileDocs, ok := cfg.DocsProfiles[name]; ok {
				if visitedProfiles[name] {
					return
				}
				visitedProfiles[name] = true
				for _, doc := range profileDocs {
					expand(doc)
				}
				return
			}
		}
		if name == "full" {
			expanded = append(expanded, docNamesInOrder(cfg)...)
			return
		}
		expanded = append(expanded, name)
	}

	for _, name := range names {
		expand(name)
	}
	return jsonc.UnionStrings(nil, expanded)
}

// validateDocNames returns an error naming any requested doc that is not
// defined in the config, so typos fail loudly instead of being skipped.
func validateDocNames(cfg *Config, names []string) error {
	if cfg == nil {
		return nil
	}
	var unknown []string
	for _, name := range names {
		if name == "all" || name == "full" {
			continue
		}
		if cfg.DocsProfiles != nil {
			if _, ok := cfg.DocsProfiles[name]; ok {
				continue
			}
		}
		if _, ok := cfg.AgentsMD.Languages[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	return fmt.Errorf("unknown doc(s) %s — available: %s",
		strings.Join(unknown, ", "), strings.Join(docNamesInOrder(cfg), ", "))
}

// resolveDocDependencies returns a deterministic dependency-first closure.
func resolveDocDependencies(cfg *Config, names []string) ([]string, error) {
	state := make(map[string]uint8)
	var resolved []string
	var visit func(string) error
	visit = func(name string) error {
		lang, ok := cfg.AgentsMD.Languages[name]
		if !ok {
			return fmt.Errorf("unknown copied doc %q", name)
		}
		if state[name] == 1 {
			return fmt.Errorf("copied-doc dependency cycle at %q", name)
		}
		if state[name] == 2 {
			return nil
		}
		state[name] = 1
		for _, dep := range lang.DependsOn {
			if _, ok := cfg.AgentsMD.Languages[dep]; !ok {
				return fmt.Errorf("doc %q depends on unknown doc %q; define it or remove depends_on", name, dep)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[name] = 2
		resolved = append(resolved, name)
		return nil
	}
	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return resolved, nil
}

// validateCopyableDocCatalog rejects unmodeled hard edges before init writes
// anything. Relative links to configured copied docs are normative edges;
// illustrative repository material must be summarized inline or externally linked.
func ValidateCopyableDocCatalog(cfg *Config) error {
	byBase := make(map[string]string)
	for _, name := range docNamesInOrder(cfg) {
		lang := cfg.AgentsMD.Languages[name]
		if lang.Local != "" {
			base := filepath.Base(lang.Local)
			if prior, exists := byBase[base]; exists {
				return fmt.Errorf("copyable docs %q and %q share local target %q", prior, name, base)
			}
			byBase[base] = name
		}
	}
	for _, name := range docNamesInOrder(cfg) {
		lang := cfg.AgentsMD.Languages[name]
		if len(lang.Capabilities) > 0 && lang.Default != "" && lang.Default != "false" {
			return fmt.Errorf("capability-scoped doc %q must use default: false; select capabilities explicitly", name)
		}
		data, err := fs.ReadFile(cfg.FS, lang.SourceFor(""))
		if err != nil {
			return fmt.Errorf("copyable doc %q source %q: %w", name, lang.Source, err)
		}
		closure, err := resolveDocDependencies(cfg, []string{name})
		if err != nil {
			return err
		}
		allowed := make(map[string]bool, len(closure))
		for _, dep := range closure[:len(closure)-1] {
			allowed[dep] = true
		}
		for _, match := range markdownRelativeLinkRE.FindAllStringSubmatch(markdownOutsideFences(data), -1) {
			if strings.Contains(match[1], "://") || strings.HasPrefix(match[1], "mailto:") {
				continue
			}
			target, configured := byBase[filepath.Base(match[1])]
			if !configured {
				return fmt.Errorf("copyable doc %q source %q has repository-relative illustrative reference %q; summarize it inline or use a stable external link", name, lang.Source, match[1])
			}
			if target != name && !allowed[target] {
				return fmt.Errorf("copyable doc %q source %q has undeclared hard reference %q; add %q to depends_on or make the reference illustrative and self-contained", name, lang.Source, match[1], target)
			}
		}
		if match := unavailableProjectDocRE.FindString(string(data)); match != "" {
			return fmt.Errorf("copyable doc %q source %q references unavailable project material %q; summarize it inline or use a stable external link", name, lang.Source, match)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if !projectDestinationRE.MatchString(line) {
				continue
			}
			lower := strings.ToLower(line)
			optional := strings.Contains(lower, "when present") ||
				strings.Contains(lower, "when one exists") || strings.Contains(lower, "optional")
			fallback := strings.Contains(lower, "otherwise")
			if !optional || !fallback {
				return fmt.Errorf("copyable doc %q source %q assumes project destination %q; make it optional and state a fallback", name, lang.Source, projectDestinationRE.FindString(line))
			}
		}
	}
	return nil
}

// detectDoc returns true if project dir contains signals for the named doc.
func detectDoc(dir, name string) bool {
	switch name {
	case "golang":
		return fileExists(filepath.Join(dir, "go.mod"))
	case "gorelease":
		return fileExists(filepath.Join(dir, "go.mod"))
	case "manpages":
		return goModRequires(dir, "github.com/spf13/cobra")
	case "bash":
		return globExists(dir, "*.sh") || globExists(filepath.Join(dir, "scripts"), "*.sh")
	case "make":
		return fileExists(filepath.Join(dir, "Makefile"))
	case "rust":
		return fileExists(filepath.Join(dir, "Cargo.toml"))
	case "zig":
		return fileExists(filepath.Join(dir, "build.zig")) ||
			fileExists(filepath.Join(dir, "build.zig.zon")) ||
			globExists(dir, "*.zig")
	case "cpp":
		return globExists(dir, "*.c") || globExists(dir, "*.cpp") || globExists(dir, "*.cc") ||
			globExists(dir, "*.h") || globExists(dir, "*.hpp") || fileExists(filepath.Join(dir, "CMakeLists.txt"))
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func globExists(dir, pattern string) bool {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	return err == nil && len(matches) > 0
}

// goModRequires reports whether dir's go.mod names module as a dependency
// (direct or indirect). It is a plain substring check, not a full go.mod
// parse — good enough to gate an optional doc's auto-detection.
func goModRequires(dir, module string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), module)
}

func initialAgentsMD(cfg *Config) string {
	if cfg != nil {
		if cfg.AgentsMD.Local.Template != "" && cfg.FS != nil {
			if data, err := fs.ReadFile(cfg.FS, cfg.AgentsMD.Local.Template); err == nil {
				return string(data)
			}
		}
		if cfg.AgentsMD.Local.Content != "" {
			return cfg.AgentsMD.Local.Content
		}
	}
	if data, err := harnez.DefaultFS.ReadFile("docs/templates/AGENTS.md"); err == nil {
		return string(data)
	}
	return "Adhere to the following conventions.\n"
}

func backfillRulesHeader(path, header string) (bool, error) {
	if header == "" {
		return false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content := string(data)
	if strings.HasPrefix(content, header+"\n\n") {
		return false, nil
	}
	// Replace an older generated header (issue 671); a user's own first line stays.
	if first, rest, ok := strings.Cut(content, "\n"); ok && isRulesHeaderLine(first) {
		content = strings.TrimLeft(rest, "\n")
	}
	if err := os.WriteFile(path, []byte(header+"\n\n"+content), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// isRulesHeaderLine reports whether line is a harnez-generated rules header,
// current or older wording.
func isRulesHeaderLine(line string) bool {
	return strings.HasPrefix(line, "**Before any work, read") &&
		strings.HasSuffix(line, "**") &&
		strings.Contains(line, ".harnez/rules/")
}

func writeRules(dir string, rules RulesConfig, quota bool) (int, error) {
	if rules.Index == "" {
		return 0, nil
	}
	rulesDir := filepath.Join(dir, ".harnez", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		return 0, err
	}
	files := make([]RuleFile, 0, len(rules.Files))
	for _, file := range rules.Files {
		if file.Quota && !quota {
			continue
		}
		files = append(files, file)
	}
	index := rules.Index
	for _, file := range files {
		index += "- [" + file.Name + "](" + file.Name + ")\n"
	}
	files = append([]RuleFile{{Name: "Index.md", Content: index}}, files...)
	changes := 0
	for _, file := range files {
		content := strings.TrimSpace(file.Content) + "\n"
		path := filepath.Join(rulesDir, file.Name)
		existing, err := os.ReadFile(path)
		if err == nil && file.Quota {
			continue
		}
		if err == nil && string(existing) == content {
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			return changes, err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return changes, err
		}
		changes++
		fmt.Printf("  wrote   %s\n", path)
	}
	return changes, nil
}

type managedBlock struct {
	name                  string
	start, bodyStart, end int
	body                  string
}

var migratableAgentBlocks = []string{
	"Local Overlays", "Harnez Managed Conventions", quota1SectionName,
}

var migratableLocalBlocks = []string{"Concise Mode", "Subagent Policy"}

func parseManagedBlocks(content string, names []string) ([]managedBlock, bool) {
	lines := strings.SplitAfter(content, "\n")
	offsets := make([]int, len(lines)+1)
	for i, line := range lines {
		offsets[i+1] = offsets[i] + len(line)
	}
	blocks := make([]managedBlock, 0)
	active := -1
	for i, line := range lines {
		text := strings.TrimSuffix(line, "\n")
		text = strings.TrimSuffix(text, "\r")
		if active >= 0 && (strings.Contains(text, "<!-- harnez:begin ") || strings.Contains(text, "<!-- harnez:end ")) {
			wantEnd := "<!-- harnez:end " + blocks[active].name + " -->"
			if text != wantEnd {
				return nil, true
			}
		}
		for _, name := range names {
			begin := "<!-- harnez:begin " + name + " -->"
			end := "<!-- harnez:end " + name + " -->"
			if text == begin {
				if active >= 0 {
					return nil, true
				}
				blocks = append(blocks, managedBlock{name: name, start: offsets[i], bodyStart: offsets[i+1]})
				active = len(blocks) - 1
			} else if text == end {
				if active < 0 || blocks[active].name != name {
					return nil, true
				}
				blocks[active].end = offsets[i+1]
				blocks[active].body = content[blocks[active].bodyStart:offsets[i]]
				active = -1
			} else if strings.Contains(text, "<!-- harnez:begin "+name) || strings.Contains(text, "<!-- harnez:end "+name) {
				return nil, true
			}
		}
	}
	if active >= 0 {
		return nil, true
	}
	return blocks, false
}

func migrateManagedBlocks(path string, names []string, destination func(string) string) (bool, error) {
	return migrateManagedBlocksFormatted(path, names, destination, false)
}

func migrateManagedBlocksFormatted(path string, names []string, destination func(string) string, wrap bool) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	content := string(data)
	blocks, malformed := parseManagedBlocks(content, names)
	if malformed {
		fmt.Printf("  ⚠️  left malformed or nested managed markers untouched in %s\n", path)
		return false, nil
	}
	if len(blocks) == 0 {
		return false, nil
	}
	var out strings.Builder
	last := 0
	for _, block := range blocks {
		out.WriteString(content[last:block.start])
		last = block.end
		if target := destination(block.name); target != "" && block.body != "" {
			existing, err := os.ReadFile(target)
			if err != nil && !os.IsNotExist(err) {
				return false, err
			}
			prefix := string(existing)
			if prefix != "" && !strings.HasSuffix(prefix, "\n") {
				prefix += "\n"
			}
			if prefix != "" {
				prefix += "\n"
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return false, err
			}
			payload := block.body
			if wrap {
				payload = markdown.MDMarkers.Begin(block.name) + "\n" + payload
				if !strings.HasSuffix(payload, "\n") {
					payload += "\n"
				}
				payload += markdown.MDMarkers.End(block.name) + "\n"
			}
			if err := os.WriteFile(target, []byte(prefix+payload), 0o644); err != nil {
				return false, err
			}
		}
	}
	out.WriteString(content[last:])
	if err := os.WriteFile(path, []byte(out.String()), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func migrateInitRules(dir, agentsPath string) (int, error) {
	rulesDir := filepath.Join(dir, ".harnez", "rules")
	agentTarget := func(name string) string {
		switch name {
		case "Harnez Managed Conventions":
			return "" // Tools.md carries this content since issue 628.
		case quota1SectionName:
			return filepath.Join(rulesDir, "Quota.md")
		case "Local Overlays":
			return "" // Superseded by the new AGENTS.md header.
		default:
			return ""
		}
	}
	changed, err := migrateManagedBlocks(agentsPath, migratableAgentBlocks, agentTarget)
	if err != nil {
		return 0, fmt.Errorf("migrate managed blocks in %s: %w", agentsPath, err)
	}
	changes := 0
	if changed {
		changes++
	}
	localPath := filepath.Join(dir, "AGENTS.local.md")
	changed, err = migrateManagedBlocksFormatted(localPath, migratableLocalBlocks, func(string) string {
		return filepath.Join(rulesDir, "Local.md")
	}, true)
	if err != nil {
		return changes, fmt.Errorf("migrate managed blocks in %s: %w", localPath, err)
	}
	if changed {
		changes++
	}
	return changes, nil
}

// MigrateLegacyLocalRules moves known managed local sections from AGENTS.local.md
// into .harnez/rules/Local.md, preserving all owner text in the legacy file.
func MigrateLegacyLocalRules(dir string) (bool, error) {
	legacyPath := filepath.Join(dir, "AGENTS.local.md")
	targetPath := filepath.Join(dir, ".harnez", "rules", "Local.md")
	return migrateManagedBlocksFormatted(legacyPath, migratableLocalBlocks, func(string) string {
		return targetPath
	}, true)
}

var projectManifestNames = map[string]struct{}{
	"go.mod":              {},
	"go.work":             {},
	"go.work.example":     {},
	"Cargo.toml":          {},
	"Cargo.lock":          {},
	"package.json":        {},
	"pnpm-workspace.yaml": {},
	"Makefile":            {},
	"GNUmakefile":         {},
	"makefile":            {},
	"justfile":            {},
	"Taskfile.yml":        {},
	"Taskfile.yaml":       {},
	"CMakeLists.txt":      {},
	"meson.build":         {},
	"build.zig":           {},
	"build.zig.zon":       {},
	"pyproject.toml":      {},
	"setup.py":            {},
	"setup.cfg":           {},
	"requirements.txt":    {},
	"Pipfile":             {},
	"pom.xml":             {},
	"build.gradle":        {},
	"build.gradle.kts":    {},
	"Gemfile":             {},
	"Containerfile":       {},
	"Dockerfile":          {},
	"docker-compose.yml":  {},
	"docker-compose.yaml": {},
	"compose.yaml":        {},
	"compose.yml":         {},
	"AGENTS.md":           {},
	"CLAUDE.md":           {},
}

var sourceFileExtensions = map[string]struct{}{
	".go":    {},
	".rs":    {},
	".py":    {},
	".c":     {},
	".cpp":   {},
	".cc":    {},
	".cxx":   {},
	".h":     {},
	".hpp":   {},
	".hxx":   {},
	".zig":   {},
	".ts":    {},
	".js":    {},
	".mjs":   {},
	".cjs":   {},
	".sh":    {},
	".bash":  {},
	".java":  {},
	".rb":    {},
	".php":   {},
	".swift": {},
	".kt":    {},
	".kts":   {},
	".scala": {},
	".lua":   {},
	".nim":   {},
	".hs":    {},
}

var codeSubdirectories = map[string]struct{}{
	"src":      {},
	"cmd":      {},
	"internal": {},
	"lib":      {},
	"pkg":      {},
	"scripts":  {},
	"app":      {},
	"issues":   {},
	"spec":     {},
}

func isEligibleProjectDir(dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	if home, herr := os.UserHomeDir(); herr == nil {
		if homeAbs, aerr := filepath.Abs(home); aerr == nil {
			if abs == homeAbs {
				return false
			}
			globalRoots := []string{
				filepath.Join(homeAbs, ".claude"),
				filepath.Join(homeAbs, ".prime"),
				filepath.Join(homeAbs, ".codex"),
				filepath.Join(homeAbs, ".gemini"),
			}
			for _, root := range globalRoots {
				if abs == root || strings.HasPrefix(abs, root+string(filepath.Separator)) {
					return false
				}
			}
		}
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return true
	}
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) == "true" {
		return true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		name := entry.Name()
		if _, ok := projectManifestNames[name]; ok {
			return true
		}
		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(name))
			if _, ok := sourceFileExtensions[ext]; ok {
				return true
			}
		} else {
			if _, ok := codeSubdirectories[strings.ToLower(name)]; ok {
				if name == "issues" || name == "spec" {
					return true
				}
				subEntries, err := os.ReadDir(filepath.Join(dir, name))
				if err == nil {
					for _, subEntry := range subEntries {
						subName := subEntry.Name()
						if _, ok := projectManifestNames[subName]; ok {
							return true
						}
						subExt := strings.ToLower(filepath.Ext(subName))
						if _, ok := sourceFileExtensions[subExt]; ok {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

// ValidateInitTarget checks whether dir is safe to initialize.
// It refuses the user's home directory, root directory, global agent config roots, or non-coding directory unless force is true.
func ValidateInitTarget(dir string, force bool) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve directory: %w", err)
	}

	if home, herr := os.UserHomeDir(); herr == nil {
		if homeAbs, aerr := filepath.Abs(home); aerr == nil {
			if abs == homeAbs {
				return fmt.Errorf("refusing to run init directly on home directory %s; use --force to override", abs)
			}
			globalRoots := []string{
				filepath.Join(homeAbs, ".claude"),
				filepath.Join(homeAbs, ".prime"),
				filepath.Join(homeAbs, ".codex"),
				filepath.Join(homeAbs, ".gemini"),
			}
			for _, root := range globalRoots {
				if abs == root || strings.HasPrefix(abs, root+string(filepath.Separator)) {
					return fmt.Errorf("refusing to run init on global agent directory %s: global agent roots are user-owned instruction locations and must not be initialized as projects", abs)
				}
			}
		}
	}

	if abs == "/" || filepath.Dir(abs) == abs {
		return fmt.Errorf("refusing to run init directly on root directory %s; use --force to override", abs)
	}

	if force {
		return nil
	}

	if !isEligibleProjectDir(abs) {
		return fmt.Errorf("refusing to run init in %s: directory is not a Git repository and contains no recognized project or source files; use --force to override", abs)
	}

	return nil
}

// ProjectedInitFiles returns the list of repository-relative paths that harnez init would create or reconcile.
func ProjectedInitFiles(dir string, cfg *Config, docs []string) []string {
	var files []string
	files = append(files, "AGENTS.md", "CLAUDE.md")
	if cfg != nil {
		allDocs := append([]string(nil), docs...)
		allDocs = append(allDocs, existingLangDocs(filepath.Join(dir, "AGENTS.md"), cfg)...)
		allDocs = append(allDocs, autoDetectDocs(dir, cfg, allDocs)...)
		allDocs, _ = resolveDocDependencies(cfg, allDocs)
		for _, name := range allDocs {
			if lang, ok := cfg.AgentsMD.Languages[name]; ok && lang.Local != "" {
				files = append(files, lang.Local)
			}
		}
	}
	return files
}

// RunInit creates AGENTS.md and CLAUDE.md symlink in a project directory,
// applies config-defined local sections, and sets up language docs and Makefile targets.
func RunInit(dir string, cfg *Config, docs []string, repoMode string, assumeYes, withSummary, update, replace bool) error {
	return RunInitWithForce(dir, cfg, docs, repoMode, assumeYes, withSummary, update, replace, nil, false, false)
}

// RunInitWithIssuesGit runs project initialization and, when issuesGit is
// non-nil, explicitly enables or disables the issue tracker Git integration.
// A nil value leaves that integration untouched.
func RunInitWithIssuesGit(dir string, cfg *Config, docs []string, repoMode string, assumeYes, withSummary, update, replace bool, issuesGit *bool) error {
	return RunInitWithForce(dir, cfg, docs, repoMode, assumeYes, withSummary, update, replace, issuesGit, false, false)
}

// RunInitWithGoWork runs project initialization and supports explicit --gowork management.
func RunInitWithGoWork(dir string, cfg *Config, docs []string, repoMode string, assumeYes, withSummary, update, replace bool, issuesGit *bool, gowork bool) error {
	return RunInitWithForce(dir, cfg, docs, repoMode, assumeYes, withSummary, update, replace, issuesGit, gowork, false)
}

// RunInitWithForce runs project initialization with explicit --gowork and --force support.
func RunInitWithForce(dir string, cfg *Config, docs []string, repoMode string, assumeYes, withSummary, update, replace bool, issuesGit *bool, gowork bool, force bool) error {
	return RunInitWithVariant(dir, cfg, docs, repoMode, assumeYes, withSummary, update, replace, issuesGit, gowork, force, "")
}

// RunInitWithVariant is RunInitWithForce with an explicit doc variant
// ("" or "lite") selecting which source (Language.SourceFor) is copied for
// docs that declare a lite_source, and an optional quota1 flag.
func RunInitWithVariant(dir string, cfg *Config, docs []string, repoMode string, assumeYes, withSummary, update, replace bool, issuesGit *bool, gowork bool, force bool, variant string, quota1 ...bool) error {
	if err := ValidateInitTarget(dir, force); err != nil {
		return err
	}
	if cfg != nil {
		docs = expandDocNames(cfg, docs)
		if err := validateDocNames(cfg, docs); err != nil {
			return err
		}
		if err := ValidateCopyableDocCatalog(cfg); err != nil {
			return err
		}
	}
	for _, w := range CheckProjectDrift(dir) {
		fmt.Printf("  ⚠️  drift: %s\n", w)
	}

	agentsPath := filepath.Join(dir, "AGENTS.md")
	claudePath := filepath.Join(dir, "CLAUDE.md")

	if update {
		withSummary = true
	}

	changes := 0

	if replace {
		if err := os.Remove(agentsPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", agentsPath, err)
		}
	}

	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		if fi, err := os.Lstat(claudePath); err == nil && fi.Mode().IsRegular() {
			if err := os.Rename(claudePath, agentsPath); err != nil {
				return fmt.Errorf("migrate %s → %s: %w", claudePath, agentsPath, err)
			}
			fmt.Printf("  migrated %s → %s\n", claudePath, agentsPath)
			changes++
		} else {
			if err := os.WriteFile(agentsPath, []byte(initialAgentsMD(cfg)), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", agentsPath, err)
			}
			fmt.Printf("  created %s\n", agentsPath)
			changes++
		}
	} else {
		if migrated, err := migrateLegacyMarkers(agentsPath); err != nil {
			return fmt.Errorf("migrate markers %s: %w", agentsPath, err)
		} else if migrated {
			fmt.Printf("  upgraded markers %s\n", agentsPath)
			changes++
		} else {
			fmt.Printf("  exists  %s (unchanged)\n", agentsPath)
		}
	}
	if cfg != nil {
		if backfilled, err := backfillRulesHeader(agentsPath, cfg.AgentsMD.Rules.Header); err != nil {
			return fmt.Errorf("backfill rules header %s: %w", agentsPath, err)
		} else if backfilled {
			fmt.Printf("  updated rules header %s\n", agentsPath)
			changes++
		}
	}
	agentBlocksSafe := true
	if data, err := os.ReadFile(agentsPath); err != nil {
		return fmt.Errorf("read %s for managed block migration: %w", agentsPath, err)
	} else if _, malformed := parseManagedBlocks(string(data), migratableAgentBlocks); malformed {
		agentBlocksSafe = false
	}

	symlinkChanged, err := fsutil.EnsureSymlink(claudePath, agentsPath)
	if err != nil {
		return fmt.Errorf("symlink %s: %w", claudePath, err)
	}
	if symlinkChanged {
		fmt.Printf("  symlink %s → %s\n", claudePath, agentsPath)
		changes++
	} else {
		fmt.Printf("  exists  %s (unchanged)\n", claudePath)
	}

	_, _ = fsutil.EnsureGitExclude(dir, "AGENTS.local.md")
	if ignored, err := fsutil.EnsureGitExclude(dir, ".harnez/rules/Local.md"); err != nil {
		return fmt.Errorf("ignore .harnez/rules/Local.md: %w", err)
	} else if ignored {
		fmt.Printf("  ignored .harnez/rules/Local.md in .git/info/exclude\n")
		changes++
	}
	if fileExists(filepath.Join(dir, "issues")) {
		// Lock files left by the README sync and `harnez issues new` (issue 672).
		for _, lock := range []string{"/issues/README.md.lock", "/issues/.reserve.lock"} {
			ignored, err := fsutil.EnsureGitExclude(dir, lock)
			if err != nil {
				return fmt.Errorf("ignore %s: %w", lock, err)
			}
			if ignored {
				fmt.Printf("  ignored %s in .git/info/exclude\n", lock)
				changes++
			}
		}
	}
	if issuesGit != nil {
		var gitChanges int
		if *issuesGit {
			gitChanges, err = installIssuesGitIntegration(dir)
		} else {
			gitChanges, err = removeIssuesGitIntegration(dir)
		}
		if err != nil {
			return err
		}
		changes += gitChanges
	}

	workspaceChanged, err := reconcileGoWorkspace(dir, gowork)
	if err != nil {
		return err
	}
	if workspaceChanged {
		changes++
	}

	if cfg != nil {
		docs = append(docs, existingLangDocs(agentsPath, cfg)...)
		docs = append(docs, autoDetectDocs(dir, cfg, docs)...)
		docs, err = resolveDocDependencies(cfg, docs)
		if err != nil {
			return err
		}
		// Resolve an omitted variant independently for each destination. Explicit
		// "lite" or "full" values always override the existing copy.
		docVariants := make(map[string]string, len(docs))
		for _, name := range docs {
			lang, ok := cfg.AgentsMD.Languages[name]
			if !ok || lang.Local == "" {
				continue
			}
			resolved := variant
			if resolved == "" {
				data, err := os.ReadFile(localPath(dir, lang.Local))
				if err == nil {
					resolved = markdown.ParseVariantMarker(string(data))
				}
				if resolved == "lite" {
					if lang.LiteSource == "" {
						fmt.Printf("  ⚠️  %s has lite marker but no lite source; using default source\n", name)
						resolved = ""
					} else if _, err := fs.Stat(cfg.FS, lang.LiteSource); err != nil {
						fmt.Printf("  ⚠️  %s has lite marker but no lite source; using default source\n", name)
						resolved = ""
					}
				}
			}
			docVariants[name] = resolved
		}
		// Check every selected doc before copying any of them. A markerless
		// local edit must never be lost halfway through a multi-doc init.
		for _, name := range docs {
			lang, ok := cfg.AgentsMD.Languages[name]
			if !ok || lang.Local == "" {
				continue
			}
			data, err := fs.ReadFile(cfg.FS, lang.SourceFor(docVariants[name]))
			if err != nil {
				return fmt.Errorf("language %s: read source: %w", name, err)
			}
			if _, _, err := prepareManagedDoc(localPath(dir, lang.Local), data); err != nil {
				return fmt.Errorf("language %s: copy: %w", name, err)
			}
		}

		// Apply config-defined local AGENTS.md sections (e.g. Language Conventions).
		if l := cfg.AgentsMD.Local; l.Target != "" && agentBlocksSafe {
			sections := append([]MDSection(nil), l.Sections...)
			if len(docs) > 0 {
				sections = append(sections, MDSection{
					Name:    "Language Conventions",
					Content: buildLangConventions(orderDocNames(cfg, docs), cfg),
				})
			}
			if repoMode != "" {
				mode, ok := cfg.AgentsMD.RepoModes[repoMode]
				if !ok {
					return fmt.Errorf("unknown repo mode %q", repoMode)
				}
				sections = append(sections, MDSection{
					Name:    "Repo Setup",
					Content: mode.Content,
				})
			}
			keepSections := make([]string, 0, len(sections)+5)
			// Preserve template, backfill, and opt-in blocks managed by init
			// outside the config-defined section list. These are sticky across
			// plain init runs because absence of a flag is not removal intent.
			keepSections = append(keepSections,
				"Local Overlays", "Project Summary", "Language Conventions",
				"Repo Setup", quota1SectionName,
			)
			for _, s := range sections {
				keepSections = append(keepSections, s.Name)
			}
			pruned, err := markdown.PruneSections(agentsPath, keepSections)
			if err != nil {
				return fmt.Errorf("agents_md.local prune: %w", err)
			}
			if pruned {
				changes++
			}
			lr := applyResult{}
			var presentNames []string
			for _, s := range sections {
				r, err := applySectionMD(agentsPath, s.Name, s.Content)
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
			printResult("wrote", agentsPath, lr)
			_ = presentNames
		}

		// Copy language docs locally and inject Makefile targets.
		for _, name := range docs {
			lang, ok := cfg.AgentsMD.Languages[name]
			if !ok || lang.Local == "" {
				continue
			}
			data, err := fs.ReadFile(cfg.FS, lang.SourceFor(docVariants[name]))
			if err != nil {
				return fmt.Errorf("language %s: read source: %w", name, err)
			}
			localDoc := localPath(dir, lang.Local)
			cr, err := writeManagedDoc(localDoc, data)
			if err != nil {
				return fmt.Errorf("language %s: copy: %w", name, err)
			}
			if cr.changed {
				changes++
				fmt.Printf("  copied %s → %s\n", lang.SourceFor(docVariants[name]), localDoc)
			}

			justScaffolded := false
			if lang.Template != "" {
				dest := localPath(dir, filepath.Base(lang.Template))
				if _, err := os.Stat(dest); os.IsNotExist(err) {
					data, err := fs.ReadFile(cfg.FS, lang.Template)
					if err != nil {
						return fmt.Errorf("language %s: read template: %w", name, err)
					}
					if err := os.WriteFile(dest, data, 0644); err != nil {
						return fmt.Errorf("language %s: scaffold template: %w", name, err)
					}
					changes++
					justScaffolded = true
					fmt.Printf("  scaffolded %s\n", dest)
				}
			}

			if lang.Targets != "" && !justScaffolded {
				dest := localPath(dir, filepath.Base(lang.Template))
				if lang.Template == "" {
					dest = localPath(dir, "Makefile")
				}
				if _, err := os.Stat(dest); err == nil {
					data, err := fs.ReadFile(cfg.FS, lang.Targets)
					if err != nil {
						return fmt.Errorf("language %s: read targets: %w", name, err)
					}
					tr, err := ReconcileMakeTargets(dest, string(data), cfg.Make, assumeYes, nil)
					if err != nil {
						return fmt.Errorf("language %s: reconcile targets: %w", name, err)
					}
					if tr {
						changes++
						fmt.Printf("  reconciled %s\n", dest)
					} else {
						fmt.Printf("  exists  %s (targets unchanged)\n", dest)
					}
				}
			}
		}
	}

	quota1Active := len(quota1) > 0 && quota1[0]
	if !quota1Active {
		quota1Active = markdown.ContainsSection(agentsPath, quota1SectionName)
	}
	if !quota1Active {
		_, err := os.Stat(filepath.Join(dir, ".harnez", "rules", "Quota.md"))
		quota1Active = err == nil
	}
	if cfg != nil {
		ruleChanges, err := writeRules(dir, cfg.AgentsMD.Rules, quota1Active)
		if err != nil {
			return fmt.Errorf("write .harnez/rules: %w", err)
		}
		changes += ruleChanges
		if ignored, err := rulesAreGitIgnored(dir); err != nil {
			return fmt.Errorf("check generated rules ignore status: %w", err)
		} else if ignored {
			fmt.Printf("  ⚠️  .harnez/rules is ignored by Git; generated rule files may not be tracked\n")
		}
	}
	if len(quota1) > 0 && quota1[0] && agentBlocksSafe {
		if cfg == nil || cfg.AgentsMD.Rules.QuotaSection == "" {
			return fmt.Errorf("quota-1 rules are not configured")
		}
		r, err := applySectionMD(agentsPath, quota1SectionName, cfg.AgentsMD.Rules.QuotaSection)
		if err != nil {
			return fmt.Errorf("agents_md [%s]: %w", quota1SectionName, err)
		}
		if r.changed {
			changes++
		}
		printResult("wrote", agentsPath, r)

		makePath := localPath(dir, "Makefile")
		targetSnippet := "test-q1: 🤖  # run tests under Quota-1 enforcement\n\tharnez exec --quota-1 -- $(MAKE) test\n"
		if _, err := os.Stat(makePath); os.IsNotExist(err) {
			initialMake := fmt.Sprintf(".PHONY: ⚙️ 🤖\n⚙️:\n🤖:\n\n%s", targetSnippet)
			if err := os.WriteFile(makePath, []byte(initialMake), 0644); err != nil {
				return fmt.Errorf("scaffold %s: %w", makePath, err)
			}
			changes++
			fmt.Printf("  scaffolded %s (test-q1)\n", makePath)
		} else if err == nil {
			var makeCfg MakeConfig
			if cfg != nil {
				makeCfg = cfg.Make
			}
			tr, err := ReconcileMakeTargets(makePath, targetSnippet, makeCfg, assumeYes, nil)
			if err != nil {
				return fmt.Errorf("reconcile test-q1 in %s: %w", makePath, err)
			}
			if tr {
				changes++
				fmt.Printf("  reconciled %s (test-q1)\n", makePath)
			} else {
				fmt.Printf("  exists  %s (test-q1 unchanged)\n", makePath)
			}
		}
	}

	if withSummary {
		fmt.Println("  running claude -p to summarise project...")
		draft, err := fetchSummary(dir)
		if err != nil {
			return err
		}
		draft = sanitizeContent(stripMetaCommentary(draft))
		if _, _, err := markdown.Apply(agentsPath, summarySection, draft); err != nil {
			return fmt.Errorf("summary section (draft): %w", err)
		}

		fmt.Println("  running claude -p to review and refine summary...")
		refined, err := reviewSummary(dir, draft)
		if err != nil {
			return err
		}
		refined = sanitizeContent(stripMetaCommentary(refined))
		summaryChanged, _, err := markdown.Apply(agentsPath, summarySection, refined)
		if err != nil {
			return fmt.Errorf("summary section (refined): %w", err)
		}
		if summaryChanged {
			fmt.Printf("  wrote   %s [%s]\n", agentsPath, summarySection)
			changes++
		} else {
			fmt.Printf("  exists  %s [%s] (unchanged)\n", agentsPath, summarySection)
		}
	}
	migrated, err := migrateInitRules(dir, agentsPath)
	if err != nil {
		return err
	}
	changes += migrated
	if cfg != nil {
		if normalized, err := normalizeRulesHeaderGap(agentsPath, cfg.AgentsMD.Rules.Header); err != nil {
			return fmt.Errorf("normalize rules header spacing %s: %w", agentsPath, err)
		} else if normalized {
			fmt.Printf("  normalized rules header spacing %s\n", agentsPath)
			changes++
		}
	}

	if changes == 0 {
		fmt.Println("No changes.")
	} else {
		fmt.Printf("%d change(s).\n", changes)
	}
	return nil
}

func normalizeRulesHeaderGap(path, header string) (bool, error) {
	if header == "" {
		return false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content := string(data)
	prefix := header + "\n"
	if !strings.HasPrefix(content, prefix) {
		return false, nil
	}
	rest := strings.TrimLeft(strings.TrimPrefix(content, prefix), "\n")
	normalized := header + "\n\n" + rest
	if normalized == content {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(normalized), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func rulesAreGitIgnored(dir string) (bool, error) {
	if _, ok := fsutil.FindGitDir(dir); !ok {
		return false, nil
	}
	cmd := exec.Command("git", "-C", dir, "check-ignore", "-q", "--", ".harnez/rules/Index.md")
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	if errors.As(err, &exitErr) {
		return false, fmt.Errorf("git check-ignore exited %d", exitErr.ExitCode())
	}
	return false, err
}

const issuesAttributesLine = "issues/README.md merge=harnez-issues-index"
const issuesHookBlock = `# harnez:begin issues-index-lint
if command -v harnez >/dev/null 2>&1
then harnez issues lint --cached -d "$(git rev-parse --show-toplevel)" || exit $?
fi
# harnez:end issues-index-lint
`

// installIssuesGitIntegration installs clone-local driver configuration and
// repository files through init. It deliberately does nothing outside Git
// repositories and appends to (rather than replacing) user-owned files.
func installIssuesGitIntegration(dir string) (int, error) {
	if err := exec.Command("git", "-C", dir, "rev-parse", "--git-dir").Run(); err != nil {
		return 0, nil
	}
	changes := 0
	attributes := filepath.Join(dir, ".gitattributes")
	content, err := os.ReadFile(attributes)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("read %s: %w", attributes, err)
	}
	if !containsExactLine(string(content), issuesAttributesLine) {
		prefix := string(content)
		if prefix != "" && !strings.HasSuffix(prefix, "\n") {
			prefix += "\n"
		}
		if err := os.WriteFile(attributes, []byte(prefix+issuesAttributesLine+"\n"), 0o644); err != nil {
			return 0, fmt.Errorf("write %s: %w", attributes, err)
		}
		fmt.Printf("  configured %s as a generated merge path\n", attributes)
		changes++
	}
	configArgs := []string{"-C", dir, "config", "--local", "merge.harnez-issues-index.driver", "harnez issues merge-driver %O %A %B"}
	if out, err := exec.Command("git", configArgs...).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("configure generated issue-index merge driver: %w: %s", err, strings.TrimSpace(string(out)))
	}
	nameArgs := []string{"-C", dir, "config", "--local", "merge.harnez-issues-index.name", "harnez generated issue index"}
	if out, err := exec.Command("git", nameArgs...).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("name generated issue-index merge driver: %w: %s", err, strings.TrimSpace(string(out)))
	}
	hooksDirOut, err := exec.Command("git", "-C", dir, "rev-parse", "--git-path", "hooks").Output()
	if err != nil {
		return 0, fmt.Errorf("locate Git hooks: %w", err)
	}
	hooksDir := strings.TrimSpace(string(hooksDirOut))
	if hooksDir == "/dev/null" {
		return changes, nil
	}
	if !filepath.IsAbs(hooksDir) {
		hooksDir = filepath.Join(dir, hooksDir)
	}
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return 0, fmt.Errorf("create Git hooks directory: %w", err)
	}
	hook := filepath.Join(hooksDir, "pre-commit")
	hookContent, err := os.ReadFile(hook)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("read %s: %w", hook, err)
	}
	hookText := string(hookContent)
	wantHook := hookText
	const hookStart = "# harnez:begin issues-index-lint"
	const hookEnd = "# harnez:end issues-index-lint"
	if start := strings.Index(hookText, hookStart); start >= 0 {
		endRel := strings.Index(hookText[start:], hookEnd)
		if endRel < 0 {
			return 0, fmt.Errorf("update %s: managed issue-index hook block has no end marker", hook)
		}
		end := start + endRel + len(hookEnd)
		if end < len(hookText) && hookText[end] == '\n' {
			end++
		}
		wantHook = hookText[:start] + issuesHookBlock + hookText[end:]
	} else {
		prefix := hookText
		if prefix == "" {
			prefix = "#!/bin/sh\n"
		} else if !strings.HasSuffix(prefix, "\n") {
			prefix += "\n"
		}
		wantHook = prefix + issuesHookBlock
	}
	if wantHook != hookText {
		if err := os.WriteFile(hook, []byte(wantHook), 0o755); err != nil {
			return 0, fmt.Errorf("write %s: %w", hook, err)
		}
		fmt.Printf("  installed issue-index lint in %s\n", hook)
		changes++
	}
	info, err := os.Stat(hook)
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", hook, err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		if err := os.Chmod(hook, info.Mode().Perm()|0o111); err != nil {
			return 0, fmt.Errorf("make %s executable: %w", hook, err)
		}
		changes++
	}
	return changes, nil
}

func removeIssuesGitIntegration(dir string) (int, error) {
	if err := exec.Command("git", "-C", dir, "rev-parse", "--git-dir").Run(); err != nil {
		return 0, nil
	}
	changes := 0
	attributes := filepath.Join(dir, ".gitattributes")
	content, err := os.ReadFile(attributes)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("read %s: %w", attributes, err)
	}
	if err == nil {
		var kept []string
		removed := false
		for _, line := range strings.Split(string(content), "\n") {
			if strings.TrimSpace(line) == issuesAttributesLine {
				removed = true
				continue
			}
			kept = append(kept, line)
		}
		if removed {
			if err := os.WriteFile(attributes, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
				return 0, fmt.Errorf("write %s: %w", attributes, err)
			}
			changes++
		}
	}
	for _, key := range []string{"merge.harnez-issues-index.driver", "merge.harnez-issues-index.name"} {
		cmd := exec.Command("git", "-C", dir, "config", "--local", "--unset-all", key)
		if out, err := cmd.CombinedOutput(); err == nil {
			changes++
		} else if cmd.ProcessState.ExitCode() != 5 {
			return 0, fmt.Errorf("remove %s: %w: %s", key, err, strings.TrimSpace(string(out)))
		}
	}
	hooksDirOut, err := exec.Command("git", "-C", dir, "rev-parse", "--git-path", "hooks").Output()
	if err != nil {
		return 0, fmt.Errorf("locate Git hooks: %w", err)
	}
	hooksDir := strings.TrimSpace(string(hooksDirOut))
	if !filepath.IsAbs(hooksDir) {
		hooksDir = filepath.Join(dir, hooksDir)
	}
	hook := filepath.Join(hooksDir, "pre-commit")
	hookContent, err := os.ReadFile(hook)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("read %s: %w", hook, err)
	}
	if err == nil {
		updated, removed, err := removeManagedIssuesHook(string(hookContent))
		if err != nil {
			return 0, fmt.Errorf("update %s: %w", hook, err)
		}
		if removed {
			if err := os.WriteFile(hook, []byte(updated), 0o755); err != nil {
				return 0, fmt.Errorf("write %s: %w", hook, err)
			}
			changes++
		}
	}
	return changes, nil
}

func removeManagedIssuesHook(content string) (string, bool, error) {
	const startMarker = "# harnez:begin issues-index-lint"
	const endMarker = "# harnez:end issues-index-lint"
	start := strings.Index(content, startMarker)
	if start < 0 {
		return content, false, nil
	}
	endRel := strings.Index(content[start:], endMarker)
	if endRel < 0 {
		return content, false, fmt.Errorf("managed issue-index hook block has no end marker")
	}
	end := start + endRel + len(endMarker)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return content[:start] + content[end:], true, nil
}

func containsExactLine(content, want string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

// RunInitAll discovers eligible child project directories under parentDir
// (directories containing AGENTS.md or CLAUDE.md — the same eligibility rule
// ScanDocs uses) and runs RunInit non-interactively against each one in turn.
// It refuses to operate directly on the user's home directory so a bare
// `harnez init --all ..` from a project one level under $HOME can never treat
// $HOME itself as a project container (see issue 068).
func RunInitAll(parentDir string, cfg *Config, docs []string, repoMode string, withSummary, update, replace bool) error {
	return RunInitAllWithIssuesGit(parentDir, cfg, docs, repoMode, withSummary, update, replace, nil)
}

func RunInitAllWithIssuesGit(parentDir string, cfg *Config, docs []string, repoMode string, withSummary, update, replace bool, issuesGit *bool) error {
	return RunInitAllWithGoWork(parentDir, cfg, docs, repoMode, withSummary, update, replace, issuesGit, false)
}

func RunInitAllWithGoWork(parentDir string, cfg *Config, docs []string, repoMode string, withSummary, update, replace bool, issuesGit *bool, gowork bool) error {
	return RunInitAllWithVariant(parentDir, cfg, docs, repoMode, withSummary, update, replace, issuesGit, gowork, "")
}

// RunInitAllWithVariant is RunInitAllWithGoWork with an explicit doc variant
// ("" or "lite") selecting which source (Language.SourceFor) is copied for
// docs that declare a lite_source, threaded into each child's RunInitWithVariant call.
func RunInitAllWithVariant(parentDir string, cfg *Config, docs []string, repoMode string, withSummary, update, replace bool, issuesGit *bool, gowork bool, variant string, quota1 ...bool) error {
	if parentDir == "" {
		return fmt.Errorf("parent directory is empty")
	}
	abs, err := filepath.Abs(parentDir)
	if err != nil {
		return fmt.Errorf("resolve parent directory: %w", err)
	}
	if home, herr := os.UserHomeDir(); herr == nil {
		if homeAbs, aerr := filepath.Abs(home); aerr == nil && abs == homeAbs {
			return fmt.Errorf("refusing to run init --all directly on the home directory %s; pass a project workspace directory instead", abs)
		}
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return fmt.Errorf("read parent directory: %w", err)
	}
	var children []string
	for _, entry := range entries {
		child := filepath.Join(abs, entry.Name())
		info, statErr := os.Stat(child)
		if statErr != nil || !info.IsDir() || !hasAgentDoc(child) {
			continue
		}
		children = append(children, child)
	}
	sort.Strings(children)
	if len(children) == 0 {
		fmt.Printf("No eligible project directories found under %s\n", abs)
		return nil
	}
	quota1Active := len(quota1) > 0 && quota1[0]
	var errs []string
	for _, child := range children {
		fmt.Printf("== %s ==\n", filepath.Base(child))
		if err := RunInitWithVariant(child, cfg, docs, repoMode, true, withSummary, update, replace, issuesGit, gowork, false, variant, quota1Active); err != nil {
			fmt.Printf("  error: %v\n", err)
			errs = append(errs, fmt.Sprintf("%s: %v", child, err))
			continue
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("init --all encountered %d error(s):\n%s", len(errs), strings.Join(errs, "\n"))
	}
	return nil
}

func migrateLegacyMarkers(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content := string(data)
	if !strings.Contains(content, "<!-- claudeconfig:") && !strings.Contains(content, "# claudeconfig:") && !strings.Contains(content, "managed by claudeconfig") {
		return false, nil
	}
	newContent := strings.ReplaceAll(content, "<!-- claudeconfig:", "<!-- harnez:")
	newContent = strings.ReplaceAll(newContent, "# claudeconfig:", "# harnez:")
	newContent = strings.ReplaceAll(newContent, "managed by claudeconfig", "managed by harnez")
	if newContent == content {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(newContent), 0o644)
}

// CheckProjectDrift checks for inconsistencies between a project's go.mod module name,
// git remote origin URL repository name, and directory name.
func CheckProjectDrift(dir string) []string {
	var warnings []string
	goModPath := filepath.Join(dir, "go.mod")
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	dirBase := filepath.Base(absDir)

	var moduleBase string
	if data, err := os.ReadFile(goModPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				modPath := strings.TrimSpace(strings.TrimPrefix(line, "module"))
				parts := strings.Split(modPath, "/")
				if len(parts) > 0 {
					moduleBase = parts[len(parts)-1]
				}
				break
			}
		}
	}

	var repoBase string
	var remoteURL string
	if out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output(); err == nil {
		remoteURL = strings.TrimSpace(string(out))
		if remoteURL != "" {
			trimmed := strings.TrimSuffix(remoteURL, ".git")
			parts := strings.Split(trimmed, "/")
			if len(parts) > 0 {
				repoBase = parts[len(parts)-1]
				if colon := strings.LastIndex(repoBase, ":"); colon >= 0 {
					repoBase = repoBase[colon+1:]
				}
			}
		}
	}

	if moduleBase != "" && repoBase != "" && !strings.EqualFold(moduleBase, repoBase) {
		warnings = append(warnings, fmt.Sprintf("go.mod module %q does not match git origin repository %q (%s)", moduleBase, repoBase, remoteURL))
	}
	if moduleBase != "" && dirBase != "" && !strings.EqualFold(moduleBase, dirBase) {
		warnings = append(warnings, fmt.Sprintf("go.mod module %q does not match directory name %q", moduleBase, dirBase))
	}
	if repoBase != "" && dirBase != "" && !strings.EqualFold(repoBase, dirBase) {
		warnings = append(warnings, fmt.Sprintf("directory name %q does not match git origin repository %q (%s)", dirBase, repoBase, remoteURL))
	}

	return warnings
}
