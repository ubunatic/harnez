package claude

import (
	"encoding/json"
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

// validateDocNames returns an error naming any requested doc that is not
// defined in the config, so typos fail loudly instead of being skipped.
func validateDocNames(cfg *Config, names []string) error {
	var unknown []string
	for _, name := range names {
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
// It refuses the user's home directory, root directory, or non-coding directory unless force is true.
func ValidateInitTarget(dir string, force bool) error {
	if force {
		return nil
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve directory: %w", err)
	}

	if home, herr := os.UserHomeDir(); herr == nil {
		if homeAbs, aerr := filepath.Abs(home); aerr == nil && abs == homeAbs {
			return fmt.Errorf("refusing to run init directly on home directory %s; use --force to override", abs)
		}
	}

	if abs == "/" || filepath.Dir(abs) == abs {
		return fmt.Errorf("refusing to run init directly on root directory %s; use --force to override", abs)
	}

	if !isEligibleProjectDir(abs) {
		return fmt.Errorf("refusing to run init in %s: directory is not a Git repository and contains no recognized project or source files; use --force to override", abs)
	}

	return nil
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
// docs that declare a lite_source.
func RunInitWithVariant(dir string, cfg *Config, docs []string, repoMode string, assumeYes, withSummary, update, replace bool, issuesGit *bool, gowork bool, force bool, variant string) error {
	if err := ValidateInitTarget(dir, force); err != nil {
		return err
	}
	if cfg != nil {
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
	if fileExists(filepath.Join(dir, "issues")) {
		const issuesReadmeLock = "/issues/README.md.lock"
		ignored, err := fsutil.EnsureGitExclude(dir, issuesReadmeLock)
		if err != nil {
			return fmt.Errorf("ignore %s: %w", issuesReadmeLock, err)
		}
		if ignored {
			fmt.Printf("  ignored %s in .git/info/exclude\n", issuesReadmeLock)
			changes++
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

		// Apply config-defined local AGENTS.md sections (e.g. Language Conventions).
		if l := cfg.AgentsMD.Local; l.Target != "" {
			sections := append([]MDSection(nil), l.Sections...)
			if len(docs) > 0 {
				sections = append(sections, MDSection{
					Name:    "Language Conventions",
					Content: buildLangConventions(docs, cfg),
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
			data, err := fs.ReadFile(cfg.FS, lang.SourceFor(variant))
			if err != nil {
				return fmt.Errorf("language %s: read source: %w", name, err)
			}
			localDoc := localPath(dir, lang.Local)
			cr, err := writeFileIfChanged(localDoc, markdown.MergeManagedDoc(localDoc, data))
			if err != nil {
				return fmt.Errorf("language %s: copy: %w", name, err)
			}
			if cr.changed {
				changes++
				fmt.Printf("  copied %s → %s\n", lang.Source, localDoc)
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

	if changes == 0 {
		fmt.Println("No changes.")
	} else {
		fmt.Printf("%d change(s).\n", changes)
	}
	return nil
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
func RunInitAllWithVariant(parentDir string, cfg *Config, docs []string, repoMode string, withSummary, update, replace bool, issuesGit *bool, gowork bool, variant string) error {
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
	var errs []string
	for _, child := range children {
		fmt.Printf("== %s ==\n", filepath.Base(child))
		if err := RunInitWithVariant(child, cfg, docs, repoMode, true, withSummary, update, replace, issuesGit, gowork, false, variant); err != nil {
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

