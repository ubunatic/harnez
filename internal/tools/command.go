package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"time"

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
	voiceInput.AddCommand(mode, newVoiceInputRecordCommand(d), newVoiceInputHistoryCommand(d), newVoiceInputConfigCommand(d))
	addDebugVoiceCommands(voiceInput, d)
	return voiceInput
}

// newVoiceInputRecordCommand controls active audio dictation recording (start, stop, toggle, status).
func newVoiceInputRecordCommand(d Dependencies) *cobra.Command {
	record := &cobra.Command{Use: "record", Short: "Start, stop, or toggle active voice dictation recording"}

	toggle := &cobra.Command{Use: "toggle", Short: "Toggle dictation recording on or off", Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ControlRecording(cmd.Context(), d, RecordActionToggle)
		}}

	start := &cobra.Command{Use: "start", Short: "Start dictation recording", Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ControlRecording(cmd.Context(), d, RecordActionStart)
		}}

	stop := &cobra.Command{Use: "stop", Short: "Stop dictation recording", Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ControlRecording(cmd.Context(), d, RecordActionStop)
		}}

	status := &cobra.Command{Use: "status", Short: "Print recording status (idle or recording)", Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			stat, err := GetRecordingStatus(cmd.Context(), d)
			if err != nil {
				return err
			}
			fmt.Fprintln(d.Stdout, stat)
			return nil
		}}

	record.AddCommand(toggle, start, stop, status)
	return record
}

// newVoiceInputHistoryCommand exposes the local, sensitive dictation
// history: list/clear/copy/retype, plus the `record` pass-through hook
// meant to be wired as Voxtype's [output.post_process] command so history
// capture needs no changes to Voxtype itself.
func newVoiceInputHistoryCommand(d Dependencies) *cobra.Command {
	historyPath := func() string { return HistoryPath(d.Getenv("HOME")) }

	history := &cobra.Command{Use: "history", Short: "Recent dictation history (local-only, treat as sensitive)"}

	var limit int
	var format string
	list := &cobra.Command{Use: "list", Short: "List recent transcripts, most recent first", Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := ListHistory(historyPath())
			if err != nil {
				return err
			}
			if limit > 0 && len(entries) > limit {
				entries = entries[:limit]
			}
			if format == "json" {
				if entries == nil {
					entries = []HistoryEntry{}
				}
				enc := json.NewEncoder(d.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			for _, e := range entries {
				fmt.Fprintf(d.Stdout, "%s\t%s\t%s\n", e.ID, e.Time.Format("2006-01-02T15:04:05"), e.Text)
			}
			return nil
		}}
	list.Flags().IntVar(&limit, "limit", 0, "show at most N entries (default: all)")
	list.Flags().StringVar(&format, "format", "text", "output format (text or json)")

	clear := &cobra.Command{Use: "clear", Short: "Delete all recorded history", Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error { return ClearHistory(historyPath()) }}

	record := &cobra.Command{Use: "record", Short: "Internal: record stdin as a history entry and echo it back unchanged", Args: cobra.NoArgs, SilenceUsage: true,
		Long: "Reads a transcription from stdin, appends it to local history, and writes it back to stdout unchanged.\n" +
			"Wire this as Voxtype's [output.post_process] command so history capture needs no changes to Voxtype itself:\n\n" +
			"  [output.post_process]\n  command = \"harnez tools voice-input history record\"\n",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := io.ReadAll(d.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			text := string(data)
			if _, err := AppendHistory(historyPath(), text, DefaultHistoryLimit, time.Now()); err != nil {
				return err
			}
			_, err = fmt.Fprint(d.Stdout, text)
			return err
		}}

	copyCmd := &cobra.Command{Use: "copy ID", Short: "Copy a history entry's text to the clipboard", Args: cobra.ExactArgs(1), SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := FindHistoryEntry(historyPath(), args[0])
			if err != nil {
				return err
			}
			return CopyText(cmd.Context(), d, entry.Text)
		}}

	retype := &cobra.Command{Use: "retype ID", Short: "Type a history entry's text into the currently focused window", Args: cobra.ExactArgs(1), SilenceUsage: true,
		Long: "Types the entry's exact text via the dotool/dotoold primitive into whatever window currently has\n" +
			"focus. This performs NO focus capture or restoration -- make sure the right window is focused\n" +
			"first (a future GNOME Shell extension is responsible for that; see issue 022).\n",
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := FindHistoryEntry(historyPath(), args[0])
			if err != nil {
				return err
			}
			return TypeText(cmd.Context(), d, entry.Text)
		}}

	history.AddCommand(list, clear, record, copyCmd, retype)
	return history
}

// newVoiceInputConfigCommand exposes voxtype config.toml fields the UI can
// safely edit without a full TOML round-trip, starting with type_delay_ms.
func newVoiceInputConfigCommand(d Dependencies) *cobra.Command {
	configPath := func() string { return voxtypeConfigPath(d.Getenv("HOME")) }

	config := &cobra.Command{Use: "config", Short: "Read or change select voxtype config.toml fields"}

	get := &cobra.Command{Use: "get type-delay-ms", Short: "Print the current type_delay_ms value", Args: cobra.ExactArgs(1), SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "type-delay-ms" {
				return fmt.Errorf("unknown config key %q (want type-delay-ms)", args[0])
			}
			ms, ok, err := ReadTypeDelayMs(configPath())
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(d.Stdout, "0 (default; key not present in config.toml)")
				return nil
			}
			fmt.Fprintln(d.Stdout, ms)
			return nil
		}}

	set := &cobra.Command{Use: "set type-delay-ms MS", Short: "Set type_delay_ms, preserving comments and other settings", Args: cobra.ExactArgs(2), SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "type-delay-ms" {
				return fmt.Errorf("unknown config key %q (want type-delay-ms)", args[0])
			}
			ms, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid milliseconds %q: %w", args[1], err)
			}
			if err := SetTypeDelayMs(configPath(), ms); err != nil {
				return err
			}
			fmt.Fprintf(d.Stdout, "type_delay_ms set to %d in %s.\n"+
				"This does not take effect until voxtype's daemon restarts (not a live change):\n"+
				"  systemctl --user restart voxtype.service            # batch mode\n"+
				"  systemctl --user restart voxtype-streaming.service   # streaming mode (see 'harnez tools voice-input mode')\n",
				ms, configPath())
			return nil
		}}

	config.AddCommand(get, set)
	return config
}
