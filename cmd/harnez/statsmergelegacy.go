package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/telemetry"
)

// newStatsMergeLegacyCmd recovers tool calls that the XDG migration left only
// in ~/.harnez/tool_catalog.sqlite.bak-<time> backups (issue 735).
func newStatsMergeLegacyCmd() *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "merge-legacy [legacy.sqlite...]",
		Short: "Merge tool calls from legacy telemetry databases into the live database",
		Long: `merge-legacy copies tool_calls rows that the live database does not have yet
from legacy telemetry files, all in one transaction. The default files are the
backups the XDG move left (~/.harnez/tool_catalog.sqlite.bak-<time>); named
backups such as bak-issue331-* hold rows deleted on purpose and are only
merged when passed explicitly.
A row is already present when its created_at, session_id, agent_id, tool_name
and call_type match. Before writing, it saves a consistent copy of the live
database next to it (telemetry.sqlite.before-merge-<time>). Running it again
adds nothing.

  harnez stats merge-legacy
  harnez stats merge-legacy ~/.harnez/tool_catalog.sqlite.bak-20260927T095508.028672123Z`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				p, err := telemetry.DefaultDBPath()
				if err != nil {
					return err
				}
				dbPath = p
			}
			if len(args) == 0 {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				matches, err := filepath.Glob(filepath.Join(home, ".harnez", "tool_catalog.sqlite.bak-[0-9]*"))
				if err != nil {
					return err
				}
				args = legacyDatabaseFiles(matches)
			}
			return runStatsMergeLegacy(cmd.Context(), cmd.OutOrStdout(), dbPath, args, time.Now())
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "override the telemetry database path (default: $XDG_DATA_HOME/harnez/telemetry.sqlite)")
	return cmd
}

// legacyDatabaseFiles drops SQLite side files (-wal, -shm, -journal) from a
// backup glob; they are not databases and come and go with each open.
func legacyDatabaseFiles(matches []string) []string {
	var out []string
	for _, path := range matches {
		if strings.HasSuffix(path, "-wal") || strings.HasSuffix(path, "-shm") || strings.HasSuffix(path, "-journal") {
			continue
		}
		out = append(out, path)
	}
	return out
}

func runStatsMergeLegacy(ctx context.Context, out io.Writer, dbPath string, sources []string, now time.Time) error {
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	before, err := db.ToolCallCount(ctx)
	if err != nil {
		return err
	}
	var missing int64
	for _, source := range sources {
		n, err := db.LegacyToolCallsMissing(ctx, source)
		if err != nil {
			return err
		}
		missing += n
	}
	fmt.Fprintf(out, "tool calls before: %d\n", before)
	if missing == 0 {
		fmt.Fprintf(out, "nothing to merge from %d file(s)\n", len(sources))
		return nil
	}
	backup := dbPath + ".before-merge-" + now.UTC().Format("20060102T150405Z")
	if err := db.BackupInto(ctx, backup); err != nil {
		return fmt.Errorf("back up %s: %w", dbPath, err)
	}
	fmt.Fprintf(out, "backup: %s\n", backup)
	results, err := db.MergeLegacyToolCalls(ctx, sources)
	if err != nil {
		return err
	}
	for _, result := range results {
		fmt.Fprintf(out, "added %d from %s\n", result.Added, result.Source)
	}
	after, err := db.ToolCallCount(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "tool calls after: %d\n", after)
	return nil
}
