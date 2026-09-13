// log implements `harnez log` (issue 327): a git-log-shaped chronological
// view of harnez's own CLI invocation history, read from the
// cli_invocations table issue 326 writes.
//
// Two structural constraints from the ticket are load-bearing here:
//
//  1. This command takes NO subcommands, ever. "History of everything
//     harnez did" invites `harnez log docs` / `log issues` / `log stats`
//     wrappers around dochistory, `find issues history`, and stats — which
//     would give this project two names for each of three features and
//     guarantee they drift. Discoverability is preserved through the Long
//     help's cross-references instead. See TestLogCmd_HasNoSubcommands.
//  2. cli_invocations is the only data source. No git-log parsing, no
//     reading ~/.harnez/sessions/*.usage.json. `harnez log` answers exactly
//     one question none of the neighbouring commands can — what did harnez
//     itself run, and when.
//
// All SQL lives in internal/telemetry (QueryCLIInvocations), per issue
// 120's "keep SQL out of the CLI command file" convention; this file only
// resolves flags into a telemetry.Filter and renders the result.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/telemetry"
)

// defaultLogLimit is the bounded zero-arg default required by
// docs/CLIDesign.md's "Interactive discovery & bounded ingestion" rules: a
// bare `harnez log` in an agent loop must not dump the whole table into the
// context window. --all is the escape hatch.
const defaultLogLimit = 20

func newLogCmd() *cobra.Command {
	var opts logOptions

	cmd := &cobra.Command{
		Use:   "log [-N | -n <limit>] [--project <name>] [--command <name>] [--failed] [--json]",
		Short: "Show harnez's own CLI invocation history, newest first (git log for harnez)",
		Long: `log lists what harnez itself has run — which subcommand, when, in which
project, by whom, and whether it succeeded — newest first, in the shape of
'git log':

  harnez log            # last 20 invocations in the current project
  harnez log -5         # last 5, exactly like 'git log -5'
  harnez log --failed   # only invocations that exited non-zero
  harnez log --human    # only invocations a person typed at a terminal

Rows come from the cli_invocations telemetry table, written automatically
around every harnez run (issue 326). With no arguments, output is scoped to
the project inferred from the current directory and falls back to all
projects when the current directory is not one harnez has recorded.

This command answers exactly one question: what did harnez run, and when.
It deliberately takes no subcommands, because the neighbouring questions
already have their own commands and must not be duplicated here:

  harnez stats                # aggregates over tool_calls: failure rates,
                              # scores, distillation byte savings
  harnez find issues history  # how open/closed ticket counts have moved
  harnez dochistory           # how managed docs have evolved
  git log -- docs/ issues/    # what actually changed in the repo's docs and
                              # tickets (they live in the git tree by design)

--agent accepts either a bare id ('claude') or the stored form
('agent:claude'). --human and --agent are mutually exclusive. --all uncaps
the result and drops the current-directory project scoping; an explicit
--project still applies.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLog(cmd.OutOrStdout(), opts)
		},
	}
	cmd.Flags().IntVarP(&opts.Limit, "limit", "n", defaultLogLimit, "maximum number of invocations to show (also: bare -N, like git log -5)")
	cmd.Flags().BoolVar(&opts.All, "all", false, "show every recorded invocation across all projects (uncapped)")
	cmd.Flags().StringVar(&opts.Project, "project", "", "filter to one project_name")
	cmd.Flags().StringVar(&opts.Session, "session", "", "filter to one session_id")
	cmd.Flags().BoolVar(&opts.Auto, "auto", false, "filter to the current session, resolved from the environment (like harnez stats --auto)")
	cmd.Flags().StringVar(&opts.Agent, "agent", "", "filter to one agent (e.g. claude, or the stored agent:claude)")
	cmd.Flags().BoolVar(&opts.Human, "human", false, "filter to invocations attributed to a human at a terminal")
	cmd.Flags().StringVar(&opts.Command, "command", "", "filter to one subcommand (the full path, e.g. 'index' or 'issues new')")
	cmd.Flags().DurationVar(&opts.Since, "since", 0, "only invocations newer than this duration ago (e.g. 2h, 48h)")
	cmd.Flags().BoolVar(&opts.Failed, "failed", false, "only invocations that exited non-zero")
	cmd.Flags().StringVar(&opts.Dir, "dir", "", "directory whose project name scopes the default view (default: cwd)")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output the rows as JSON instead of a formatted table")
	return cmd
}

// logOptions bundles runLog's inputs. DBPath/Getenv/StateDir are test-only
// overrides, the same pattern statsOptions and findHistoryOptions use;
// production callers leave them empty.
type logOptions struct {
	Limit   int
	All     bool
	Project string
	Session string
	Auto    bool
	Agent   string
	Human   bool
	Command string
	Since   time.Duration
	Failed  bool
	Dir     string
	JSON    bool

	DBPath   string
	Getenv   func(string) string
	StateDir string
	Now      func() time.Time
}

// runLog resolves opts into a telemetry.Filter, queries cli_invocations,
// and renders the result to w.
func runLog(w io.Writer, opts logOptions) error {
	if opts.Human && opts.Agent != "" {
		return fmt.Errorf("log: --human and --agent cannot be combined")
	}
	if opts.Limit <= 0 && !opts.All {
		return fmt.Errorf("log: --limit must be greater than zero (use --all to uncap)")
	}

	dbPath := opts.DBPath
	if dbPath == "" {
		p, err := telemetry.DefaultDBPath()
		if err != nil {
			return fmt.Errorf("log: %w", err)
		}
		dbPath = p
	}
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return fmt.Errorf("log: open telemetry db: %w", err)
	}
	defer db.Close()

	f := telemetry.Filter{
		Project:    opts.Project,
		SessionID:  opts.Session,
		Command:    opts.Command,
		FailedOnly: opts.Failed,
		AgentID:    normalizeAgentFilter(opts.Agent, opts.Human),
	}
	if opts.Since > 0 {
		f.Since = logNow(opts).Add(-opts.Since)
	}
	if opts.Auto {
		sessionID, err := resolve.Session(resolve.SessionOptions{Getenv: opts.Getenv, LockDir: opts.StateDir})
		if err != nil {
			return fmt.Errorf("log: resolve current session: %w", err)
		}
		f.SessionID = sessionID
	}

	// Zero-arg scoping: prefer the project the current directory belongs to,
	// but only when harnez has actually recorded invocations for it —
	// otherwise a bare `harnez log` run from an unrelated directory would
	// print "no data" instead of the useful cross-project view.
	scoped := false
	if f.Project == "" && !opts.All {
		if name, ok := inferredProject(db, opts); ok {
			f.Project = name
			scoped = true
		}
	}

	limit := opts.Limit
	if opts.All {
		limit = 0
	}
	rows, err := db.QueryCLIInvocations(f, limit)
	if err != nil {
		return fmt.Errorf("log: %w", err)
	}

	if opts.JSON {
		if rows == nil {
			rows = []telemetry.CLIInvocation{}
		}
		data, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			return fmt.Errorf("log: %w", err)
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	if len(rows) == 0 {
		fmt.Fprintln(w, noLogDataMessage(f, scoped))
		return nil
	}
	return renderLogTable(w, rows)
}

// logNow is the clock, injectable so --since is testable.
func logNow(opts logOptions) time.Time {
	if opts.Now == nil {
		return time.Now()
	}
	return opts.Now()
}

// normalizeAgentFilter maps the user-facing --agent/--human flags onto the
// stored agent_id vocabulary (issue 328). --agent accepts both the bare id
// people actually type ("claude") and the stored form ("agent:claude"), so
// `harnez log --agent claude` works without the user having to know the
// column's internal shape.
func normalizeAgentFilter(agent string, human bool) string {
	if human {
		return InvokerHuman
	}
	agent = strings.TrimSpace(agent)
	switch {
	case agent == "":
		return ""
	case agent == InvokerHuman || agent == InvokerUnknown:
		return agent
	case strings.HasPrefix(agent, InvokerAgentPrefix):
		return agent
	default:
		return InvokerAgentPrefix + agent
	}
}

// inferredProject reports the project_name the current directory belongs
// to, and whether cli_invocations has any row for it. The "is this a known
// project" test is the table itself rather than a filesystem heuristic:
// this command's whole world is what harnez has recorded, so a directory
// harnez has never run in is, for these purposes, not a project.
func inferredProject(db *telemetry.DB, opts logOptions) (string, bool) {
	dir := opts.Dir
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", false
		}
		dir = wd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	name := filepath.Base(abs)

	probe, err := db.QueryCLIInvocations(telemetry.Filter{Project: name}, 1)
	if err != nil || len(probe) == 0 {
		return "", false
	}
	return name, true
}

// noLogDataMessage names the likely cause of an empty result, matching
// runFindHistory's "no data: ..." convention for an unpopulated table.
func noLogDataMessage(f telemetry.Filter, scoped bool) string {
	var narrowed []string
	if f.Project != "" && !scoped {
		narrowed = append(narrowed, "project "+f.Project)
	}
	if f.Command != "" {
		narrowed = append(narrowed, "command "+f.Command)
	}
	if f.AgentID != "" {
		narrowed = append(narrowed, "agent "+f.AgentID)
	}
	if f.SessionID != "" {
		narrowed = append(narrowed, "session "+f.SessionID)
	}
	if f.FailedOnly {
		narrowed = append(narrowed, "failed runs only")
	}
	if !f.Since.IsZero() {
		narrowed = append(narrowed, "since "+f.Since.Format(time.RFC3339))
	}
	if len(narrowed) > 0 {
		return "no data: no harnez invocations recorded matching " + strings.Join(narrowed, ", ")
	}
	if scoped {
		return fmt.Sprintf("no data: no harnez invocations recorded for project %s (try --all)", f.Project)
	}
	return "no data: no harnez invocations recorded yet (recording is automatic unless $" +
		disableCLILogEnv + " is set)"
}

// renderLogTable writes the house-style tabwriter table, matching
// runFindHistory's rendering in cmd/harnez/find.go.
func renderLogTable(w io.Writer, rows []telemetry.CLIInvocation) error {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "TIME\tPROJECT\tWHO\tCOMMAND\tEXIT\tDUR")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			r.ProjectName,
			displayWho(r.AgentID),
			r.Command,
			displayExit(r.ExitCode),
			(time.Duration(r.DurationMs) * time.Millisecond).String(),
		)
	}
	return tw.Flush()
}

// displayWho renders the stored agent_id for the WHO column: the
// "agent:" namespace prefix is an on-disk disambiguator (issue 328), not
// something worth spending table width on.
func displayWho(agentID string) string {
	return strings.TrimPrefix(agentID, InvokerAgentPrefix)
}

// displayExit renders exit_code, which is nullable in the schema even
// though today's writer always sets it.
func displayExit(code *int) string {
	if code == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *code)
}
