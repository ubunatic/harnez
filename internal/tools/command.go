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
	cmd := &cobra.Command{Use: "tools", Short: "List and manage optional workstation capabilities", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return show(cmd.Context(), nil, false) }}
	status := &cobra.Command{Use: "status [voice-input]", Short: "Show tool readiness without changing the host", Args: cobra.MaximumNArgs(1), SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				if _, err := find(args[0]); err != nil {
					return err
				}
			}
			return show(cmd.Context(), args, len(args) == 1)
		}}
	var options InstallOptions
	install := &cobra.Command{Use: "install voice-input", Short: "Plan a workstation capability install", Args: cobra.ExactArgs(1), SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			tool, err := find(args[0])
			if err != nil {
				return err
			}
			return Install(cmd.Context(), tool, DetectHost(d), d, options)
		}}
	install.Flags().StringVar(&options.Scope, "scope", "user", "installation scope (user or system)")
	install.Flags().BoolVar(&options.DryRun, "dry-run", false, "print the plan without network access or changes")
	install.Flags().BoolVarP(&options.Yes, "yes", "y", false, "approve the printed plan noninteractively")
	cmd.AddCommand(status, install, newVoiceInputCommand(d))
	return cmd, nil
}

// newVoiceInputCommand groups voice-input host-state commands that are
// independent of the install/status catalog flow, starting with the
// streaming/batch mode toggle from issue 021.
func newVoiceInputCommand(d Dependencies) *cobra.Command {
	voiceInput := &cobra.Command{Use: "voice-input", Short: "Manage the local voice-input daemon's runtime mode"}
	mode := &cobra.Command{Use: "mode [streaming|batch]", Short: "Show or switch the active voice-input mode", Args: cobra.MaximumNArgs(1), SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if len(args) == 0 {
				fmt.Fprintln(d.Stdout, DescribeVoiceInputMode(CurrentVoiceInputMode(ctx, d)))
				return nil
			}
			var target VoiceInputMode
			switch args[0] {
			case "streaming":
				target = ModeStreaming
			case "batch":
				target = ModeBatch
			default:
				return fmt.Errorf("invalid mode %q (want streaming or batch)", args[0])
			}
			return SwitchVoiceInputMode(ctx, d, target)
		}}
	voiceInput.AddCommand(mode)
	return voiceInput
}
