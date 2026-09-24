package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/assess"
	"ubunatic.com/harnez/internal/claude"
)

func newInitCmd() *cobra.Command {
	var initDir string
	var initDocs []string
	var initConfigPath string
	var initRepoMode string
	var initVariant string
	var initSummary, initUpdate, initReplace, initYes, initAll, initIssuesGit, initGoWork, initForce, initQuota1 bool
	var initRAMP, initRAMPReport, initJSON bool

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Set up a project directory with AGENTS.md, language docs, and Makefile targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := claude.OpenConfig(initConfigPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if initRAMP || initRAMPReport {
				projected := claude.ProjectedInitFiles(initDir, cfg, initDocs)
				profile, err := assess.AssessRAMPWithProjected(initDir, projected)
				if err != nil {
					return fmt.Errorf("assess RAMP %s: %w", initDir, err)
				}
				if initJSON {
					out, err := assess.RenderRAMPJSON(profile)
					if err != nil {
						return err
					}
					fmt.Println(out)
					return nil
				}
				fmt.Print(assess.RenderRAMPInitText(profile))
				return nil
			}
			var issuesGit *bool
			if cmd.Flags().Changed("issues-git") {
				issuesGit = &initIssuesGit
			}
			if initAll {
				return claude.RunInitAllWithVariant(initDir, cfg, initDocs, initRepoMode, initSummary, initUpdate, initReplace, issuesGit, initGoWork, initVariant, initQuota1)
			}
			return claude.RunInitWithVariant(initDir, cfg, initDocs, initRepoMode, initYes, initSummary, initUpdate, initReplace, issuesGit, initGoWork, initForce, initVariant, initQuota1)
		},
	}
	initCmd.Flags().StringVarP(&initConfigPath, "config", "c", "", "path to config YAML file (default: embedded)")
	initCmd.Flags().StringVarP(&initDir, "dir", "d", ".", "project directory to initialise (default: current directory)")
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "allow initializing home directory, root, or non-coding directory")
	initCmd.Flags().BoolVar(&initAll, "all", false,
		"treat --dir as a workspace directory and non-interactively init every eligible child (has AGENTS.md/CLAUDE.md); refuses $HOME (see issue 068)")
	initCmd.Flags().StringSliceVar(&initDocs, "docs", nil, "docs to set up in the project, comma-separated or repeated (e.g. golang,canary)")
	initCmd.Flags().StringVar(&initVariant, "variant", "", "doc variant to install: lite or full (default: keep each existing doc variant, else full)")
	initCmd.Flags().StringVarP(&initRepoMode, "repo-mode", "m", "", "repo git setup to note in AGENTS.md (solo, fork, team)")
	initCmd.Flags().BoolVarP(&initYes, "yes", "y", false, "assume yes when reconciling Makefile targets (no prompt)")
	initCmd.Flags().BoolVar(&initSummary, "summary", false, "run claude -p to generate a project summary and add it to AGENTS.md")
	initCmd.Flags().BoolVar(&initUpdate, "update", false, "re-fetch and refresh the project summary (implies --summary)")
	initCmd.Flags().BoolVar(&initReplace, "replace", false, "delete existing AGENTS.md and recreate from template before init")
	initCmd.Flags().BoolVar(&initIssuesGit, "issues-git", false, "enable issue-index Git integration (use --issues-git=false to remove it)")
	initCmd.Flags().BoolVar(&initGoWork, "gowork", false, "set up or migrate Go workspace (go.work.example + untracked local go.work symlink)")
	initCmd.Flags().BoolVar(&initQuota1, "quota-1", false, "scaffold Quota-1 single-test guardrails in AGENTS.md and Makefile")
	initCmd.Flags().BoolVar(&initRAMP, "ramp", false, "report RAMP maturity profile and projected init changes (offline, read-only)")
	initCmd.Flags().BoolVar(&initRAMPReport, "ramp-report", false, "alias for --ramp")
	initCmd.Flags().BoolVar(&initJSON, "json", false, "output RAMP report in JSON format")

	return initCmd
}
