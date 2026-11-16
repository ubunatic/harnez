package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/dependency"
	"ubunatic.com/harnez/internal/selfinstall"
)

func newInstallCmd() *cobra.Command {
	var opts selfinstall.Options
	cmd := &cobra.Command{
		Use:   "install [host | dependency]",
		Short: "Install harnez locally or over ssh, or install an optional dependency",
		Long: "Install or update harnez locally (into --dir, configuring shell PATH, completions,\n" +
			"the ⚙ symlink, and man pages) or onto a remote host over ssh.\n" +
			"When the argument names a package in spec/dependencies.yaml (such as neus),\n" +
			"it runs that dependency's installer instead.",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && dependency.HasPackage(args[0]) {
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
	return cmd
}
