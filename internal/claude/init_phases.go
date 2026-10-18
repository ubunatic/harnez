package claude

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"ubunatic.com/harnez/internal/markdown"
)

// initReport accumulates the change count of one RunInitWithVariant run.
type initReport struct {
	changes int
}

// initLocalSections writes the config-defined local AGENTS.md sections (e.g. Language Conventions).
func initLocalSections(ir *initReport, cfg *Config, docs []string, repoMode string, agentsPath string, agentBlocksSafe bool) error {
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
			ir.changes++
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
			ir.changes++
		}
		printResult("wrote", agentsPath, lr)
		_ = presentNames
	}
	return nil
}

// initCopyDocs copies the selected language docs into the project and injects their Makefile targets.
func initCopyDocs(ir *initReport, assumeYes bool, cfg *Config, dir string, docs []string, docVariants map[string]string) error {
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
		warnStaleEmbeddedDoc(os.Stderr, dir, lang.SourceFor(docVariants[name]), data)
		localDoc := localPath(dir, lang.Local)
		cr, err := writeManagedDoc(localDoc, data)
		if err != nil {
			return fmt.Errorf("language %s: copy: %w", name, err)
		}
		if cr.changed {
			ir.changes++
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
				ir.changes++
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
					ir.changes++
					fmt.Printf("  reconciled %s\n", dest)
				} else {
					fmt.Printf("  exists  %s (targets unchanged)\n", dest)
				}
			}
		}
	}
	return nil
}

// initProjectSummary drafts the AGENTS.md project summary with claude -p (--summary).
func initProjectSummary(ir *initReport, dir string, withSummary bool, agentsPath string) error {
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
			ir.changes++
		} else {
			fmt.Printf("  exists  %s [%s] (unchanged)\n", agentsPath, summarySection)
		}
	}
	return nil
}
