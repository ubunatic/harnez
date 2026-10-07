package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/channel"
)

func newChannelCmd() *cobra.Command {
	var listenAddress string
	cmd := &cobra.Command{
		Use: "channel", Short: "Serve the opt-in local Claude MCP channel canary",
		Long: "Serve the opt-in local Claude MCP channel. The user starts Claude manually. At first project startup, accept 'New MCP server found in this project' with 'Use this MCP server'; until approval the server stays pending and no endpoint registers. After installing an updated binary, restart the MCP server (or the Claude session) to load the reply tool.",
		Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := channel.LoadConfig()
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("listen") {
				cfg.ListenAddress = listenAddress
			}
			input, ok := cmd.InOrStdin().(io.ReadCloser)
			if !ok {
				input = io.NopCloser(cmd.InOrStdin())
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return (channel.Server{Config: cfg, SessionID: os.Getenv("CLAUDE_CODE_SESSION_ID"), In: input, Out: cmd.OutOrStdout()}).Run(ctx)
		},
	}
	cmd.Flags().StringVar(&listenAddress, "listen", "", "numeric loopback bind address (default from spec/channel.yaml)")
	cmd.AddCommand(&cobra.Command{
		Use: "setup", Short: "Print MCP JSON and the command for a manually started Claude session",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := channel.LoadConfig()
			if err != nil {
				return err
			}
			executable, err := os.Executable()
			if err != nil {
				return err
			}
			return printChannelSetup(cmd.OutOrStdout(), cfg, executable)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use: "list", Short: "List live channel endpoints by Claude session ID as JSON",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := channel.LoadConfig()
			if err != nil {
				return err
			}
			endpoints, err := channel.List(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(endpoints)
		},
	})
	var message string
	send := &cobra.Command{
		Use: "send <session-id>", Short: "Send a canary message; accepted does not confirm Claude receipt",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := channel.LoadConfig()
			if err != nil {
				return err
			}
			content := message
			if !cmd.Flags().Changed("message") {
				content = cfg.CanaryMessage
			}
			result, err := channel.Send(cmd.Context(), cfg, args[0], content)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
	send.Flags().StringVarP(&message, "message", "m", "", "message text (default: spec canary asks Claude to record its message ID)")
	cmd.AddCommand(send)
	return cmd
}

func printChannelSetup(out io.Writer, cfg channel.Config, executable string) error {
	mcpSnippet := map[string]any{"mcpServers": map[string]any{
		cfg.ServerName: map[string]any{"type": "stdio", "command": executable, "args": []string{"channel"}},
	}}
	mcpData, err := json.MarshalIndent(mcpSnippet, "", "  ")
	if err != nil {
		return err
	}
	settingsSnippet := map[string]any{"permissions": map[string]any{
		"allow": []string{fmt.Sprintf("mcp__%s__%s", cfg.ServerName, cfg.ReplyToolName)},
	}}
	settingsData, err := json.MarshalIndent(settingsSnippet, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, `Merge this entry into .mcp.json in a scratch directory (preserve existing servers):
%s

Optional (for unattended receipts without per-call approval dialogs):
merge this entry into .claude/settings.json in that directory:
%s

In that directory, manually run:
claude --model %s --dangerously-load-development-channels server:%s

Optional launch line with explicit startup trust prompt:
claude --model %s --dangerously-load-development-channels server:%s --append-system-prompt %q

Accept the development-channel prompt, then "New MCP server found in this project"
with "Use this MCP server"; pending approval prevents endpoint registration.
Registry: %s
In another terminal: harnez channel list
Send: harnez channel send <session-id>
Default canary message: %s
Verify the returned message_id in the requested file in the target directory.
`, mcpData, settingsData, cfg.CanaryModel, cfg.ServerName, cfg.CanaryModel, cfg.ServerName, cfg.Instructions, cfg.RegistryDir, cfg.CanaryMessage)
	return err
}
