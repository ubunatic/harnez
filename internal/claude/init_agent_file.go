package claude

import (
	"fmt"
	"os"
	"path/filepath"

	"ubunatic.com/harnez/internal/fsutil"
)

// initAgentFile creates or migrates AGENTS.md and reconciles the CLAUDE.md link.
// The result gates later section edits when existing managed blocks are malformed.
func initAgentFile(ir *initReport, dir string, cfg *Config, replace bool) (bool, error) {
	agentsPath := filepath.Join(dir, "AGENTS.md")
	claudePath := filepath.Join(dir, "CLAUDE.md")
	if replace {
		if err := os.Remove(agentsPath); err != nil && !os.IsNotExist(err) {
			return false, fmt.Errorf("remove %s: %w", agentsPath, err)
		}
	}

	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		if fi, err := os.Lstat(claudePath); err == nil && fi.Mode().IsRegular() {
			if err := os.Rename(claudePath, agentsPath); err != nil {
				return false, fmt.Errorf("migrate %s → %s: %w", claudePath, agentsPath, err)
			}
			fmt.Printf("  migrated %s → %s\n", claudePath, agentsPath)
			ir.changes++
		} else {
			if err := os.WriteFile(agentsPath, []byte(initialAgentsMD(cfg)), 0o644); err != nil {
				return false, fmt.Errorf("write %s: %w", agentsPath, err)
			}
			fmt.Printf("  created %s\n", agentsPath)
			ir.changes++
		}
	} else {
		if migrated, err := migrateLegacyMarkers(agentsPath); err != nil {
			return false, fmt.Errorf("migrate markers %s: %w", agentsPath, err)
		} else if migrated {
			fmt.Printf("  upgraded markers %s\n", agentsPath)
			ir.changes++
		} else {
			fmt.Printf("  exists  %s (unchanged)\n", agentsPath)
		}
	}
	if cfg != nil {
		if backfilled, err := backfillRulesHeader(agentsPath, cfg.AgentsMD.Rules.Header); err != nil {
			return false, fmt.Errorf("backfill rules header %s: %w", agentsPath, err)
		} else if backfilled {
			fmt.Printf("  updated rules header %s\n", agentsPath)
			ir.changes++
		}
	}
	agentBlocksSafe := true
	if data, err := os.ReadFile(agentsPath); err != nil {
		return false, fmt.Errorf("read %s for managed block migration: %w", agentsPath, err)
	} else if _, malformed := parseManagedBlocks(string(data), migratableAgentBlocks); malformed {
		agentBlocksSafe = false
	}

	symlinkChanged, err := fsutil.EnsureSymlink(claudePath, agentsPath)
	if err != nil {
		return false, fmt.Errorf("symlink %s: %w", claudePath, err)
	}
	if symlinkChanged {
		fmt.Printf("  symlink %s → %s\n", claudePath, agentsPath)
		ir.changes++
	} else {
		fmt.Printf("  exists  %s (unchanged)\n", claudePath)
	}
	return agentBlocksSafe, nil
}

// initGitIntegration maintains local excludes and issue-index Git integration.
func initGitIntegration(ir *initReport, dir string, repoConfig *RepoConfig) error {
	var err error
	_, _ = fsutil.EnsureGitExclude(dir, "AGENTS.local.md")
	if ignored, err := fsutil.EnsureGitExclude(dir, ".harnez/rules/Local.md"); err != nil {
		return fmt.Errorf("ignore .harnez/rules/Local.md: %w", err)
	} else if ignored {
		fmt.Printf("  ignored .harnez/rules/Local.md in .git/info/exclude\n")
		ir.changes++
	}
	// Per-session owner overrides written by /disable (issue 1188).
	if ignored, err := fsutil.EnsureGitExclude(dir, "/.harnez/state/"); err != nil {
		return fmt.Errorf("ignore .harnez/state/: %w", err)
	} else if ignored {
		fmt.Printf("  ignored .harnez/state/ in .git/info/exclude\n")
		ir.changes++
	}
	if fileExists(filepath.Join(dir, "issues")) || (repoConfig != nil && repoConfig.Issues.Tracker == "harnez") {
		addedGitignore, addedExcludes, err := ensureIssueLockIgnores(dir)
		if err != nil {
			return err
		}
		if addedGitignore {
			fmt.Printf("  ignored issue lock files in .gitignore\n")
			ir.changes++
		}
		for _, lock := range addedExcludes {
			fmt.Printf("  ignored %s in .git/info/exclude\n", lock)
			ir.changes++
		}
	}
	var gitChanges int
	if repoConfig != nil && repoConfig.Issues.Tracker == "harnez" && repoConfig.Issues.MergeDriver {
		gitChanges, err = installIssuesGitIntegration(dir)
	} else {
		gitChanges, err = removeIssuesGitIntegration(dir)
	}
	if err != nil {
		return err
	}
	ir.changes += gitChanges
	return nil
}

// ensureIssueLockIgnores adds issue-tracker lock exclusions to .gitignore and
// .git/info/exclude (issues 672, 703, 1066).
func ensureIssueLockIgnores(dir string) (addedGitignore bool, addedExcludes []string, err error) {
	locks := []string{"/issues/README.md.lock", "/issues/.reserve.lock"}
	addedGitignore, err = ensureGitignoreSection(dir, "# harnez issue-index lock files", locks...)
	if err != nil {
		return false, nil, fmt.Errorf("ignore issue locks in .gitignore: %w", err)
	}
	for _, lock := range locks {
		ignored, err := fsutil.EnsureGitExclude(dir, lock)
		if err != nil {
			return addedGitignore, addedExcludes, fmt.Errorf("ignore %s: %w", lock, err)
		}
		if ignored {
			addedExcludes = append(addedExcludes, lock)
		}
	}
	return addedGitignore, addedExcludes, nil
}
