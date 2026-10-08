package main

import (
	"context"
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
	list := &cobra.Command{
		Use: "list", Short: "List Claude channel endpoints and Codex sessions",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChannelList(cmd, args, channel.List, func(ctx context.Context, cfg channel.Config) ([]channel.CodexThread, error) {
				return channel.DiscoverCodexThreads(ctx, "codex", time.Duration(cfg.IOTimeoutMS)*time.Millisecond, cfg.MaxRPCBytes)
			})
		},
	}
	list.Flags().Bool("all", false, "show inactive and stale sessions too")
	list.Flags().Bool("names", false, "print session names only (falls back to IDs)")
	list.Flags().Bool("ids", false, "print session IDs only")
	list.Flags().Bool("json", false, "print full session details as JSON")
	cmd.AddCommand(list)
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

func runChannelList(cmd *cobra.Command, _ []string, listEndpoints func(context.Context, channel.Config) ([]channel.Endpoint, error), discover func(context.Context, channel.Config) ([]channel.CodexThread, error)) error {
	allSessions, _ := cmd.Flags().GetBool("all")
	namesOnly, _ := cmd.Flags().GetBool("names")
	idsOnly, _ := cmd.Flags().GetBool("ids")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	if namesOnly && idsOnly {
		return fmt.Errorf("--names and --ids are mutually exclusive")
	}
	if jsonOutput && (namesOnly || idsOnly) {
		return fmt.Errorf("--json cannot be combined with --names or --ids")
	}
	cfg, err := channel.LoadConfig()
	if err != nil {
		return err
	}
	endpoints, err := listEndpoints(cmd.Context(), cfg)
	if err != nil {
		return err
	}
	threads, discoverErr := discover(cmd.Context(), cfg)
	currentThreadID := os.Getenv("CODEX_THREAD_ID")
	if currentThreadID == "" {
		currentThreadID = os.Getenv("CODEX_SESSION_ID")
	}
	listed := mergeChannelSessions(endpoints, func() ([]channel.CodexThread, error) { return threads, discoverErr }, currentThreadID)
	if !allSessions {
		listed = filterLiveChannelSessions(listed)
	}
	if jsonOutput {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(listed)
	}
	if namesOnly {
		return printChannelProjection(cmd.OutOrStdout(), listed, true)
	}
	if idsOnly {
		return printChannelProjection(cmd.OutOrStdout(), listed, false)
	}
	return printChannelOverview(cmd.OutOrStdout(), listed)
}

type listedChannel struct {
	Provider   string `json:"provider"`
	SessionID  string `json:"session_id"`
	Name       string `json:"name,omitempty"`
	PID        int    `json:"pid,omitempty"`
	URL        string `json:"url,omitempty"`
	WorkingDir string `json:"working_dir"`
	Live       bool   `json:"-"`
}

func filterLiveChannelSessions(listed []listedChannel) []listedChannel {
	live := make([]listedChannel, 0, len(listed))
	for _, session := range listed {
		if isLiveChannelSession(session) {
			live = append(live, session)
		}
	}
	return live
}

func isLiveChannelSession(session listedChannel) bool {
	return (session.Provider == "claude" && session.PID > 0 && session.URL != "") ||
		(session.Provider == "codex" && session.Live)
}

func printChannelProjection(out io.Writer, listed []listedChannel, names bool) error {
	for _, session := range listed {
		value := session.SessionID
		if names {
			value = session.Name
			if value == "" {
				value = session.SessionID
			}
		}
		if value == "" {
			continue
		}
		if _, err := fmt.Fprintln(out, value); err != nil {
			return err
		}
	}
	return nil
}

func printChannelOverview(out io.Writer, listed []listedChannel) error {
	if _, err := fmt.Fprintln(out, "PROVIDER\tNAME\tID\tSTATUS"); err != nil {
		return err
	}
	for _, session := range listed {
		name := session.Name
		if name == "" {
			name = "-"
		}
		status := "inactive"
		if isLiveChannelSession(session) {
			status = "live"
		}
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", session.Provider, name, session.SessionID, status); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out, `Use "harnez channel list --json" or "harnez channel inspect <id>" for full details`)
	return err
}

func mergeChannelSessions(endpoints []channel.Endpoint, discover func() ([]channel.CodexThread, error), currentThreadID string) []listedChannel {
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
				Live: thread.ID == currentThreadID && currentThreadID != "",
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
