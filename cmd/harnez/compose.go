package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/channel"
)

func newComposeCmd() *cobra.Command {
	names := []string{"claude-channels-prompt", "claude-channels-args"}
	return &cobra.Command{
		Use: "compose [what]", Short: "Print a ready-to-use snippet from the embedded spec",
		Long: `Print a snippet without a banner. With no name, list available snippets.
Run harnez apply first to install and enable the managed channel plugin.
Channel snippets use the same spec/channel.yaml instructions as channel setup.
The args snippet is shell-quoted; evaluate it to preserve the prompt as one argument.`,
		Example: `  claude --channels plugin:harnez-channel@harnez-local --append-system-prompt "$(harnez compose claude-channels-prompt)"
  alias claude-channels='claude --channels plugin:harnez-channel@harnez-local --append-system-prompt "$(harnez compose claude-channels-prompt)"'
  claude_channels() {
    eval "command claude $(harnez compose claude-channels-args) \"\$@\""
  }`,
		Args: cobra.MaximumNArgs(1), ValidArgs: names, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "Available snippets:")
				if err != nil {
					return err
				}
				for _, name := range names {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), name); err != nil {
						return err
					}
				}
				return nil
			}
			if args[0] != names[0] && args[0] != names[1] {
				return fmt.Errorf("unknown snippet %q; available: %s, %s", args[0], names[0], names[1])
			}
			cfg, err := channel.LoadConfig()
			if err != nil {
				return err
			}
			snippet := cfg.Instructions
			if args[0] == "claude-channels-args" {
				snippet = composeChannelArgs(cfg)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), snippet)
			return err
		},
	}
}

func composeChannelArgs(cfg channel.Config) string {
	return "--channels " + shellQuote("plugin:"+cfg.PluginName+"@"+cfg.MarketplaceName) +
		" --append-system-prompt " + shellQuote(cfg.Instructions)
}
