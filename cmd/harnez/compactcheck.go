// compactcheck implements `harnez compact-check`, a manual/opt-in comparator
// for assessing whether a /compact likely dropped information that
// mattered. See issue 180: Claude Code exposes a PreCompact hook but no
// PostCompact hook, so there is no host-native signal to watch compaction
// automatically — this command lets a user or agent run the same cheap
// heuristic manually against two saved transcript/summary snapshots.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/compactcheck"
)

func newCompactCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compact-check <before-file> <after-file>",
		Short: "Assess whether a /compact likely dropped information (manual, opt-in)",
		Long: `compact-check compares a pre-compact transcript snapshot against the
post-compact summary and flags identifiers (file paths, dotted/snake_case
names, etc.) that were mentioned repeatedly before and vanished entirely
after — a cheap, no-LLM-call signal that something load-bearing may have
been lost.

This is a manual command, not automatic watching: Claude Code has a
PreCompact hook but no PostCompact hook, so harnez has no host-native
signal to observe the post-compact result on its own. Save the transcript
before compacting (or point --before at a saved copy) and the summary
after, then run:

  harnez compact-check before.txt after.txt`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			before, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read before file: %w", err)
			}
			after, err := os.ReadFile(args[1])
			if err != nil {
				return fmt.Errorf("read after file: %w", err)
			}
			report := compactcheck.Compare(string(before), string(after))
			fmt.Print(compactcheck.FormatReport(report))
			return nil
		},
	}
	return cmd
}
