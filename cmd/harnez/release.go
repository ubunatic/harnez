package main

import (
	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/release"
)

func newReleaseCmd() *cobra.Command {
	var opt release.Options

	cmd := &cobra.Command{
		Use:   "release",
		Short: "Language-agnostic version bump, build, minisign, and forge publish pipeline",
		Long: `Automates the full project release lifecycle:
  1. Reads or initializes the single source of truth version.yaml spec.
  2. Bumps semver (patch, minor, major, or explicit version) or resumes (--continue).
  3. Propagates version to language files (version.go, __version__.py, build.zig.zon, etc.).
  4. Runs GoReleaser or custom build command to produce dist/ artifacts.
  5. Cryptographically signs SHA256SUMS with minisign (non-interactive).
  6. Verifies/enables Forgejo/Codeberg repository 'has_releases' unit via API.
  7. Tags and pushes git branch and tags to remote origin.
  8. Publishes release and attaches artifacts to Codeberg/Forgejo via fj.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opt.Out = cmd.OutOrStdout()
			if len(args) > 0 && opt.Bump == "" {
				opt.Bump = args[0]
			}
			return release.Run(opt)
		},
	}

	cmd.Flags().StringVarP(&opt.Bump, "bump", "b", "patch", "version bump type ('patch', 'minor', 'major', or explicit semver 'x.y.z')")
	cmd.Flags().BoolVar(&opt.Continue, "continue", false, "resume a previous release safely without bumping or re-tagging")
	cmd.Flags().BoolVarP(&opt.DryRun, "dry-run", "n", false, "simulate execution without modifying files, git, or remote forge")
	cmd.Flags().BoolVar(&opt.Force, "force", false, "re-release the current code as a new version even if nothing changed since the previous tag")
	cmd.Flags().StringVarP(&opt.SignKey, "sign-key", "s", "", "path to secret minisign key (defaults to ~/.minisign/<project>.key)")
	cmd.Flags().StringVarP(&opt.Dir, "dir", "d", ".", "target project directory")
	cmd.Flags().StringVar(&opt.BuildCmd, "build-cmd", "", "custom build command (defaults to goreleaser or make dist)")
	cmd.Flags().StringVar(&opt.TagPrefix, "tag-prefix", "", "override git tag prefix (defaults to tag_prefix in version.yaml or 'v')")
	cmd.Flags().BoolVar(&opt.SkipBuild, "skip-build", false, "skip artifact compilation/packaging")
	cmd.Flags().BoolVar(&opt.SkipSign, "skip-sign", false, "skip minisign cryptographic signing")
	cmd.Flags().BoolVar(&opt.SkipPublish, "skip-publish", false, "skip publishing artifacts to forge via fj")
	cmd.Flags().BoolVar(&opt.SkipPush, "skip-push", false, "skip pushing git commit and tags to remote")

	return cmd
}
