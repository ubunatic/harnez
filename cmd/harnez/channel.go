package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

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
		Use: "setup", Short: "Print local plugin installation, MCP JSON and manual Claude launch commands",
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
		Use: "list", Short: "List live Claude channel endpoints and Codex sessions as JSON",
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
			listed := mergeChannelSessions(endpoints, func() ([]channel.CodexThread, error) {
				return channel.DiscoverCodexThreads(cmd.Context(), "codex", time.Duration(cfg.IOTimeoutMS)*time.Millisecond, cfg.MaxRPCBytes)
			})
			return json.NewEncoder(cmd.OutOrStdout()).Encode(listed)
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

type listedChannel struct {
	Provider   string `json:"provider"`
	SessionID  string `json:"session_id"`
	Name       string `json:"name,omitempty"`
	PID        int    `json:"pid,omitempty"`
	URL        string `json:"url,omitempty"`
	WorkingDir string `json:"working_dir"`
}

func mergeChannelSessions(endpoints []channel.Endpoint, discover func() ([]channel.CodexThread, error)) []listedChannel {
	listed := make([]listedChannel, 0, len(endpoints))
	for _, endpoint := range endpoints {
		listed = append(listed, listedChannel{
			Provider: "claude", SessionID: endpoint.SessionID, PID: endpoint.PID,
			URL: endpoint.URL, WorkingDir: endpoint.WorkingDir,
		})
	}
	threads, err := discover()
	if err == nil {
		for _, thread := range threads {
			listed = append(listed, listedChannel{
				Provider: "codex", SessionID: thread.ID, Name: thread.Name, WorkingDir: thread.CWD,
			})
		}
	}
	sort.Slice(listed, func(i, j int) bool {
		if listed[i].Provider == listed[j].Provider {
			return listed[i].SessionID < listed[j].SessionID
		}
		return listed[i].Provider < listed[j].Provider
	})
	return listed
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
claude --model %s --dangerously-load-development-channels server:%s --append-system-prompt %q

Accept the development-channel prompt, then "New MCP server found in this project"
with "Use this MCP server"; pending approval prevents endpoint registration.
Registry: %s
In another terminal: harnez channel list
Send: harnez channel send <session-id>
Default canary message: %s
Verify the returned message_id in the requested file in the target directory.
`, mcpData, settingsData, cfg.CanaryModel, cfg.ServerName, cfg.Instructions, cfg.RegistryDir, cfg.CanaryMessage)
	if err != nil {
		return err
	}
	pluginID := cfg.PluginName + "@" + cfg.MarketplaceName
	_, err = fmt.Fprintf(out, `
Local plugin route (requires the user-configured managed channel allowlist):
Install harnez on PATH first (make install from the Harnez checkout).
Run harnez apply to install and enable the managed harnez-local plugins.

For unattended plugin receipts, add this permission to .claude/settings.json:
{"permissions":{"allow":["mcp__plugin_%s_%s__%s"]}}

Then start a fresh session in the recipient directory:
claude --channels plugin:%s --append-system-prompt %q

Verify: harnez channel list
Send and wait: harnez channel send <session-id> (or harnez agent send --name <name> --wait <message>)
The plugin route avoids the development prompt only when this plugin is allowlisted.
`, cfg.PluginName, cfg.ServerName, cfg.ReplyToolName, pluginID, cfg.Instructions)
	return err
}
