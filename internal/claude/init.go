package claude

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"ubunatic.com/claudeconfig/internal/fsutil"
	"ubunatic.com/claudeconfig/internal/markdown"
)

const agentsMDTemplate = `Adhere to the following conventions.

<!-- claudeconfig:begin Project Summary -->
<!-- claudeconfig:end Project Summary -->

## Development Scripts

Run from project root.

`

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
		if strings.HasPrefix(strings.TrimSpace(line), "<!-- claudeconfig:") {
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

// detectDoc returns true if project dir contains signals for the named doc.
func detectDoc(dir, name string) bool {
	switch name {
	case "golang":
		return fileExists(filepath.Join(dir, "go.mod"))
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
		return globExists(dir, "*.cpp") || globExists(dir, "*.cc") ||
			globExists(dir, "*.h") || fileExists(filepath.Join(dir, "CMakeLists.txt"))
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

// RunInit creates AGENTS.md and CLAUDE.md symlink in a project directory,
// applies config-defined local sections, and sets up language docs and Makefile targets.
func RunInit(dir string, cfg *Config, docs []string, repoMode string, assumeYes, withSummary, update, replace bool) error {
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
		if err := os.WriteFile(agentsPath, []byte(agentsMDTemplate), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", agentsPath, err)
		}
		fmt.Printf("  created %s\n", agentsPath)
		changes++
	} else {
		fmt.Printf("  exists  %s (unchanged)\n", agentsPath)
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

	if cfg != nil {
		if err := validateDocNames(cfg, docs); err != nil {
			return err
		}
		docs = append(docs, autoDetectDocs(dir, cfg, docs)...)

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
			data, err := fs.ReadFile(cfg.FS, lang.Source)
			if err != nil {
				return fmt.Errorf("language %s: read source: %w", name, err)
			}
			localDoc := localPath(dir, lang.Local)
			cr, err := writeFileIfChanged(localDoc, data)
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
