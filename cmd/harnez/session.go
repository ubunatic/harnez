package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/transcript"
)

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Extract retrospective evidence from local Claude Code sessions",
	}
	var asJSON bool
	extract := &cobra.Command{
		Use:   "extract <id|path>",
		Short: "Stream compaction summaries and owner messages from a Claude transcript",
		Long: `Extract all compaction summaries and real owner messages in transcript order.
Accepts a JSONL path or unique session ID prefix under ~/.claude/projects/.
Tool results, system reminders, task notifications and skill injections are removed.
Slash-command names and arguments and image placeholders are preserved.
Only Claude Code transcripts are supported. No transcript is modified.

Output includes source path/size, timestamps, counts and a truncated-tail indicator.
--json emits one JSON object with source, records and stats; default output is text.
Redirect output to a private file before reading bounded sections for a retro.
Check the exit status before consuming output: errors can leave a partial extract.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			path, err := transcript.Resolve(filepath.Join(home, ".claude"), args[0])
			if err != nil {
				return err
			}
			return transcript.WriteFile(cmd.OutOrStdout(), path, asJSON)
		},
	}
	extract.Flags().BoolVar(&asJSON, "json", false, "emit a JSON object with source, records and stats")
	cmd.AddCommand(extract)
	return cmd
}
