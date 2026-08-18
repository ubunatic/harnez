package tools

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/spf13/cobra"
)

// NewCommand creates the independent tools command.
func NewCommand(specFS fs.FS, d Dependencies) (*cobra.Command, error) {
	catalog, err := LoadCatalog(specFS)
	if err != nil {
		return nil, err
	}
	find := func(id string) (Tool, error) {
		for _, tool := range catalog.Tools {
			if tool.ID == id {
				return tool, nil
			}
		}
		return Tool{}, fmt.Errorf("unknown tool %q", id)
	}
	show := func(ctx context.Context, ids []string, requireReady bool) error {
		host := DetectHost(d)
		for _, tool := range catalog.Tools {
			if len(ids) > 0 && ids[0] != tool.ID {
				continue
			}
			status := Probe(ctx, tool, host, d)
			fmt.Fprintf(d.Stdout, "%-14s %-11s %s\n", tool.ID, status.State, status.Detail)
			if requireReady && status.State != Ready {
				return fmt.Errorf("%s is %s", tool.ID, status.State)
			}
		}
		return nil
	}
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List and manage optional workstation capabilities",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return show(cmd.Context(), nil, false) },
	}
	status := &cobra.Command{
		Use:          "status [voice-input]",
		Short:        "Show tool readiness without changing the host",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				if _, err := find(args[0]); err != nil {
					return err
				}
			}
			return show(cmd.Context(), args, len(args) == 1)
		},
	}
	var options InstallOptions
	install := &cobra.Command{
		Use:          "install voice-input",
		Short:        "Plan a workstation capability install",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			tool, err := find(args[0])
			if err != nil {
				return err
			}
			return Install(cmd.Context(), tool, DetectHost(d), d, options)
		},
	}
	install.Flags().StringVar(&options.Scope, "scope", "user", "installation scope (user or system)")
	install.Flags().BoolVar(&options.DryRun, "dry-run", false, "print the plan without network access or changes")
	install.Flags().BoolVarP(&options.Yes, "yes", "y", false, "approve the printed plan noninteractively")

	cmd.AddCommand(status, install, newVoiceInputCommand(d))
	return cmd, nil
}

// newVoiceInputCommand delegates voice-input commands to the standalone voxi engine.
func newVoiceInputCommand(d Dependencies) *cobra.Command {
	voiceInput := &cobra.Command{
		Use:                "voice-input [args...]",
		Short:              "Manage local voice-input dictation (delegates to standalone voxi engine)",
		Long:               "Voice input engine capabilities have been extracted into the standalone 'voxi' tool (ubunatic/voxi).\nThis command delegates to the installed 'voxi' CLI or provides installation guidance.",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			voxiPath, err := d.LookPath("voxi")
			if err != nil {
				fmt.Fprintln(d.Stdout, "voxi (ubunatic/voxi) is not installed on PATH.")
				fmt.Fprintln(d.Stdout, "Voice input engine has been extracted into a standalone tool.")
				fmt.Fprintln(d.Stdout, "\nTo install voxi:")
				fmt.Fprintln(d.Stdout, "  go install ubunatic.com/voxi/cmd/voxi@latest")
				fmt.Fprintln(d.Stdout, "  # or from source repository:")
				fmt.Fprintln(d.Stdout, "  cd ~/projects/voxi && make install")
				return nil
			}

			if len(args) == 0 {
				args = []string{"--help"}
			}

			if d.Run != nil {
				return d.Run(ctx, voxiPath, args...)
			}
			return nil
		},
	}
	return voiceInput
}
