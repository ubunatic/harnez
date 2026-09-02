// stats implements `harnez stats`, an analytical report over the
// tool_calls telemetry table (internal/telemetry, issue 116): call
// frequency, average score, and failure rate broken down per tool and
// per agent, plus a global distillation byte-savings ratio. See
// issues/120-harnez-stats-analytical-reporting.md.
//
// All SQL lives in internal/telemetry (AggregateByTool, AggregateByAgent,
// DistillationSavings) per this ticket's explicit "keep SQL out of the
// CLI command file" note — this file only resolves flags into a
// telemetry.Filter, calls those methods, and renders the result.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/telemetry"
)

func newStatsCmd() *cobra.Command {
	var toolFlag string
	var agentFlag string
	var ticketFlag string
	var autoFlag bool
	var jsonOut bool
	var overheadFlag bool

	cmd := &cobra.Command{
		Use:   "stats [--tool <name>] [--agent <name>] [--ticket <ticket_id>] [--auto]",
		Short: "Report call frequency, average score, failure rate, and byte savings from tool_calls telemetry",
		Long: `stats renders an analytical report over the tool_calls telemetry table
(internal/telemetry, issue 116, populated by 'harnez rate' and 'harnez exec'):

  - Call frequency, average score, and failure rate broken down per tool
    and per agent.
  - A global distillation byte-savings ratio (1 - distilled/raw bytes),
    computed only over rows where distillation actually ran
    (distilled_bytes IS NOT NULL).

  harnez stats
  harnez stats --tool Read --agent claude
  harnez stats --ticket harnez/120-harnez-stats-analytical-reporting --json
  harnez stats --auto

--auto resolves session_id from the current environment (internal/resolve,
same resolution harnez rate/harnez exec use) and filters to just this
session's calls, instead of the all-time/all-session default.

Filters combine with AND when more than one is given. Default output is a
formatted terminal table; --json emits the same numbers unformatted for
scripting (e.g. average score as a float, not a "2 decimal places" string).`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStats(cmd.OutOrStdout(), statsOptions{
				Tool:     toolFlag,
				Agent:    agentFlag,
				Ticket:   ticketFlag,
				Auto:     autoFlag,
				JSON:     jsonOut,
				Overhead: overheadFlag,
			})
		},
	}
	cmd.Flags().StringVar(&toolFlag, "tool", "", "filter to one tool_name")
	cmd.Flags().StringVar(&agentFlag, "agent", "", "filter to one agent_id")
	cmd.Flags().StringVar(&ticketFlag, "ticket", "", "filter to one ticket_id")
	cmd.Flags().BoolVar(&autoFlag, "auto", false, "filter to the current session, resolved from the environment (like harnez rate/exec)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output the report as JSON instead of a formatted table")
	cmd.Flags().BoolVar(&overheadFlag, "overhead", false,
		"also report the harnez rate feedback overhead (issue 142): call count/bytes from telemetry plus an ESTIMATED token cost, clearly labeled as an estimate, not provider-reported")
	return cmd
}

// statsOptions bundles runStats's inputs. The zero value plus explicit
// filter fields matches production behavior (default DB path); tests
// override DBPath/Getenv/StateDir to isolate from the user's real
// telemetry DB and environment.
type statsOptions struct {
	Tool     string
	Agent    string
	Ticket   string
	Auto     bool
	JSON     bool
	Overhead bool

	DBPath   string              // telemetry DB path override; empty means telemetry.DefaultDBPath()
	Getenv   func(string) string // nil means os.Getenv; only consulted when Auto is set
	StateDir string              // resolve.Session state/lock dir override; only consulted when Auto is set
	Config   *claude.Config      // config override for the --overhead instruction-text size; nil means claude.LoadConfigEmbedded()
}

// statsReport is the full shape rendered by both the table and JSON
// renderers — kept as one Go value so --json is guaranteed to report the
// same numbers the table does (they're built from the same struct).
type statsReport struct {
	Filter    telemetry.Filter              `json:"filter"`
	Empty     bool                          `json:"empty"`
	ByTool    []telemetry.GroupStats        `json:"by_tool,omitempty"`
	ByAgent   []telemetry.GroupStats        `json:"by_agent,omitempty"`
	Savings   telemetry.DistillationSavings `json:"distillation_savings"`
	Heartbeat telemetry.HeartbeatInfo       `json:"heartbeat"`
	Overhead  *rateOverheadReport           `json:"rate_feedback_overhead,omitempty"`
}

// rateOverheadReport is the --overhead addendum (issue 142): real measured
// call-count/byte data from telemetry (Calls, TotalCallBytes, AvgCallBytes,
// InstructionBytes) alongside token figures explicitly derived with a
// ~4-bytes-per-token heuristic and labeled as such — never presented as
// provider-reported.
type rateOverheadReport struct {
	Calls                      int64   `json:"calls"`
	TotalCallBytes             int64   `json:"total_call_bytes"`
	AvgCallBytes               float64 `json:"avg_call_bytes"`
	InstructionBytes           int     `json:"instruction_bytes"`
	EstimatedCallTokens        int64   `json:"estimated_call_tokens"`
	EstimatedInstructionTokens int64   `json:"estimated_instruction_tokens"`
	EstimateMethod             string  `json:"estimate_method"`
}

// runStats resolves opts into a telemetry.Filter, queries the DB, and
// writes the rendered report to w.
func runStats(w io.Writer, opts statsOptions) error {
	dbPath := opts.DBPath
	if dbPath == "" {
		p, err := telemetry.DefaultDBPath()
		if err != nil {
			return fmt.Errorf("stats: %w", err)
		}
		dbPath = p
	}

	db, err := telemetry.Open(dbPath)
	if err != nil {
		return fmt.Errorf("stats: open telemetry db: %w", err)
	}
	defer db.Close()

	f := telemetry.Filter{
		ToolName: opts.Tool,
		AgentID:  opts.Agent,
		TicketID: opts.Ticket,
	}

	if opts.Auto {
		sessionID, err := resolve.Session(resolve.SessionOptions{
			Getenv:  opts.Getenv,
			LockDir: opts.StateDir,
		})
		if err != nil {
			return fmt.Errorf("stats: resolve current session: %w", err)
		}
		f.SessionID = sessionID
	}

	report, err := buildStatsReport(db, f)
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}

	if opts.Overhead {
		overhead, err := buildRateOverheadReport(db, f, opts.Config)
		if err != nil {
			return fmt.Errorf("stats: %w", err)
		}
		report.Overhead = &overhead
	}

	if opts.JSON {
		return renderStatsJSON(w, report)
	}
	return renderStatsTable(w, report)
}

// buildStatsReport runs the three telemetry queries stats needs and
// assembles them into one statsReport. Split out from runStats so tests
// can build a report directly off an already-open *telemetry.DB.
func buildStatsReport(db *telemetry.DB, f telemetry.Filter) (statsReport, error) {
	byTool, err := db.AggregateByTool(f)
	if err != nil {
		return statsReport{}, fmt.Errorf("aggregate by tool: %w", err)
	}
	byAgent, err := db.AggregateByAgent(f)
	if err != nil {
		return statsReport{}, fmt.Errorf("aggregate by agent: %w", err)
	}
	savings, err := db.DistillationSavings(f)
	if err != nil {
		return statsReport{}, fmt.Errorf("distillation savings: %w", err)
	}
	heartbeat, err := db.HeartbeatStats(f)
	if err != nil {
		return statsReport{}, fmt.Errorf("heartbeat stats: %w", err)
	}

	return statsReport{
		Filter:    f,
		Empty:     len(byTool) == 0 && len(byAgent) == 0,
		ByTool:    byTool,
		ByAgent:   byAgent,
		Savings:   savings,
		Heartbeat: heartbeat,
	}, nil
}

// buildRateOverheadReport pairs telemetry.RateCallOverhead (real, measured
// per-call data) with claude.ToolFeedbackProtocolBytes (the one-time
// instruction-text size) and converts both to ESTIMATED token counts via
// telemetry.EstimateTokens — see issue 142. cfg defaults to
// claude.LoadConfigEmbedded() when nil (production use); tests inject a
// fixture config instead.
func buildRateOverheadReport(db *telemetry.DB, f telemetry.Filter, cfg *claude.Config) (rateOverheadReport, error) {
	overhead, err := db.RateCallOverhead(f)
	if err != nil {
		return rateOverheadReport{}, fmt.Errorf("rate overhead: %w", err)
	}

	if cfg == nil {
		loaded, err := claude.LoadConfigEmbedded()
		if err != nil {
			return rateOverheadReport{}, fmt.Errorf("load config: %w", err)
		}
		cfg = loaded
	}
	instructionBytes := claude.ToolFeedbackProtocolBytes(cfg)

	return rateOverheadReport{
		Calls:                      overhead.Count,
		TotalCallBytes:             overhead.TotalCallBytes,
		AvgCallBytes:               overhead.AvgCallBytes,
		InstructionBytes:           instructionBytes,
		EstimatedCallTokens:        telemetry.EstimateTokens(overhead.TotalCallBytes),
		EstimatedInstructionTokens: telemetry.EstimateTokens(int64(instructionBytes)),
		EstimateMethod:             "ESTIMATE: ~4 bytes/token heuristic, not provider-reported",
	}, nil
}

// renderStatsJSON writes report as indented JSON. An empty report is
// still valid, non-error JSON (report.Empty is true and the by-tool/
// by-agent slices are simply absent via omitempty) rather than an error
// or an empty-but-ambiguous "{}" — the "empty":true field is what a
// script should check, per issue 120's empty-result acceptance criterion.
func renderStatsJSON(w io.Writer, report statsReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return fmt.Errorf("render json: %w", err)
	}
	return nil
}

// renderStatsTable writes report as a formatted terminal table.
func renderStatsTable(w io.Writer, report statsReport) error {
	if report.Empty {
		fmt.Fprintln(w, "no data: no tool_calls rows match the given filters")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)

	fmt.Fprintln(tw, "TOOL\tCALLS\tAVG SCORE\tFAILURE RATE")
	for _, g := range report.ByTool {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			g.Key, g.Count, formatAvgScore(g), formatFailureRate(g))
	}
	fmt.Fprintln(tw)
	fmt.Fprintln(tw, "AGENT\tCALLS\tAVG SCORE\tFAILURE RATE")
	for _, g := range report.ByAgent {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			g.Key, g.Count, formatAvgScore(g), formatFailureRate(g))
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("render table: %w", err)
	}

	fmt.Fprintln(w)
	if report.Savings.Count == 0 {
		fmt.Fprintln(w, "distillation byte savings: no rows with distillation data")
	} else {
		fmt.Fprintf(w, "distillation byte savings: %.2f%% (%d rows, %d -> %d bytes)\n",
			report.Savings.Ratio*100, report.Savings.Count,
			report.Savings.RawBytes, report.Savings.DistilledBytes)
	}

	fmt.Fprintln(w)
	if hb := report.Heartbeat; hb.Count == 0 {
		fmt.Fprintln(w, "heartbeat (harnez rate --ok, issue 179): none recorded")
	} else {
		fmt.Fprintf(w, "heartbeat (harnez rate --ok, issue 179): %d recorded, last at %s, %d call(s) since\n",
			hb.Count, hb.LastAt.Format("2006-01-02T15:04:05Z07:00"), hb.CallsSince)
	}

	if o := report.Overhead; o != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "harnez rate feedback overhead (issue 142):")
		fmt.Fprintf(w, "  calls: %d, total call bytes: %d, avg call bytes: %.1f (measured)\n",
			o.Calls, o.TotalCallBytes, o.AvgCallBytes)
		fmt.Fprintf(w, "  instruction text: %d bytes, injected once per session (not per call)\n", o.InstructionBytes)
		fmt.Fprintf(w, "  %s: ~%d call tokens + ~%d one-time instruction tokens\n",
			o.EstimateMethod, o.EstimatedCallTokens, o.EstimatedInstructionTokens)
	}
	return nil
}

// formatAvgScore renders a group's average score to 2 decimal places, or
// "n/a" when no row in the group has a score to average (ScoredCount==0
// leaves AvgScore at its zero value, which would otherwise misleadingly
// print as "0.00").
func formatAvgScore(g telemetry.GroupStats) string {
	if g.ScoredCount == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.2f", g.AvgScore)
}

// formatFailureRate renders a group's failure rate — exit_code != 0 OR
// score <= 2 (issue 120's definition; telemetry.GroupStats.FailureCount
// already applies it) — as a percentage of all calls in the group.
func formatFailureRate(g telemetry.GroupStats) string {
	if g.Count == 0 {
		return "n/a"
	}
	rate := float64(g.FailureCount) / float64(g.Count) * 100
	return fmt.Sprintf("%.1f%%", rate)
}
