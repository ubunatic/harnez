package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"ubunatic.com/harnez/internal/dependency"
	"ubunatic.com/harnez/internal/selfinstall"
)

func newInstallCmd() *cobra.Command {
	var opts selfinstall.Options
	var stagedDir string
	var stagedSHA256 string
	cmd := &cobra.Command{
		Use:   "install [host | dependency]",
		Short: "Install harnez locally or over ssh, or install an optional dependency",
		Long: "Install or update harnez locally (into --dir, configuring shell PATH, completions,\n" +
			"the ⚙ symlink, and man pages), promote a privately staged build into trusted\n" +
			"host roots (--staged <dir>), or install onto a remote host over ssh.\n" +
			"When the argument names a package in spec/dependencies.yaml (such as neus),\n" +
			"it runs that dependency's installer instead.",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if stagedDir != "" {
				if len(args) > 0 {
					return fmt.Errorf("--staged cannot be combined with a host or dependency argument")
				}
				for _, name := range []string{"strategy", "dir", "version", "allow-dirty", "direct", "no-shell", "dry-run"} {
					if cmd.Flags().Changed(name) {
						return fmt.Errorf("flag --%s cannot be combined with --staged", name)
					}
				}
				manifest, err := expectedManManifest(cmd.Root())
				if err != nil {
					return err
				}
				_, err = selfinstall.PromoteStaged(cmd.Context(), selfinstall.PromoteOptions{
					StagedDir:        stagedDir,
					ExpectedManFiles: manifest,
					ExpectedSHA256:   stagedSHA256,
				}, cmd.OutOrStdout())
				return err
			}
			if stagedSHA256 != "" {
				return fmt.Errorf("--sha256 requires --staged")
			}
			if len(args) == 1 && dependency.HasPackage(args[0]) {
				if os.Getenv("TAIA_SESSION_DIR") != "" && !opts.DryRun {
					return fmt.Errorf("harnez install %s: host-only operation in runner mode", args[0])
				}
				for _, name := range []string{"strategy", "dir", "version", "allow-dirty", "direct", "no-shell", "no-man"} {
					if cmd.Flags().Changed(name) {
						return fmt.Errorf("flag --%s cannot be used when installing dependency %q", name, args[0])
					}
				}
				return dependency.Install(cmd.Context(), args[0], opts.DryRun, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			}
			if len(args) == 1 {
				opts.Host = args[0]
			}
			opts.InstallMan = func(out io.Writer) error {
				return installManPages(cmd.Root(), "", out)
			}
			return selfinstall.Run(cmd.Context(), opts, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&opts.Strategy, "strategy", "auto", "Binary source: auto, self, build, release, or go")
	cmd.Flags().StringVar(&opts.Dir, "dir", selfinstall.DefaultDir, "Install directory on the target")
	cmd.Flags().StringVar(&opts.Version, "version", "", "Version or git ref for the release and go strategies")
	cmd.Flags().BoolVar(&opts.AllowDirty, "allow-dirty", false, "Allow build strategy with uncommitted changes to a remote target")
	cmd.Flags().BoolVar(&opts.Direct, "direct", false, "Use GOPROXY=direct for the go strategy")
	cmd.Flags().BoolVar(&opts.NoShell, "no-shell", false, "Skip shell PATH and completion setup on local install")
	cmd.Flags().BoolVar(&opts.NoMan, "no-man", false, "Skip installing roff man pages on local install")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Print actions without installing")
	cmd.Flags().StringVar(&stagedDir, "staged", "", "Promote a privately staged build into trusted host GOBIN/GOPATH/bin and XDG man roots")
	cmd.Flags().StringVar(&stagedSHA256, "sha256", "", "Expected 64-hex SHA-256 digest of staged bin/harnez when using --staged")
	return cmd
}

func expectedManManifest(rootCmd *cobra.Command) ([]string, error) {
	tmpDir, err := os.MkdirTemp("", "harnez-man-manifest-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)
	if err := doc.GenManTree(rootCmd, manHeader(), tmpDir); err != nil {
		return nil, fmt.Errorf("generate expected man manifest: %w", err)
	}
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".1") {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}
