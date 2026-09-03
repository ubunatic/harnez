// usageexport implements `harnez usage export`, issue 204's sanitized JSON
// export of tool_calls telemetry (internal/telemetry) and recorded
// usage-history token snapshots (internal/usage) for external
// visualization tools (e.g. ubunatic.com dashboards).
//
// Scope note (issue 204, first pass): JSON only. A SQLite export format is
// deliberately deferred to a follow-up ticket — see
// issues/204-sanitized-telemetry-and-token-export-for-datavis.md's
// "Progress / Scope Note" section.
//
// All scrubbing (path normalization, account masking, hostname
// sanitization) lives in internal/telemetry/export.go and
// internal/usage/export.go, not here — this file only resolves flags,
// calls those exporters, and writes the combined envelope to --out.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usage"
)

// usageExportEnvelope is the top-level JSON shape written to --out: both
// exporters stay independently testable (see their own export_test.go
// files); this envelope is only assembled here at the CLI layer.
type usageExportEnvelope struct {
	GeneratedAt time.Time         `json:"generated_at"`
	Telemetry   telemetry.Export  `json:"telemetry"`
	Usage       usage.UsageExport `json:"usage"`
}

func newUsageExportCmd() *cobra.Command {
	var out string
	var dbPath string
	var historyDir string

	cmd := &cobra.Command{
		Use:   "export --out=<file>",
		Short: "Export sanitized telemetry and token-usage history as JSON for external visualization",
		Long: `export writes a single scrubbed JSON file combining:

  - Tool-call telemetry from internal/telemetry (~/.harnez/tool_catalog.sqlite):
    call frequency, scores, exit codes, durations, byte savings.
  - Token/session usage history from internal/usage
    (~/.claude/harnez/usage-history/*.jsonl): per-agent token totals and
    quota-window percentages over time.

Privacy: absolute filesystem paths (working directories, ticket IDs,
project names) are reduced to their final path component only; account/
email fields are masked via the same MaskAccount helper used elsewhere in
harnez; hostnames are sanitized the same way the on-disk history filenames
already are. Free-form fields with no safe automatic scrubbing (call
notes, Details, Sources) are dropped rather than risk a leak.

This is a JSON-only export (issue 204's first pass) — SQLite export is not
yet implemented.

  harnez usage export --out=telemetry.json`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if out == "" {
				return fmt.Errorf("--out is required")
			}
			return runUsageExport(out, dbPath, historyDir)
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "output file path for the sanitized JSON export (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "override the telemetry database path (default: ~/.harnez/tool_catalog.sqlite)")
	cmd.Flags().StringVar(&historyDir, "history-dir", "", "override the usage-history directory (default: ~/.claude/harnez/usage-history)")
	return cmd
}

func runUsageExport(out, dbPath, historyDir string) error {
	now := time.Now()

	if dbPath == "" {
		p, err := telemetry.DefaultDBPath()
		if err != nil {
			return fmt.Errorf("resolve telemetry db path: %w", err)
		}
		dbPath = p
	}
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open telemetry db: %w", err)
	}
	defer db.Close()

	telExport, err := telemetry.ExportAll(db, now)
	if err != nil {
		return fmt.Errorf("export telemetry: %w", err)
	}

	if historyDir == "" {
		historyDir = usage.HistoryDir("")
	}
	usageExport, err := usage.ExportHistory(historyDir, now)
	if err != nil {
		return fmt.Errorf("export usage history: %w", err)
	}

	envelope := usageExportEnvelope{
		GeneratedAt: now,
		Telemetry:   telExport,
		Usage:       usageExport,
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal export envelope: %w", err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return fmt.Errorf("write export file %s: %w", out, err)
	}
	return nil
}
