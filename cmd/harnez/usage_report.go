package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagereport"
)

type usageReportFlags struct {
	Since   string
	Window  string
	Session string
	JSON    bool
}

// newUsageHistoryReportCmd is `harnez usage history report` (ticket 1302 M1):
// read-only, it reads Claude transcripts and quota history and writes nothing.
func newUsageHistoryReportCmd() *cobra.Command {
	var f usageReportFlags
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Report Claude API cost, weekly quota share and subsidy factor for a range or the weekly window",
		Long: "Report Claude API cost, weekly quota share, plan cost (net and gross) and the subsidy factor\n" +
			"(API cost / plan cost) per session and in total. Tokens come from the Claude transcripts under\n" +
			"~/.claude/projects, the quota share from quota-history.jsonl. Read-only.",
		Example: "  harnez usage history report --since 7d\n  harnez usage history report --window weekly --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if f.Window != "" && cmd.Flags().Changed("since") {
				return fmt.Errorf("--window and --since are exclusive")
			}
			return runUsageReport(cmd.OutOrStdout(), f)
		},
	}
	cmd.Flags().StringVar(&f.Since, "since", "7d", "range start before now: a Go duration or whole days (7d)")
	cmd.Flags().StringVar(&f.Window, "window", "", `use the newest weekly quota window as the range ("weekly")`)
	cmd.Flags().StringVar(&f.Session, "session", "", "show only this session id (attribution still uses every session)")
	cmd.Flags().BoolVar(&f.JSON, "json", false, "print the report as JSON")
	return cmd
}

// parseReportSince reads --since: a Go duration, or whole days written "7d".
func parseReportSince(s string) (time.Duration, error) {
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("--since %q: days must be a positive whole number", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("--since %q: want a positive duration such as 36h or 7d", s)
	}
	return d, nil
}

func runUsageReport(w io.Writer, f usageReportFlags) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	projectsDir := filepath.Join(home, ".claude", "projects")
	historyDir := usage.HistoryDir("")
	samples, err := usage.ReadQuotaHistory(historyDir)
	if err != nil {
		return fmt.Errorf("read quota history: %w", err)
	}
	resetTol, err := usage.TurnQuotaResetTolerance()
	if err != nil {
		return err
	}
	until := time.Now().UTC()
	var since time.Time
	switch f.Window {
	case "":
		d, err := parseReportSince(f.Since)
		if err != nil {
			return err
		}
		since = until.Add(-d)
	case "weekly":
		start, ok := usagereport.WeeklyWindowStart(samples, resetTol)
		if !ok {
			return fmt.Errorf("--window weekly: no weekly quota history in %s; use --since", historyDir)
		}
		since = start
	default:
		return fmt.Errorf(`--window %q: only "weekly" is supported`, f.Window)
	}
	catalog, err := telemetry.LoadPricingCatalog("")
	if err != nil {
		return err
	}
	monthly, err := subagent.PlanPriceFor("claude")
	if err != nil {
		return err
	}
	var weekly *int64
	if monthly != nil {
		v := usage.WeeklyPlanMicros(*monthly)
		weekly = &v
	}
	vat, err := subagent.PlanVATRate()
	if err != nil {
		return err
	}
	unexplainedTol, err := usage.ReportUnexplainedTolerancePct()
	if err != nil {
		return err
	}
	calls, err := usagereport.ScanCalls(projectsDir)
	if err != nil {
		return fmt.Errorf("read claude transcripts: %w", err)
	}
	rep := usagereport.Build(calls, samples, usagereport.Options{
		Since: since, Until: until, Session: f.Session, Catalog: catalog,
		ResetTolerance: resetTol, WeeklyPlanMicros: weekly, VATRate: vat,
		UnexplainedTolerancePct: unexplainedTol,
	})
	if f.JSON {
		return usagereport.RenderJSON(w, rep)
	}
	return usagereport.RenderText(w, rep, unexplainedTol)
}
