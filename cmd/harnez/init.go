package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	var initWizard, initNoWizard bool

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
			opts := claude.InitOptions{
				Docs: initDocs, RepoMode: initRepoMode, AssumeYes: initYes,
				WithSummary: initSummary, Update: initUpdate, Replace: initReplace,
				IssuesGit: issuesGit, GoWork: initGoWork, Force: initForce,
				Variant: initVariant, Quota1: initQuota1,
			}
			input, output := cmd.InOrStdin(), cmd.OutOrStdout()
			if initWizardEnabled(initWizard, initNoWizard, initYes, initAll, os.Getenv("CI") != "", initStreamTTY(input), initStreamTTY(output)) {
				opts.WizardInput, opts.WizardOutput = input, output
			}
			if initAll {
				return claude.RunInitAllWithVariant(initDir, cfg, opts)
			}
			return claude.RunInitWithVariant(initDir, cfg, opts)
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
	initCmd.Flags().BoolVarP(&initYes, "yes", "y", false, "use detected/recorded setup and reconcile Makefile targets without prompts")
	initCmd.Flags().BoolVar(&initWizard, "wizard", false, "prompt for repository features even without a terminal (EOF keeps defaults)")
	initCmd.Flags().BoolVar(&initNoWizard, "no-wizard", false, "skip repository feature prompts")
	initCmd.MarkFlagsMutuallyExclusive("wizard", "no-wizard")
	initCmd.Flags().BoolVar(&initSummary, "summary", false, "run claude -p to generate a project summary and add it to AGENTS.md")
	initCmd.Flags().BoolVar(&initUpdate, "update", false, "re-fetch and refresh the project summary (implies --summary)")
	initCmd.Flags().BoolVar(&initReplace, "replace", false, "delete existing AGENTS.md and recreate from template before init")
	initCmd.Flags().BoolVar(&initIssuesGit, "issues-git", false, "record issue-index Git integration for a Harnez tracker (use --issues-git=false to remove it)")
	initCmd.Flags().BoolVar(&initGoWork, "gowork", false, "set up or migrate Go workspace (go.work.example + untracked local go.work symlink)")
	initCmd.Flags().BoolVar(&initQuota1, "quota-1", false, "scaffold Quota-1 single-test guardrails in AGENTS.md and Makefile")
	initCmd.Flags().BoolVar(&initRAMP, "ramp", false, "report RAMP maturity profile and projected init changes (offline, read-only)")
	initCmd.Flags().BoolVar(&initRAMPReport, "ramp-report", false, "alias for --ramp")
	initCmd.Flags().BoolVar(&initJSON, "json", false, "output RAMP report in JSON format")

	return initCmd
}

func initWizardEnabled(force, disabled, yes, all, ci, inputTTY, outputTTY bool) bool {
	return !disabled && !yes && !all && (force || (!ci && inputTTY && outputTTY))
}

func initStreamTTY(stream any) bool {
	file, ok := stream.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}
