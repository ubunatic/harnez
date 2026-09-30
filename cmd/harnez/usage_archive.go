package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usage"
)

func newUsageArchiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Copy legacy usage files into a checksummed archive",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "create",
		Short: "Copy known legacy usage files and write their manifest",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := usage.ArchiveLegacyUsageData(usage.DefaultLegacyUsageArchivePaths())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "archived %d files to %s\n", result.Files, result.Path)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "import <archive-directory>",
		Short: "Validate and import a legacy usage archive into the telemetry store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath, err := telemetry.DefaultDBPath()
			if err != nil {
				return err
			}
			result, err := usage.ImportLegacyUsageArchive(cmd.Context(), dbPath, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "validated and imported archive %s (%d files)\n", result.Path, result.Files)
			return nil
		},
	})
	return cmd
}
